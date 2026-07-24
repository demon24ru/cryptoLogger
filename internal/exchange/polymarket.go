package exchange

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/milkywaybrain/cryptogalaxy/internal/config"
	"github.com/milkywaybrain/cryptogalaxy/internal/connector"
	"github.com/milkywaybrain/cryptogalaxy/internal/storage"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// Polymarket connector.
//
// Unlike the crypto-spot exchanges, Polymarket markets are dynamic and recurring,
// so this connector DISCOVERS the active ABOVE/RANGE markets for the configured
// coins (markets[].id, e.g. "BTC", "ETH") via the Gamma REST API — rather than
// reading a static market list from config. It upserts their metadata + resolution
// into `polymarket_market`, and subscribes to each outcome token's CLOB order book
// on the market websocket channel, persisting the RAW book/price_change stream into
// `polymarket_book`. The spot price (model underlying) is NOT written here — the
// backtest joins it from the existing spot ticker. See REAL_DATA_SCHEMA.md.

// Defaults for Polymarket connector config (used when the config value is zero).
const (
	polyDefaultDiscoveryIntSec  = 300
	polyDefaultResolutionIntSec = 300
	polyDefaultFullBookIntSec   = 300  // force a REST full-book anchor per token this often
	polyDefaultGammaTagID       = 1312 // Crypto Prices (base tag; coins from markets[].id)
	polyGammaPageLimit          = 100  // Gamma caps events per page at 100 -> paginate by offset
	polyGammaMaxPages           = 50   // safety cap on discovery pagination
	polyHTTPTimeoutSec          = 20
	polyPingIntSec              = 10
	polyBookFlushIntSec         = 2  // time-based flush so quiet strikes' tail rows are not held back
	polyFullBookThrottleMs      = 25 // gap between per-token REST /book requests in an anchor sweep
)

// polyDefaultExcludeTagIDs are Gamma sub-tags excluded from discovery so the
// Crypto-Prices base tag yields the full BTC price-market set without off-topic
// / non-price events. Overridable via config (exclude_tag_ids).
var polyDefaultExcludeTagIDs = []int{136, 102368, 102127, 102536, 100198, 101337, 100150, 102537}

// polyBadContext lists non-price contexts to exclude even when the phrasing
// otherwise looks like a level (e.g. "Bitcoin Volatility Index above 40",
// "Bitcoin dominance ...").
var polyBadContext = []string{"volatility", "dominance", "index", "market cap", "outperform", "valuable", "flip"}

// polyTagToTicker maps a Gamma tag label (lower-cased) to the coin ticker. The
// discovery feed (Crypto Prices) carries every coin, so the coin is derived per
// event from its tags (first coin tag wins), with a title fallback below.
var polyTagToTicker = map[string]string{
	"bitcoin":  "BTC",
	"ethereum": "ETH", "ether": "ETH",
	"solana": "SOL",
	"xrp":    "XRP", "ripple": "XRP",
	"dogecoin": "DOGE", "doge": "DOGE",
	"bnb":       "BNB",
	"chainlink": "LINK", "link": "LINK",
	"cardano": "ADA", "ada": "ADA",
	"avalanche": "AVAX", "avax": "AVAX",
	"polkadot": "DOT", "dot": "DOT",
	"litecoin": "LTC", "ltc": "LTC",
	"tron": "TRX", "trx": "TRX",
	"pepe": "PEPE", "shiba": "SHIB", "shib": "SHIB",
	"sui": "SUI", "aptos": "APT",
	"hyperliquid": "HYPE", "hype": "HYPE",
	"ethena": "ENA", "ena": "ENA",
	"bittensor": "TAO", "tao": "TAO",
}

// polyTitleKeywords resolves the coin from the event title when tags are
// ambiguous. Ordered longest/most-specific first.
var polyTitleKeywords = []struct{ kw, ticker string }{
	{"bitcoin", "BTC"}, {"ethereum", "ETH"}, {"solana", "SOL"},
	{"ripple", "XRP"}, {"xrp", "XRP"}, {"dogecoin", "DOGE"}, {"doge", "DOGE"},
	{"chainlink", "LINK"}, {"cardano", "ADA"}, {"avalanche", "AVAX"},
	{"polkadot", "DOT"}, {"litecoin", "LTC"}, {"tron", "TRX"},
}

// eventCoin derives the uppercase coin ticker of a Gamma event: first from its
// tags (label -> ticker), then falling back to keywords in the title. Returns ""
// when no coin can be determined (e.g. stablecoin / RWA / index events).
func eventCoin(ev *gammaEvent) string {
	for _, tag := range ev.Tags {
		if t, ok := polyTagToTicker[strings.ToLower(strings.TrimSpace(tag.Label))]; ok {
			return t
		}
	}
	tl := strings.ToLower(ev.Title)
	for _, kw := range polyTitleKeywords {
		if strings.Contains(tl, kw.kw) {
			return kw.ticker
		}
	}
	return ""
}

// polyNumRe extracts integer/decimal groups (thousands-separated) from a title,
// e.g. "60,000" -> 60000, "↑ 100,000" -> 100000, "60,000-65,000" -> [60000,65000].
var polyNumRe = regexp.MustCompile(`\d[\d,]*(?:\.\d+)?`)

// polyRangeRe matches an "A-B" numeric range in a groupItemTitle (hyphen or
// en-dash), e.g. "56,000-58,000" or "66,000–68,000".
var polyRangeRe = regexp.MustCompile(`\d[\d,]*(?:\.\d+)?\s*[-–]\s*\d[\d,]*(?:\.\d+)?`)

// StartPolymarket is for starting the Polymarket connector functions.
func StartPolymarket(appCtx context.Context, markets []config.Market, retry *config.Retry, connCfg *config.Connection) error {

	// Retry with a time gap on any error / lost connection, same policy as other exchanges.
	var retryCount int
	lastRetryTime := time.Now()

	// Shared per-token sequence counter, seeded from wall-clock so book rows stay
	// ordered across reconnects (this loop) AND process restarts (the seed jumps
	// forward with time). ~1e6 headroom per ms is far beyond real message rates.
	seq := uint64(time.Now().UnixMilli()) * 1_000_000

	for {
		log.Error().Str("exchange", "polymarket").Msg("start polymarket")
		err := newPolymarket(appCtx, markets, connCfg, &seq)
		if err != nil {
			log.Error().Err(err).Str("exchange", "polymarket").Msg("error occurred")
			if retry.Number == 0 {
				return errors.New("not able to connect polymarket. please check the log for details")
			}
			if retry.ResetSec == 0 || time.Since(lastRetryTime).Seconds() < float64(retry.ResetSec) {
				retryCount++
			} else {
				retryCount = 1
			}
			lastRetryTime = time.Now()
			if retryCount > retry.Number {
				err = fmt.Errorf("not able to connect polymarket even after %d retry", retry.Number)
				log.Error().Err(err).Str("exchange", "polymarket").Msg("")
				return err
			}

			log.Error().Str("exchange", "polymarket").Int("retry", retryCount).Msg(fmt.Sprintf("retrying functions in %d seconds", retry.GapSec))
			tick := time.NewTicker(time.Duration(retry.GapSec) * time.Second)
			select {
			case <-tick.C:
				tick.Stop()
			case <-appCtx.Done():
				log.Error().Str("exchange", "polymarket").Msg("ctx canceled, return from StartPolymarket")
				return appCtx.Err()
			}
		}
	}
}

// tokenMeta is the per-token grouping info needed to write a book snapshot row
// and to route it to the coin's configured storages.
type tokenMeta struct {
	eventID     string
	conditionID string
	coin        string
}

// polyCoinCfg is the per-coin (markets[].id) config: which event types to keep
// and which storages to route to — the Polymarket analog of the per-market
// cfgLookup used by the other exchange connectors (kucoin et al.).
type polyCoinCfg struct {
	types     map[string]struct{} // ABOVE / RANGE; empty = all supported types
	terStr    bool
	clickHStr bool
}

// keepType reports whether a market type should be recorded for this coin.
func (c *polyCoinCfg) keepType(mtype string) bool {
	if len(c.types) == 0 {
		return true
	}
	_, ok := c.types[mtype]
	return ok
}

// knownMarket is a tracked outcome-token metadata row plus its coin, so that
// resolution upserts can be routed to the coin's storages.
type knownMarket struct {
	row  storage.PolymarketMarket
	coin string
}

// polyBookItem is a raw book row plus the per-coin storage routing flags, sent
// from the reader/anchor goroutines to the batcher.
type polyBookItem struct {
	row storage.PolymarketBook
	ter bool
	ch  bool
}

type polymarket struct {
	ws      connector.Websocket
	http    *http.Client
	connCfg *config.Connection

	// cfg (resolved with defaults).
	discoveryIntSec  int
	resolutionIntSec int
	fullBookIntSec   int
	coinCfg          map[string]*polyCoinCfg // coin ticker (markets[].id) -> types + storage routing
	gammaTagID       int
	excludeTagIDs    []int
	autoCreateTables bool

	// per-storage commit batch sizes, taken from the terminal/clickhouse connection
	// config (book -> orders_book_commit_buffer, metadata -> market_commit_buffer).
	terBookBuf   int
	chBookBuf    int
	terMarketBuf int
	chMarketBuf  int

	// seq is a per-token monotonic ordering counter shared across reconnects
	// (owned by StartPolymarket, time-seeded so it stays monotonic across process
	// restarts too). Incremented atomically per persisted book row.
	seq *uint64

	// storages (terStr/clickHStr are the union across coins — used to create the
	// commit channels/workers; per-coin routing lives in coinCfg).
	ter        *storage.Terminal
	clickhouse *storage.ClickHouse
	terStr     bool
	clickHStr  bool

	// book commit channels/buffers.
	bookOut          chan polyBookItem // reader/anchor -> batcher (single rows + routing flags)
	wsTerBook        chan []storage.PolymarketBook
	wsClickHouseBook chan []storage.PolymarketBook

	// shared discovery state (guarded by mu).
	mu         sync.RWMutex
	meta       map[string]tokenMeta    // token_id -> grouping ids + coin
	subscribed map[string]struct{}     // token_id currently subscribed on ws
	known      map[string]*knownMarket // token_id -> latest metadata row + coin

	// ws write serialization (discovery sub/unsub + pinger share the socket).
	wsMu sync.Mutex
}

func newPolymarket(appCtx context.Context, markets []config.Market, connCfg *config.Connection, seq *uint64) error {
	// Own a cancelable context so any early return (e.g. initial discovery
	// failure) cancels and unblocks the goroutines already registered on the
	// errgroup — otherwise pingWs / closeWsConnOnError / commit workers would
	// leak, along with the open websocket, on every startup retry.
	groupCtx, cancel := context.WithCancel(appCtx)
	defer cancel()
	polyErrGroup, ctx := errgroup.WithContext(groupCtx)

	p := polymarket{
		connCfg:    connCfg,
		seq:        seq,
		coinCfg:    make(map[string]*polyCoinCfg),
		meta:       make(map[string]tokenMeta),
		subscribed: make(map[string]struct{}),
		known:      make(map[string]*knownMarket),
	}
	p.applyConfig(connCfg)

	// Each configured market is a coin (id, e.g. "BTC", "ETH"); its info entries
	// carry the event-type filter (market_types) and the storages to route to.
	// Aggregated per coin — the Polymarket analog of the other exchanges' per-market
	// cfgLookup. terStr/clickHStr are the union (to create channels/workers once).
	for _, market := range markets {
		coin := strings.ToUpper(strings.TrimSpace(market.ID))
		if coin == "" {
			continue
		}
		cc := p.coinCfg[coin]
		if cc == nil {
			cc = &polyCoinCfg{types: make(map[string]struct{})}
			p.coinCfg[coin] = cc
		}
		for _, info := range market.Info {
			for _, t := range info.MarketTypes {
				t = strings.ToUpper(strings.TrimSpace(t))
				if t == "" {
					continue
				}
				if t != "ABOVE" && t != "RANGE" {
					log.Error().Str("exchange", "polymarket").Str("coin", coin).Str("market_type", t).Msg("unknown market_type in config (supported: ABOVE, RANGE) — nothing will match it")
				}
				cc.types[t] = struct{}{}
			}
			for _, str := range info.Storages {
				switch str {
				case "terminal":
					cc.terStr = true
					if p.ter == nil {
						p.ter = storage.GetTerminal()
					}
					if !p.terStr {
						p.terStr = true
						p.wsTerBook = make(chan []storage.PolymarketBook, 1)
					}
				case "clickhouse":
					cc.clickHStr = true
					if p.clickhouse == nil {
						p.clickhouse = storage.GetClickHouse()
					}
					if !p.clickHStr {
						p.clickHStr = true
						p.wsClickHouseBook = make(chan []storage.PolymarketBook, 1)
					}
				}
			}
		}
	}
	if len(p.coinCfg) == 0 {
		return errors.New("polymarket: no coins configured (set markets[].id, e.g. \"BTC\", \"ETH\")")
	}
	if !p.terStr && !p.clickHStr {
		return errors.New("polymarket: no storage configured (need terminal and/or clickhouse)")
	}
	p.bookOut = make(chan polyBookItem, 256)

	timeout := connCfg.REST.ReqTimeoutSec
	if timeout <= 0 {
		timeout = polyHTTPTimeoutSec
	}
	p.http = &http.Client{Timeout: time.Duration(timeout) * time.Second}

	// Best-effort table creation on a fresh database.
	if p.clickHStr && p.autoCreateTables {
		if err := p.clickhouse.CreatePolymarketTables(ctx); err != nil {
			log.Error().Err(err).Str("exchange", "polymarket").Msg("auto-create tables failed (continuing; ensure scripts/clickhouse_schema.sql applied)")
		} else {
			log.Info().Str("exchange", "polymarket").Msg("polymarket tables ensured")
		}
	}

	// Connect websocket.
	if err := p.connectWs(ctx); err != nil {
		return err
	}
	polyErrGroup.Go(func() error {
		return p.closeWsConnOnError(ctx)
	})
	polyErrGroup.Go(func() error {
		return p.pingWs(ctx)
	})

	// Book commit workers + batcher (count- or time-triggered flush).
	polyErrGroup.Go(func() error {
		return p.bookBatcher(ctx)
	})
	if p.terStr {
		polyErrGroup.Go(func() error {
			return WsPolymarketBookToStorage(ctx, p.ter, p.wsTerBook)
		})
	}
	if p.clickHStr {
		polyErrGroup.Go(func() error {
			return WsPolymarketBookToStorage(ctx, p.clickhouse, p.wsClickHouseBook)
		})
	}

	// Initial discovery + subscription before the reader starts, so the first
	// book snapshots the server pushes are for tokens we already know.
	if err := p.discover(ctx, true); err != nil {
		return err
	}

	polyErrGroup.Go(func() error {
		return p.readWs(ctx)
	})
	polyErrGroup.Go(func() error {
		return p.discoveryLoop(ctx)
	})
	polyErrGroup.Go(func() error {
		return p.resolutionLoop(ctx)
	})
	polyErrGroup.Go(func() error {
		return p.fullBookLoop(ctx)
	})

	return polyErrGroup.Wait()
}

func (p *polymarket) applyConfig(connCfg *config.Connection) {
	pc := connCfg.Polymarket
	p.discoveryIntSec = pc.DiscoveryIntSec
	if p.discoveryIntSec <= 0 {
		p.discoveryIntSec = polyDefaultDiscoveryIntSec
	}
	p.resolutionIntSec = pc.ResolutionIntSec
	if p.resolutionIntSec <= 0 {
		p.resolutionIntSec = polyDefaultResolutionIntSec
	}
	p.fullBookIntSec = pc.FullBookIntSec
	if p.fullBookIntSec <= 0 {
		p.fullBookIntSec = polyDefaultFullBookIntSec
	}
	// Batch sizes are taken from the terminal/clickhouse connection config — the
	// book stream reuses orders_book_commit_buffer, metadata uses market_commit_buffer.
	// (No polymarket-specific buffer settings — those would just duplicate these.)
	p.terBookBuf = atLeastOne(connCfg.Terminal.OrdersBookCommitBuf)
	p.chBookBuf = atLeastOne(connCfg.ClickHouse.OrdersBookCommitBuf)
	p.terMarketBuf = atLeastOne(connCfg.Terminal.MarketCommitBuf)
	p.chMarketBuf = atLeastOne(connCfg.ClickHouse.MarketCommitBuf)
	p.gammaTagID = pc.GammaTagID
	if p.gammaTagID <= 0 {
		p.gammaTagID = polyDefaultGammaTagID
	}
	p.excludeTagIDs = pc.ExcludeTagIDs
	if len(p.excludeTagIDs) == 0 {
		p.excludeTagIDs = polyDefaultExcludeTagIDs
	}
	p.autoCreateTables = pc.AutoCreateTables
}

func (p *polymarket) connectWs(ctx context.Context) error {
	ws, err := connector.NewWebsocket(ctx, &p.connCfg.WS, config.PolymarketWebsocketURL)
	if err != nil {
		if !errors.Is(err, ctx.Err()) {
			logErrStack(err)
		}
		return err
	}
	p.ws = ws
	log.Info().Str("exchange", "polymarket").Msg("websocket connected")
	return nil
}

// closeWsConnOnError closes the websocket connection on app-context cancel.
func (p *polymarket) closeWsConnOnError(ctx context.Context) error {
	<-ctx.Done()
	err := p.ws.Conn.Close()
	if err != nil {
		return err
	}
	return ctx.Err()
}

// pingWs sends the literal "PING" text frame required by Polymarket to keep the
// market channel alive.
func (p *polymarket) pingWs(ctx context.Context) error {
	tick := time.NewTicker(polyPingIntSec * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if err := p.wsWrite([]byte("PING")); err != nil {
				if errors.Is(err, net.ErrClosed) {
					err = errors.New("context canceled")
				} else {
					logErrStack(err)
				}
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *polymarket) wsWrite(frame []byte) error {
	p.wsMu.Lock()
	defer p.wsMu.Unlock()
	return p.ws.Write(frame)
}

// --- Discovery (Gamma REST) ------------------------------------------------ //

type gammaEvent struct {
	ID      jsonFlexString `json:"id"`
	Slug    string         `json:"slug"`
	Title   string         `json:"title"`
	Tags    []gammaTag     `json:"tags"`
	Markets []gammaMarket  `json:"markets"`
}

type gammaTag struct {
	Label string `json:"label"`
}

type gammaMarket struct {
	ConditionID           string    `json:"conditionId"`
	Question              string    `json:"question"`
	GroupItemTitle        string    `json:"groupItemTitle"`
	ClobTokenIDs          string    `json:"clobTokenIds"`  // JSON-encoded string array
	Outcomes              string    `json:"outcomes"`      // JSON-encoded string array
	OutcomePrices         string    `json:"outcomePrices"` // JSON-encoded string array
	EndDate               string    `json:"endDate"`
	StartDate             string    `json:"startDate"`
	Active                bool      `json:"active"`
	Closed                bool      `json:"closed"`
	EnableOrderBook       bool      `json:"enableOrderBook"`
	OrderMinSize          flexFloat `json:"orderMinSize"`
	OrderPriceMinTickSize flexFloat `json:"orderPriceMinTickSize"`
}

// discoveryLoop periodically re-discovers active markets for the configured coins.
func (p *polymarket) discoveryLoop(ctx context.Context) error {
	tick := time.NewTicker(time.Duration(p.discoveryIntSec) * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if err := p.discover(ctx, false); err != nil {
				if errors.Is(err, ctx.Err()) {
					return err
				}
				// Discovery failures are transient (network / Gamma hiccup);
				// log and keep the existing subscriptions rather than tearing
				// down the whole connector.
				logErrStack(err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// discover fetches active BTC markets, upserts new metadata, and reconciles the
// websocket subscription set. When initial is true it sends one bulk subscribe
// frame; otherwise it sends incremental subscribe/unsubscribe operations.
func (p *polymarket) discover(ctx context.Context, initial bool) error {
	events, err := p.fetchActiveEvents(ctx)
	if err != nil {
		return err
	}

	desired := make(map[string]tokenMeta)
	newByCoin := make(map[string][]storage.PolymarketMarket) // coin -> new metadata rows
	now := time.Now().UTC()

	for _, ev := range events {
		ev := ev
		// Coin is determined per event from its tags (title fallback); keep only
		// events whose coin is a configured market id (markets[].id).
		coin := eventCoin(&ev)
		cc, ok := p.coinCfg[coin]
		if !ok {
			continue
		}
		for _, m := range ev.Markets {
			if !m.Active || m.Closed || !m.EnableOrderBook {
				continue
			}
			low, high, mtype, ok := parseMarket(m.GroupItemTitle, m.Question)
			if !ok {
				continue // not an in-scope ABOVE/RANGE price market — skip
			}
			if !cc.keepType(mtype) {
				continue // event type not selected for this coin (market_types filter)
			}
			tokenIDs := parseJSONStringArray(m.ClobTokenIDs)
			outcomes := parseJSONStringArray(m.Outcomes)
			if len(tokenIDs) == 0 {
				continue
			}

			// Write BOTH tokens of the strike (index 0 = Yes/Up, 1 = No/Down).
			// Both share the same bounds/market_type (No is the complement of the
			// same strike); the reader merges Yes+No into one effective book.
			for idx, tokenID := range tokenIDs {
				if idx > 1 {
					break // binary markets only ever have 2 tokens
				}
				if tokenID == "" {
					continue // skip a missing id but still consider the sibling token
				}
				outcomeName := defaultOutcomeName(idx)
				if idx < len(outcomes) && outcomes[idx] != "" {
					outcomeName = outcomes[idx]
				}

				desired[tokenID] = tokenMeta{eventID: string(ev.ID), conditionID: m.ConditionID, coin: coin}

				p.mu.RLock()
				_, seen := p.known[tokenID]
				p.mu.RUnlock()
				if seen {
					continue
				}

				row := storage.PolymarketMarket{
					EventID:        string(ev.ID),
					EventSlug:      ev.Slug,
					ConditionID:    m.ConditionID,
					TokenID:        tokenID,
					TokenIndex:     uint8(idx),
					Question:       m.Question,
					OutcomeName:    outcomeName,
					MarketType:     mtype,
					PriceLow:       low,
					PriceHigh:      high,
					TickSize:       float64(m.OrderPriceMinTickSize),
					MinOrderSize:   float64(m.OrderMinSize),
					CreatedTs:      parseISOTime(m.StartDate),
					ExpiryTs:       parseISOTime(m.EndDate),
					Resolved:       0,
					WinningOutcome: nil,
					UpdatedTs:      now,
				}
				newByCoin[coin] = append(newByCoin[coin], row)
			}
		}
	}

	// Persist new market rows (routed to each coin's storages) and register them.
	newCount := 0
	for coin, rows := range newByCoin {
		if err := p.upsertMarkets(ctx, coin, rows); err != nil {
			return err
		}
		p.mu.Lock()
		for i := range rows {
			r := rows[i]
			p.known[r.TokenID] = &knownMarket{row: r, coin: coin}
		}
		p.mu.Unlock()
		newCount += len(rows)
	}
	if newCount > 0 {
		log.Info().Str("exchange", "polymarket").Int("new_tokens", newCount).Msg("discovered new markets")
	}

	// Guard against a transient empty result (Gamma returning 200 with an empty
	// list) mass-unsubscribing every token on a periodic cycle. On the initial
	// pass an empty set is reported by reconcileSubscriptions instead.
	if !initial && len(desired) == 0 {
		log.Error().Str("exchange", "polymarket").Msg("discovery returned no active markets; keeping existing subscriptions")
		return nil
	}

	// Publish the meta map for the reader (grouping ids per token).
	p.mu.Lock()
	for tok, meta := range desired {
		p.meta[tok] = meta
	}
	p.mu.Unlock()

	// Reconcile subscriptions.
	return p.reconcileSubscriptions(desired, initial)
}

func (p *polymarket) reconcileSubscriptions(desired map[string]tokenMeta, initial bool) error {
	var toSub, toUnsub []string

	p.mu.Lock()
	if initial {
		for tok := range desired {
			p.subscribed[tok] = struct{}{}
			toSub = append(toSub, tok)
		}
	} else {
		for tok := range desired {
			if _, ok := p.subscribed[tok]; !ok {
				p.subscribed[tok] = struct{}{}
				toSub = append(toSub, tok)
			}
		}
		for tok := range p.subscribed {
			if _, ok := desired[tok]; !ok {
				delete(p.subscribed, tok)
				delete(p.meta, tok) // reader no longer needs grouping ids for it
				toUnsub = append(toUnsub, tok)
			}
		}
	}
	p.mu.Unlock()

	if initial {
		if len(toSub) == 0 {
			log.Error().Str("exchange", "polymarket").Msg("no active BTC markets found on initial discovery")
			return nil
		}
		return p.sendSubscribe(toSub, "")
	}
	if len(toSub) > 0 {
		if err := p.sendSubscribe(toSub, "subscribe"); err != nil {
			return err
		}
	}
	if len(toUnsub) > 0 {
		if err := p.sendSubscribe(toUnsub, "unsubscribe"); err != nil {
			return err
		}
	}
	return nil
}

// sendSubscribe sends a market-channel (un)subscribe frame. An empty operation
// is the initial bulk subscribe; "subscribe"/"unsubscribe" are live operations.
func (p *polymarket) sendSubscribe(tokenIDs []string, operation string) error {
	msg := map[string]interface{}{
		"assets_ids": tokenIDs,
		"type":       "market",
	}
	if operation != "" {
		msg["operation"] = operation
	}
	frame, err := jsoniter.Marshal(msg)
	if err != nil {
		logErrStack(err)
		return err
	}
	if err := p.wsWrite(frame); err != nil {
		if errors.Is(err, net.ErrClosed) {
			err = errors.New("context canceled")
		} else {
			logErrStack(err)
		}
		return err
	}
	return nil
}

func (p *polymarket) fetchActiveEvents(ctx context.Context) ([]gammaEvent, error) {
	// Gamma caps events at 100 per page, so paginate by offset until a short page.
	// A single tag can't return every BTC event; discovery uses a base tag
	// (Crypto Prices) plus exclude_tag_id filters, then keeps only BTC markets.
	var excl strings.Builder
	for _, ex := range p.excludeTagIDs {
		fmt.Fprintf(&excl, "&exclude_tag_id=%d", ex)
	}

	var all []gammaEvent
	for page := 0; page < polyGammaMaxPages; page++ {
		offset := page * polyGammaPageLimit
		url := fmt.Sprintf("%sevents?tag_id=%d&closed=false&active=true&order=startDate&ascending=false&limit=%d&offset=%d%s",
			config.PolymarketGammaBaseURL, p.gammaTagID, polyGammaPageLimit, offset, excl.String())
		var events []gammaEvent
		if err := p.getJSON(ctx, url, &events); err != nil {
			return nil, err
		}
		all = append(all, events...)
		if len(events) < polyGammaPageLimit {
			break
		}
		if page == polyGammaMaxPages-1 {
			log.Error().Str("exchange", "polymarket").Int("pages", polyGammaMaxPages).Msg("discovery hit the pagination cap; some active markets may be omitted")
		}
	}
	return all, nil
}

// --- Resolution (CLOB REST) ------------------------------------------------ //

type clobMarket struct {
	ConditionID      string    `json:"condition_id"`
	Closed           bool      `json:"closed"`
	MinimumTickSize  flexFloat `json:"minimum_tick_size"`
	MinimumOrderSize flexFloat `json:"minimum_order_size"`
	Tokens           []struct {
		TokenID string    `json:"token_id"`
		Outcome string    `json:"outcome"`
		Price   flexFloat `json:"price"`
		Winner  bool      `json:"winner"`
	} `json:"tokens"`
}

// resolutionLoop periodically checks markets past expiry for a real Polymarket
// resolution and updates winning_outcome / resolved.
func (p *polymarket) resolutionLoop(ctx context.Context) error {
	tick := time.NewTicker(time.Duration(p.resolutionIntSec) * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if err := p.checkResolutions(ctx); err != nil {
				if errors.Is(err, ctx.Err()) {
					return err
				}
				logErrStack(err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *polymarket) checkResolutions(ctx context.Context) error {
	now := time.Now().UTC()

	// Snapshot candidate markets (past expiry and not yet resolved). Group by
	// condition_id so we do one CLOB request per market, not per token.
	conds := make(map[string]struct{})
	p.mu.RLock()
	for _, kn := range p.known {
		if kn.row.Resolved == 1 {
			continue
		}
		if kn.row.ExpiryTs.IsZero() || kn.row.ExpiryTs.After(now) {
			continue
		}
		conds[kn.row.ConditionID] = struct{}{}
	}
	p.mu.RUnlock()

	if len(conds) == 0 {
		return nil
	}

	updatedByCoin := make(map[string][]storage.PolymarketMarket)
	var resolvedTokens []string
	for conditionID := range conds {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		cm, err := p.fetchCLOBMarket(ctx, conditionID)
		if err != nil {
			logErrStack(err)
			continue
		}
		if !cm.Closed {
			continue // not settled yet
		}
		winnerByToken := make(map[string]bool)
		for _, t := range cm.Tokens {
			winnerByToken[t.TokenID] = t.Winner
		}

		p.mu.Lock()
		for tok, kn := range p.known {
			if kn.row.ConditionID != conditionID || kn.row.Resolved == 1 {
				continue
			}
			win, ok := winnerByToken[tok]
			if !ok {
				continue
			}
			var wo uint8
			if win {
				wo = 1
			}
			kn.row.Resolved = 1
			woCopy := wo
			kn.row.WinningOutcome = &woCopy
			kn.row.UpdatedTs = time.Now().UTC()
			updatedByCoin[kn.coin] = append(updatedByCoin[kn.coin], kn.row)
			resolvedTokens = append(resolvedTokens, tok)
		}
		p.mu.Unlock()
	}

	updated := 0
	for coin, rows := range updatedByCoin {
		if err := p.upsertMarkets(ctx, coin, rows); err != nil {
			return err
		}
		updated += len(rows)
	}
	if updated > 0 {
		// Resolution persisted — drop these from the working set so it does not
		// grow unbounded over a long-running recorder.
		p.mu.Lock()
		for _, tok := range resolvedTokens {
			delete(p.known, tok)
			delete(p.meta, tok)
		}
		p.mu.Unlock()
		log.Info().Str("exchange", "polymarket").Int("resolved", updated).Msg("updated market resolutions")
	}
	return nil
}

func (p *polymarket) fetchCLOBMarket(ctx context.Context, conditionID string) (*clobMarket, error) {
	url := config.PolymarketCLOBBaseURL + "markets/" + conditionID
	var cm clobMarket
	if err := p.getJSON(ctx, url, &cm); err != nil {
		return nil, err
	}
	return &cm, nil
}

// upsertMarkets commits (ReplacingMergeTree upsert) market rows to the storages
// configured for the given coin (per-coin routing, like the other exchanges).
func (p *polymarket) upsertMarkets(ctx context.Context, coin string, rows []storage.PolymarketMarket) error {
	if len(rows) == 0 {
		return nil
	}
	cc := p.coinCfg[coin]
	if cc == nil {
		return nil
	}
	// Route to each enabled storage using that storage's market_commit_buffer.
	type dst struct {
		store storage.Storage
		buf   int
	}
	var dsts []dst
	if cc.terStr {
		dsts = append(dsts, dst{p.ter, p.terMarketBuf})
	}
	if cc.clickHStr {
		dsts = append(dsts, dst{p.clickhouse, p.chMarketBuf})
	}
	for _, d := range dsts {
		for start := 0; start < len(rows); start += d.buf {
			end := start + d.buf
			if end > len(rows) {
				end = len(rows)
			}
			if err := d.store.CommitPolymarketMarket(ctx, rows[start:end]); err != nil {
				if !errors.Is(err, ctx.Err()) {
					logErrStack(err)
				}
				return err
			}
		}
	}
	return nil
}

// --- Websocket reader (RAW stream persistence) ----------------------------- //
//
// The writer keeps NO book state: it persists each Polymarket message verbatim
// (a 'book' full-depth anchor or a 'price_change' delta). The reader reassembles
// the book by replaying anchors+deltas per token and verifying book_hash.

type polyBookMsg struct {
	AssetID   string          `json:"asset_id"`
	Market    string          `json:"market"`
	Timestamp string          `json:"timestamp"`
	Hash      string          `json:"hash"`
	Bids      []polyBookLevel `json:"bids"`
	Asks      []polyBookLevel `json:"asks"`
}

type polyBookLevel struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

type polyPriceChangeMsg struct {
	Market       string            `json:"market"`
	Timestamp    string            `json:"timestamp"`
	PriceChanges []polyPriceChange `json:"price_changes"`
}

type polyPriceChange struct {
	AssetID string `json:"asset_id"`
	Price   string `json:"price"`
	Size    string `json:"size"`
	Side    string `json:"side"`
	Hash    string `json:"hash"`
}

// polyBookData is the JSON stored in polymarket_book.data for a 'book' row.
type polyBookData struct {
	Bids [][2]string `json:"bids"`
	Asks [][2]string `json:"asks"`
}

func (p *polymarket) readWs(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			frame, err := p.ws.Read()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					err = errors.New("context canceled")
				} else {
					if err == io.EOF {
						err = errors.Wrap(err, "connection close by exchange server")
					}
					logErrStack(err)
				}
				return err
			}
			if len(frame) == 0 {
				continue
			}
			trimmed := strings.TrimSpace(string(frame))
			if trimmed == "" || trimmed == "PONG" || trimmed == "pong" {
				continue
			}

			// Frame may be a single event object or a JSON array of events.
			var raws []jsoniter.RawMessage
			if trimmed[0] == '[' {
				if err := jsoniter.UnmarshalFromString(trimmed, &raws); err != nil {
					logErrStack(err)
					continue
				}
			} else {
				raws = []jsoniter.RawMessage{jsoniter.RawMessage(trimmed)}
			}

			for _, raw := range raws {
				var head struct {
					EventType string `json:"event_type"`
				}
				if err := jsoniter.Unmarshal(raw, &head); err != nil {
					continue
				}
				switch head.EventType {
				case "book":
					var msg polyBookMsg
					if err := jsoniter.Unmarshal(raw, &msg); err != nil {
						logErrStack(err)
						continue
					}
					// Persist the full-depth anchor verbatim.
					data := marshalBookData(msg.Bids, msg.Asks)
					if err := p.persistBook(ctx, msg.AssetID, "book", data, msg.Hash, msg.Timestamp); err != nil {
						return err
					}
				case "price_change":
					var msg polyPriceChangeMsg
					if err := jsoniter.Unmarshal(raw, &msg); err != nil {
						logErrStack(err)
						continue
					}
					// One WS price_change can carry changes for both tokens of a
					// market; demux per token so each token's stream stays clean.
					// Preserve first-seen order; book_hash = the token's last change.
					byTok := make(map[string][][3]string)
					hashByTok := make(map[string]string)
					var order []string
					for _, ch := range msg.PriceChanges {
						if _, ok := byTok[ch.AssetID]; !ok {
							order = append(order, ch.AssetID)
						}
						byTok[ch.AssetID] = append(byTok[ch.AssetID], [3]string{ch.Price, ch.Size, ch.Side})
						hashByTok[ch.AssetID] = ch.Hash
					}
					for _, tok := range order {
						data, err := jsoniter.MarshalToString(byTok[tok])
						if err != nil {
							logErrStack(err)
							continue
						}
						if err := p.persistBook(ctx, tok, "price_change", data, hashByTok[tok], msg.Timestamp); err != nil {
							return err
						}
					}
				default:
					// last_trade_price / tick_size_change / etc. — not stored.
				}
			}
		}
	}
}

// trackedMeta returns the grouping ids for a token if it is currently tracked.
func (p *polymarket) trackedMeta(tokenID string) (tokenMeta, bool) {
	p.mu.RLock()
	m, ok := p.meta[tokenID]
	p.mu.RUnlock()
	return m, ok
}

// persistBook stamps a raw stream row (seq + grouping ids) and hands it to the
// batcher with the coin's storage routing flags. Rows for untracked tokens are dropped.
func (p *polymarket) persistBook(ctx context.Context, tokenID, msgType, data, hash, tsStr string) error {
	meta, ok := p.trackedMeta(tokenID)
	if !ok {
		return nil
	}
	cc := p.coinCfg[meta.coin]
	if cc == nil {
		return nil
	}
	item := polyBookItem{
		row: storage.PolymarketBook{
			EventID:     meta.eventID,
			ConditionID: meta.conditionID,
			TokenID:     tokenID,
			Timestamp:   parseMillisTime(tsStr),
			Seq:         atomic.AddUint64(p.seq, 1),
			MsgType:     msgType,
			Data:        data,
			BookHash:    hash,
		},
		ter: cc.terStr,
		ch:  cc.clickHStr,
	}
	select {
	case p.bookOut <- item:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// --- Forced full-book anchors (REST) --------------------------------------- //
//
// The WS 'book' anchor arrives on subscribe and on every market trade, so quiet
// strikes could accumulate a long delta tail with no anchor. This loop forces a
// fresh full-depth anchor per subscribed token periodically. It also runs one
// sweep immediately at startup — since the retry loop rebuilds the connector on
// every reconnect, that guarantees a fresh anchor set right after each reconnect.
func (p *polymarket) fullBookLoop(ctx context.Context) error {
	if err := p.fullBookSweep(ctx); err != nil {
		if errors.Is(err, ctx.Err()) {
			return err
		}
		logErrStack(err)
	}
	tick := time.NewTicker(time.Duration(p.fullBookIntSec) * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if err := p.fullBookSweep(ctx); err != nil {
				if errors.Is(err, ctx.Err()) {
					return err
				}
				logErrStack(err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *polymarket) fullBookSweep(ctx context.Context) error {
	p.mu.RLock()
	tokens := make([]string, 0, len(p.subscribed))
	for tok := range p.subscribed {
		tokens = append(tokens, tok)
	}
	p.mu.RUnlock()
	if len(tokens) == 0 {
		return nil
	}

	throttle := time.NewTicker(polyFullBookThrottleMs * time.Millisecond)
	defer throttle.Stop()
	for _, tok := range tokens {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-throttle.C:
		}
		b, err := p.fetchBook(ctx, tok)
		if err != nil {
			if errors.Is(err, ctx.Err()) {
				return err
			}
			logErrStack(err)
			continue
		}
		data := marshalBookData(b.Bids, b.Asks)
		if err := p.persistBook(ctx, tok, "book", data, b.Hash, b.Timestamp); err != nil {
			return err
		}
	}
	return nil
}

func (p *polymarket) fetchBook(ctx context.Context, tokenID string) (*polyBookMsg, error) {
	url := config.PolymarketCLOBBaseURL + "book?token_id=" + tokenID
	var b polyBookMsg
	if err := p.getJSON(ctx, url, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// bookBatcher accumulates raw book rows into per-storage batches and flushes each
// to its commit worker when it reaches that storage's orders_book_commit_buffer OR
// on a periodic timer (so a strike that goes quiet does not hold its last rows
// hostage in a partial buffer).
// Routing is per-row (per-coin flags), mirroring the other exchanges' per-market
// storage routing: a row goes to terminal and/or clickhouse per its coin's config.
func (p *polymarket) bookBatcher(ctx context.Context) error {
	tick := time.NewTicker(polyBookFlushIntSec * time.Second)
	defer tick.Stop()

	terBatch := make([]storage.PolymarketBook, 0, p.terBookBuf)
	chBatch := make([]storage.PolymarketBook, 0, p.chBookBuf)

	flushTer := func() error {
		if len(terBatch) == 0 {
			return nil
		}
		select {
		case p.wsTerBook <- terBatch:
		case <-ctx.Done():
			return ctx.Err()
		}
		terBatch = make([]storage.PolymarketBook, 0, p.terBookBuf)
		return nil
	}
	flushCh := func() error {
		if len(chBatch) == 0 {
			return nil
		}
		select {
		case p.wsClickHouseBook <- chBatch:
		case <-ctx.Done():
			return ctx.Err()
		}
		chBatch = make([]storage.PolymarketBook, 0, p.chBookBuf)
		return nil
	}

	for {
		select {
		case item := <-p.bookOut:
			if item.ter {
				terBatch = append(terBatch, item.row)
				if len(terBatch) >= p.terBookBuf {
					if err := flushTer(); err != nil {
						return err
					}
				}
			}
			if item.ch {
				chBatch = append(chBatch, item.row)
				if len(chBatch) >= p.chBookBuf {
					if err := flushCh(); err != nil {
						return err
					}
				}
			}
		case <-tick.C:
			if err := flushTer(); err != nil {
				return err
			}
			if err := flushCh(); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// --- helpers --------------------------------------------------------------- //

// marshalBookData encodes a full-depth book (verbatim, unsorted — the reader
// sorts) as the JSON stored in polymarket_book.data for a 'book' row.
func marshalBookData(bids, asks []polyBookLevel) string {
	s, err := jsoniter.MarshalToString(polyBookData{
		Bids: levelsToPairs(bids),
		Asks: levelsToPairs(asks),
	})
	if err != nil {
		return `{"bids":[],"asks":[]}`
	}
	return s
}

func levelsToPairs(levels []polyBookLevel) [][2]string {
	out := make([][2]string, 0, len(levels))
	for _, l := range levels {
		out = append(out, [2]string{l.Price, l.Size})
	}
	return out
}

// defaultOutcomeName is the fallback label when the Gamma outcomes array is absent.
func defaultOutcomeName(idx int) string {
	if idx == 0 {
		return "Yes"
	}
	return "No"
}

// atLeastOne clamps a configured batch size to a minimum of 1 (a 0 would make the
// chunk loop never advance / the batch never flush by count).
func atLeastOne(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

func (p *polymarket) getJSON(ctx context.Context, url string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("polymarket GET %s: status %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := jsoniter.NewDecoder(resp.Body).Decode(target); err != nil {
		return err
	}
	return nil
}

// parseMarket classifies a single market into (market_type, price_low, price_high).
// The price level(s) come from groupItemTitle, and the interval SHAPE is taken from
// the groupItemTitle prefix (the reliable signal, matching Polymarket's own labels),
// with the question as a fallback. The coin is filtered at the event level (see
// eventCoin), so this only decides the price-level type — the same logic fits any coin.
//
// Accepted (in-scope price levels):
//   - "A-B" title / "between $X and $Y"        -> RANGE (low=min, high=max)
//   - "<X" title / "less than $X" / "below $X"  -> RANGE (low=nil, high=X)   [ladder bottom tail]
//   - ">X" title / "greater than $X"            -> RANGE (low=X, high=nil)   [ladder top tail]
//   - plain "X" title with "above $X"           -> ABOVE (low=X, high=nil)   [above-ladder strike]
//
// Note the ">X" / "greater than" form is the OPEN-ENDED TOP of a range ladder
// ("bitcoin-price-on-DATE" events), NOT a standalone ABOVE market — ABOVE-ladder
// strikes use the word "above" with a bare-number title. Both settle identically
// (expiry >= X), but the type must stay consistent within an event so the reader's
// per-event RANGE ladder is not left with a gap.
//
// Skipped (ok=false): TOUCH markets ("reach/hit/dip $X"); "Up or Down" directional;
// and non-price contexts (volatility index, dominance, market cap, comparisons —
// see polyBadContext).
func parseMarket(groupItemTitle, question string) (low *float64, high *float64, mtype string, ok bool) {
	ql := strings.ToLower(question)
	for _, bad := range polyBadContext {
		if strings.Contains(ql, bad) {
			return nil, nil, "", false
		}
	}

	g := strings.TrimSpace(groupItemTitle)
	nums := parseNums(g)

	switch {
	// RANGE bucket: "A-B" title, or "between" question.
	case polyRangeRe.MatchString(g) || strings.Contains(ql, "between"):
		if len(nums) >= 2 {
			l, h := nums[0], nums[1]
			if l > h {
				l, h = h, l
			}
			return &l, &h, "RANGE", true
		}
	// RANGE open-ended lower tail: "<X" title, or "less than"/"below" question.
	case strings.HasPrefix(g, "<") || strings.Contains(ql, "less than") || strings.Contains(ql, "below"):
		if len(nums) >= 1 {
			n := nums[0]
			return nil, &n, "RANGE", true
		}
	// RANGE open-ended upper tail: ">X" title (top of a range ladder).
	case strings.HasPrefix(g, ">"):
		if len(nums) >= 1 {
			n := nums[0]
			return &n, nil, "RANGE", true
		}
	// ABOVE strike: bare-number title with an "above"/"greater than" question.
	case strings.Contains(ql, "above") || strings.Contains(ql, "greater than"):
		if len(nums) >= 1 {
			n := nums[0]
			return &n, nil, "ABOVE", true
		}
	}
	return nil, nil, "", false
}

// parseNums extracts the thousands-separated numbers from a groupItemTitle
// (e.g. "56,000-58,000" -> [56000,58000], ">74,000" -> [74000]).
func parseNums(s string) []float64 {
	matches := polyNumRe.FindAllString(s, -1)
	nums := make([]float64, 0, len(matches))
	for _, m := range matches {
		v, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", ""), 64)
		if err != nil {
			continue
		}
		nums = append(nums, v)
	}
	return nums
}

// parseJSONStringArray parses a JSON-encoded string array (Gamma returns
// clobTokenIds / outcomes as strings-inside-JSON), tolerating a plain empty value.
func parseJSONStringArray(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	if err := jsoniter.UnmarshalFromString(s, &out); err != nil {
		return nil
	}
	return out
}

func parseISOTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05.000Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func parseMillisTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s != "" {
		if ms, err := strconv.ParseInt(s, 10, 64); err == nil && ms > 0 {
			return time.UnixMilli(ms).UTC()
		}
	}
	return time.Now().UTC()
}

// jsonFlexString unmarshals a JSON string OR number into a Go string (Gamma
// event ids come back sometimes as numbers, sometimes as strings).
type jsonFlexString string

func (s *jsonFlexString) UnmarshalJSON(b []byte) error {
	str := strings.TrimSpace(string(b))
	if str == "null" {
		*s = ""
		return nil
	}
	str = strings.Trim(str, "\"")
	*s = jsonFlexString(str)
	return nil
}

// flexFloat unmarshals a JSON number OR numeric-string into a float64.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	str := strings.Trim(strings.TrimSpace(string(b)), "\"")
	if str == "" || str == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return err
	}
	*f = flexFloat(v)
	return nil
}
