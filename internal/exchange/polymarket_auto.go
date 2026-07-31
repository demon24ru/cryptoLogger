package exchange

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/milkywaybrain/cryptogalaxy/internal/config"
	"github.com/milkywaybrain/cryptogalaxy/internal/screener"
	"github.com/milkywaybrain/cryptogalaxy/internal/storage"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Auto-discovery screening mode for the Polymarket connector.
//
// The configured subjects (BTC/SPY/WTI/XAUUSD/weather) name the markets they
// want up front. This mode does the opposite: it sweeps the WHOLE Polymarket
// universe through Gamma, keeps a cheap watch on the plausible ones
// (top-of-book only, one batched REST call per window), runs the canonical
// "zero curvature" screener (internal/screener, ported from
// mm_engine/screener_zero_curvature.py) over each candidate's observation
// window, and DECIDES ITSELF which markets are worth recording in full.
//
// A promoted market is handed to the SAME machinery the configured subjects
// use — dynamic websocket subscription, raw book stream into polymarket_book,
// metadata/resolution into polymarket_market — under the pseudo-subject 'AUTO',
// and is recorded UNTIL RESOLUTION. Every measurement window of every watched
// token is persisted to polymarket_screener, so the metric history survives
// even for markets that never promote.
//
//	UNIVERSE (Gamma) --coarse--> CANDIDATE --observe, screen every window-->
//	  K consecutive PASS windows --> RECORDING (full record, until resolution)
//	  losing PASS only clears in_pass_list (a signal to the bot), never stops
//	  the recording; the budget blocks NEW promotions, it never evicts a
//	  recording that already started.
//
// This file is strictly additive: with connection.polymarket.auto.enabled unset
// the connector behaves exactly as before.

// polyAutoSubject is the pseudo-subject auto-discovered markets are written
// under, in both polymarket_book and polymarket_market.
const polyAutoSubject = "AUTO"

// Screener state machine states (polymarket_screener.state Enum8).
const (
	polyAutoStateCandidate = "CANDIDATE"
	polyAutoStateRecording = "RECORDING"
	polyAutoStateDropped   = "DROPPED"
)

// Defaults for the auto mode (used when the config value is zero).
const (
	polyAutoDefaultScanIntSec       = 900  // universe sweep
	polyAutoDefaultPollIntSec       = 300  // one screener window = the canon's 5-min grid step
	polyAutoDefaultWindowSec        = 7200 // observation window (canon validated 2-6h)
	polyAutoDefaultMaxRecording     = 50
	polyAutoDefaultMaxCandidates    = 400
	polyAutoDefaultHysteresisK      = 2 // consecutive PASS windows to promote
	polyAutoDefaultHysteresisM      = 1 // consecutive fail windows to leave the pass list
	polyAutoDefaultScanMaxPages     = 20
	polyAutoDefaultMinLiquidity     = 5000.0
	polyAutoDefaultMinHoursToExpiry = 6.0
	polyAutoDefaultMaxHoursToExpiry = 720.0
	polyAutoDefaultMaxTickSize      = 0.01
	polyAutoDefaultFinalPhaseFrac   = 0.10

	// polyAutoBooksChunk is the number of tokens per POST /books request. The
	// endpoint accepts up to 500 and caps the body around 50KB; a token id is
	// ~77 digits, so 400 entries is ~38KB — comfortably inside both limits.
	polyAutoBooksChunk = 400

	// polyAutoScanThrottleMs paces the universe sweep so a 20-page scan cannot
	// look like a burst to Gamma.
	polyAutoScanThrottleMs = 250

	// polyAutoMaxRetries / polyAutoBackoffBaseMs drive the backoff used for
	// rate-limited (429) and transient 5xx responses.
	polyAutoMaxRetries    = 4
	polyAutoBackoffBaseMs = 500

	// polyAutoNoLifetimeFinalPhase is the forced-final-phase window used when a
	// market's lifetime cannot be computed (missing startDate): the last day
	// before expiry, matching the "final day of a weekly" rule of §118.
	polyAutoNoLifetimeFinalPhase = 24 * time.Hour

	// polyAutoRecordingGrace is how long past expiry a promoted market may stay
	// unresolved before its budget slot is released. The recording itself is not
	// stopped — only the accounting that would otherwise leak a slot forever.
	polyAutoRecordingGrace = 7 * 24 * time.Hour
)

// autoCandidate is one WATCHED market. The screener runs on the Yes token
// (token_index 0) — the canon screens `yes_token` — but a promotion records
// BOTH tokens, since the reader needs Yes+No to build the effective book.
type autoCandidate struct {
	// Identity / metadata, captured from Gamma at scan time.
	eventID     string
	eventSlug   string
	conditionID string
	category    string
	question    string
	yesTokenID  string
	noTokenID   string
	outcomes    []string
	tickSize    float64
	minOrderSze float64
	createdTs   time.Time
	expiryTs    time.Time
	marketType  string
	priceLow    *float64
	priceHigh   *float64

	// Observation: top-of-book samples covering the observation window (plus a
	// small margin so the window's first grid point has a book to snapshot).
	firstSeen time.Time
	samples   []screener.Sample

	// State machine.
	state      string
	passStreak int
	failStreak int
	inPassList bool
	lastScore  float64
}

// polyAuto owns the auto-discovery mode: the candidate watch list, the state
// machine, and the budget. It borrows the connector for HTTP, storage routing
// and the shared websocket.
type polyAuto struct {
	p   *polymarket
	cfg config.PolymarketAuto

	// Resolved config (defaults applied).
	scanIntSec       int
	pollIntSec       int
	windowSec        int
	maxRecording     int
	maxCandidates    int
	hystK            int
	hystM            int
	scanMaxPages     int
	minLiquidity     float64
	minVolume24hr    float64
	minHoursToExpiry float64
	maxHoursToExpiry float64
	maxTickSize      float64
	finalPhaseFrac   float64
	gates            screener.Gates

	mu    sync.Mutex
	cands map[string]*autoCandidate // yes token id -> candidate (CANDIDATE or RECORDING)
}

// newPolyAuto resolves the auto-mode config onto defaults. The connector is
// bound later (and re-bound on every reconnect) via attach, because the auto
// manager outlives any single connector instance.
func newPolyAuto(cfg config.PolymarketAuto) *polyAuto {
	a := &polyAuto{
		cfg:   cfg,
		cands: make(map[string]*autoCandidate),
	}
	a.scanIntSec = orDefaultInt(cfg.ScanIntSec, polyAutoDefaultScanIntSec)
	a.pollIntSec = orDefaultInt(cfg.PollIntSec, polyAutoDefaultPollIntSec)
	a.windowSec = orDefaultInt(cfg.ObservationWindowSec, polyAutoDefaultWindowSec)
	a.maxRecording = orDefaultInt(cfg.MaxRecording, polyAutoDefaultMaxRecording)
	a.maxCandidates = orDefaultInt(cfg.MaxCandidates, polyAutoDefaultMaxCandidates)
	a.hystK = orDefaultInt(cfg.HysteresisK, polyAutoDefaultHysteresisK)
	a.hystM = orDefaultInt(cfg.HysteresisM, polyAutoDefaultHysteresisM)
	a.scanMaxPages = orDefaultInt(cfg.ScanMaxPages, polyAutoDefaultScanMaxPages)
	a.minLiquidity = orDefaultFloat(cfg.MinLiquidity, polyAutoDefaultMinLiquidity)
	a.minVolume24hr = cfg.MinVolume24hr // 0 = no floor, so no default
	a.minHoursToExpiry = orDefaultFloat(cfg.MinHoursToExpiry, polyAutoDefaultMinHoursToExpiry)
	a.maxHoursToExpiry = orDefaultFloat(cfg.MaxHoursToExpiry, polyAutoDefaultMaxHoursToExpiry)
	a.maxTickSize = orDefaultFloat(cfg.MaxTickSize, polyAutoDefaultMaxTickSize)
	a.finalPhaseFrac = orDefaultFloat(cfg.FinalPhaseFrac, polyAutoDefaultFinalPhaseFrac)

	// Gate thresholds: canon defaults, each overridable independently.
	a.gates = screener.DefaultGates()
	if v := cfg.Gates.MinTwoSided; v > 0 {
		a.gates.MinTwoSided = v
	}
	if v := cfg.Gates.MinSpreadMed; v > 0 {
		a.gates.MinSpreadMed = v
	}
	if v := cfg.Gates.MinDepthMed; v > 0 {
		a.gates.MinDepthMed = v
	}
	if v := cfg.Gates.MaxJumpRate; v > 0 {
		a.gates.MaxJumpRate = v
	}
	if v := cfg.Gates.MaxResStdT; v > 0 {
		a.gates.MaxResStdT = v
	}
	return a
}

func orDefaultInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

func orDefaultFloat(v, def float64) float64 {
	if v <= 0 {
		return def
	}
	return v
}

// --- Connector binding ----------------------------------------------------- //

// attach binds the auto manager to a freshly built connector. Called once per
// (re)connect: the watch list, the observation buffers and the RECORDING set
// all survive, so a promoted market keeps its promotion across a websocket drop.
func (a *polyAuto) attach(p *polymarket) {
	a.mu.Lock()
	a.p = p
	a.mu.Unlock()
}

// conn returns the connector currently bound to this manager.
func (a *polyAuto) conn() *polymarket {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.p
}

// newAutoSubjectCfg builds the subjectCfg entry for the 'AUTO' pseudo-subject.
// It carries no Gamma tag query (the screener chooses the markets) — only the
// storage routing that persistBook and upsertMarkets need. Defaults to
// clickhouse, which is where the analysis side reads from.
func (p *polymarket) newAutoSubjectCfg(cfg config.PolymarketAuto) *polySubjectCfg {
	cc := &polySubjectCfg{name: polyAutoSubject, types: make(map[string]struct{}), auto: true}
	storages := cfg.Storages
	if len(storages) == 0 {
		storages = []string{"clickhouse"}
	}
	for _, str := range storages {
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
		default:
			log.Error().Str("exchange", "polymarket").Str("subject", polyAutoSubject).Str("storage", str).Msg("unknown storage in polymarket.auto.storages (supported: terminal, clickhouse)")
		}
	}
	return cc
}

// tokenOwned reports whether a CONFIGURED (non-auto) subject currently tracks
// this token. The auto mode never touches such a token: recording one asset
// under two subjects would make the two fight over the routing metadata and
// degrade the subject the operator explicitly asked for.
func (p *polymarket) tokenOwned(tokenID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if m, ok := p.meta[tokenID]; ok && m.subject != polyAutoSubject {
		return true
	}
	if kn, ok := p.known[tokenID]; ok && kn.subject != polyAutoSubject {
		return true
	}
	return false
}

// subscribeAuto registers auto-discovered tokens in the connector's working set
// and subscribes them on the live websocket. From this point the existing
// reader, full-book anchor sweep and resolution loop handle them exactly as
// they handle a configured subject's tokens.
func (p *polymarket) subscribeAuto(rows []storage.PolymarketMarket) error {
	toSub := make([]string, 0, len(rows))
	p.mu.Lock()
	for i := range rows {
		r := rows[i]
		p.meta[r.TokenID] = tokenMeta{eventID: r.EventID, conditionID: r.ConditionID, subject: polyAutoSubject}
		p.known[r.TokenID] = &knownMarket{row: r, subject: polyAutoSubject}
		if _, ok := p.subscribed[r.TokenID]; !ok {
			p.subscribed[r.TokenID] = struct{}{}
			toSub = append(toSub, r.TokenID)
		}
	}
	p.mu.Unlock()
	if len(toSub) == 0 {
		return nil
	}
	return p.sendSubscribe(toSub, "subscribe")
}

// resubscribeRecording re-registers and re-subscribes every market that was
// already RECORDING before this (re)connect.
func (a *polyAuto) resubscribeRecording() error {
	now := time.Now().UTC()
	a.mu.Lock()
	p := a.p
	var rows []storage.PolymarketMarket
	for _, c := range a.cands {
		if c.state == polyAutoStateRecording {
			rows = append(rows, c.marketRows(now)...)
		}
	}
	a.mu.Unlock()
	if len(rows) == 0 {
		return nil
	}
	log.Info().Str("exchange", "polymarket").Str("subject", polyAutoSubject).Int("tokens", len(rows)).Msg("re-subscribing auto markets that were recording before the reconnect")
	return p.subscribeAuto(rows)
}

// --- Universe scan (coarse) ------------------------------------------------ //

// scanLoop sweeps the Gamma universe periodically, running one sweep at startup
// so the watch list starts filling immediately.
func (a *polyAuto) scanLoop(ctx context.Context) error {
	if err := a.scanUniverse(ctx); err != nil {
		if errors.Is(err, ctx.Err()) {
			return err
		}
		logErrStack(err)
	}
	tick := time.NewTicker(time.Duration(a.scanIntSec) * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if err := a.scanUniverse(ctx); err != nil {
				if errors.Is(err, ctx.Err()) {
					return err
				}
				// A scan failure is transient (Gamma hiccup / rate limit). The
				// existing watch list and recordings are untouched.
				logErrStack(err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// scanUniverse fetches the top-by-liquidity slice of the active Polymarket
// universe and adds everything that survives the coarse filter to the watch
// list. Gamma refuses offset pagination past ~2100 ("offset too large, use
// /events/keyset"), so the sweep is deliberately BOUNDED and ordered by
// liquidity descending — the tail we cannot reach is the illiquid tail, which
// the coarse filter would drop anyway.
func (a *polyAuto) scanUniverse(ctx context.Context) error {
	now := time.Now().UTC()
	p := a.conn()
	var scanned, added int

	throttle := time.NewTicker(polyAutoScanThrottleMs * time.Millisecond)
	defer throttle.Stop()

	for page := 0; page < a.scanMaxPages; page++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-throttle.C:
		}

		offset := page * polyGammaPageLimit
		url := fmt.Sprintf("%sevents?closed=false&active=true&order=liquidity&ascending=false&limit=%d&offset=%d",
			config.PolymarketGammaBaseURL, polyGammaPageLimit, offset)
		var events []gammaEvent
		if err := a.getJSONRetry(ctx, url, &events); err != nil {
			if errors.Is(err, ctx.Err()) {
				return err
			}
			// Gamma caps offset pagination. Stop the sweep and keep what we got
			// — the pages we did read are the most liquid ones.
			if isGammaOffsetLimit(err) {
				log.Info().Str("exchange", "polymarket").Str("subject", polyAutoSubject).Int("offset", offset).Msg("auto scan reached the Gamma offset limit; keeping the top-by-liquidity slice already scanned")
				break
			}
			return err
		}
		if len(events) == 0 {
			break
		}
		scanned += len(events)

		for i := range events {
			added += a.considerEvent(p, &events[i], now)
		}
		if len(events) < polyGammaPageLimit {
			break
		}
	}

	a.mu.Lock()
	watching, recording := len(a.cands), a.recordingCountLocked()
	a.mu.Unlock()
	log.Info().Str("exchange", "polymarket").Str("subject", polyAutoSubject).
		Int("events_scanned", scanned).Int("new_candidates", added).
		Int("watching", watching).Int("recording", recording).
		Msg("auto universe scan complete")
	return nil
}

// considerEvent applies the coarse filter to one Gamma event and registers the
// surviving markets as candidates. Returns how many were newly added.
func (a *polyAuto) considerEvent(p *polymarket, ev *gammaEvent, now time.Time) int {
	// Event-level activity gates — no CLOB call needed, Gamma already carries
	// these. liquidityClob is the order-book slice of liquidity; use whichever
	// is larger so an event is not dropped for reporting only one of them.
	liq := math.Max(float64(ev.Liquidity), float64(ev.LiquidityClob))
	if liq < a.minLiquidity {
		return 0
	}
	if a.minVolume24hr > 0 && float64(ev.Volume24hr) < a.minVolume24hr {
		return 0
	}

	category := gammaCategory(ev)
	var added int
	for i := range ev.Markets {
		m := &ev.Markets[i]
		if !a.coarseKeepMarket(m, now) {
			continue
		}
		tokenIDs := parseJSONStringArray(m.ClobTokenIDs)
		if len(tokenIDs) < 2 || tokenIDs[0] == "" || tokenIDs[1] == "" {
			continue // binary markets only; we record the Yes/No pair together
		}
		yes, no := tokenIDs[0], tokenIDs[1]

		// Never touch a token a CONFIGURED subject owns — recording the same
		// asset under two subjects would fight over the routing metadata and
		// degrade that subject. The configured subjects always win.
		if p.tokenOwned(yes) || p.tokenOwned(no) {
			continue
		}

		a.mu.Lock()
		if _, ok := a.cands[yes]; ok {
			a.mu.Unlock()
			continue // already watching
		}
		if len(a.cands) >= a.maxCandidates {
			a.mu.Unlock()
			return added // watch list full; the next scan will retry
		}

		low, high, mtype := autoClassify(m)

		a.cands[yes] = &autoCandidate{
			eventID:     string(ev.ID),
			eventSlug:   ev.Slug,
			conditionID: m.ConditionID,
			category:    category,
			question:    m.Question,
			yesTokenID:  yes,
			noTokenID:   no,
			outcomes:    parseJSONStringArray(m.Outcomes),
			tickSize:    float64(m.OrderPriceMinTickSize),
			minOrderSze: float64(m.OrderMinSize),
			createdTs:   parseISOTime(m.StartDate),
			expiryTs:    parseISOTime(m.EndDate),
			marketType:  mtype,
			priceLow:    low,
			priceHigh:   high,
			firstSeen:   now,
			state:       polyAutoStateCandidate,
		}
		a.mu.Unlock()
		added++
	}
	return added
}

// autoClassify decides the market_type and price bounds of an auto-discovered
// market. Anything that is not a recognisable price/level form is GENERIC with
// no bounds — the screener and the bot do not need bounds, and resolution needs
// only winning_outcome.
//
// parseMarket alone is NOT enough here. It was written for a feed already
// narrowed to price markets by the subject's Gamma tags, so it reads any
// numbers it finds as levels. The auto mode scans the whole universe, where
// that misfires badly: "Exact Score: Club Puebla 3 - 2 CD Guadalajara" matches
// the "A-B" range form and would be written as a RANGE with price_low=2,
// price_high=3. So the classification is only trusted when the market actually
// talks about a price or a temperature; everything else stays GENERIC.
func autoClassify(m *gammaMarket) (low, high *float64, mtype string) {
	if !autoPriceLike(m.GroupItemTitle, m.Question) {
		return nil, nil, "GENERIC"
	}
	low, high, mtype, ok := parseMarket(m.GroupItemTitle, m.Question)
	if !ok {
		return nil, nil, "GENERIC"
	}
	return low, high, mtype
}

// autoPriceLike reports whether a market carries a unit that makes a numeric
// level meaningful: a currency symbol (price ladders — "above $60,000",
// "reach $100,000") or a degree sign (weather temperature buckets). A bare
// number without a unit is a score, a count or a jersey number, not a level.
func autoPriceLike(groupItemTitle, question string) bool {
	return strings.ContainsAny(groupItemTitle+question, "$€£¥°")
}

// coarseKeepMarket is the per-market half of the coarse filter: quotable now,
// on a fine enough price grid, and far enough from expiry that a full
// observation window still fits before it settles.
func (a *polyAuto) coarseKeepMarket(m *gammaMarket, now time.Time) bool {
	if !m.Active || m.Closed || !m.EnableOrderBook || !m.AcceptingOrders {
		return false
	}
	tick := float64(m.OrderPriceMinTickSize)
	if tick <= 0 || tick > a.maxTickSize {
		return false
	}
	expiry := parseISOTime(m.EndDate)
	if expiry.IsZero() {
		return false
	}
	hours := expiry.Sub(now).Hours()
	if hours < a.minHoursToExpiry || hours > a.maxHoursToExpiry {
		return false
	}
	// The observation window must fit before expiry, otherwise the market can
	// never be judged and would just occupy a watch slot.
	return expiry.Sub(now) > time.Duration(a.windowSec)*time.Second
}

// gammaCategory derives the market category from the event's Gamma tags. The
// tag list mixes a broad category ("Politics", "Sports", "Crypto") with narrow
// topical tags, so a known top-level label wins; otherwise the first tag label
// is used as-is.
func gammaCategory(ev *gammaEvent) string {
	known := map[string]string{
		"politics": "Politics", "sports": "Sports", "crypto": "Crypto",
		"economics": "Economics", "business": "Business", "culture": "Culture",
		"world": "World", "tech": "Tech", "science": "Science",
		"health": "Health", "weather": "Weather", "elections": "Elections",
	}
	var first string
	for _, t := range ev.Tags {
		label := strings.TrimSpace(t.Label)
		if label == "" {
			continue
		}
		if first == "" {
			first = label
		}
		if c, ok := known[strings.ToLower(label)]; ok {
			return c
		}
	}
	return first
}

// isGammaOffsetLimit reports whether err is Gamma refusing deep offset
// pagination ("offset too large, use /events/keyset", HTTP 422).
func isGammaOffsetLimit(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "offset too large") || strings.Contains(s, "status 422")
}

// --- Candidate poll (fine) ------------------------------------------------- //

// pollLoop runs one screener window per tick: batch top-of-book for every
// watched token, update its ring buffer, score the observation window, advance
// the state machine, promote what earned it, and persist the metric rows.
func (a *polyAuto) pollLoop(ctx context.Context) error {
	tick := time.NewTicker(time.Duration(a.pollIntSec) * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if err := a.pollWindow(ctx); err != nil {
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

func (a *polyAuto) pollWindow(ctx context.Context) error {
	now := time.Now().UTC()

	// Retire candidates that expired or were taken over by a configured
	// subject, then snapshot the tokens still worth polling.
	rows := a.retireStale(now)

	a.mu.Lock()
	tokens := make([]string, 0, len(a.cands))
	for tok := range a.cands {
		tokens = append(tokens, tok)
	}
	a.mu.Unlock()

	if len(tokens) > 0 {
		books, err := a.fetchBooks(ctx, tokens)
		if err != nil {
			if errors.Is(err, ctx.Err()) {
				return err
			}
			// Lost this window's observation; the ring buffers keep what they
			// have and the next window carries on.
			log.Error().Err(err).Str("exchange", "polymarket").Str("subject", polyAutoSubject).Int("tokens", len(tokens)).Msg("auto top-of-book poll failed; skipping this window")
			logErrStack(err)
			return nil
		}
		rows = append(rows, a.scoreWindow(tokens, books, now)...)
	}

	// Promote what earned it, inside the budget.
	promoted := a.selectForPromotion()
	for _, c := range promoted {
		if err := a.promote(ctx, c, now); err != nil {
			if errors.Is(err, ctx.Err()) {
				return err
			}
			log.Error().Err(err).Str("exchange", "polymarket").Str("subject", polyAutoSubject).Str("condition_id", c.conditionID).Msg("auto promotion failed; candidate stays under observation")
			logErrStack(err)
			continue
		}
		// The row for this window was already built as CANDIDATE; restamp it so
		// the promotion is visible at the window it happened.
		for i := range rows {
			if rows[i].TokenID == c.yesTokenID {
				rows[i].State = polyAutoStateRecording
			}
		}
	}

	return a.commitScreener(ctx, rows)
}

// retireStale drops candidates that can no longer be judged: past expiry, or
// claimed by a configured subject since the last window. RECORDING markets are
// NOT retired here — they are released only by resolution (releaseResolved), so
// a promoted market is always recorded to the end. Returns their DROPPED rows.
func (a *polyAuto) retireStale(now time.Time) []storage.PolymarketScreener {
	var rows []storage.PolymarketScreener
	a.mu.Lock()
	defer a.mu.Unlock()
	for tok, c := range a.cands {
		var reason string
		switch {
		case a.p.tokenOwned(tok):
			// A configured subject discovered this token after we started
			// watching it. The subject wins — it keeps recording the token
			// under its own name, we just stop tracking it.
			reason = "claimed by a configured subject"
		case c.state == polyAutoStateRecording:
			// A RECORDING market is normally released only by resolution, so it
			// is always recorded to the end. The one exception is a market that
			// is long past expiry and still has not settled: stop counting it
			// against the budget so a stuck resolution cannot starve the mode.
			// Dropping it from the WATCH list does not stop the recording — the
			// connector keeps it subscribed and the resolution loop keeps
			// polling until it settles.
			if !c.expiryTs.IsZero() && now.After(c.expiryTs.Add(polyAutoRecordingGrace)) {
				reason = "long past expiry without resolution; budget slot released (recording continues)"
			}
		case !c.expiryTs.IsZero() && !c.expiryTs.After(now):
			reason = "expired while under observation"
		}
		if reason == "" {
			continue
		}
		rows = append(rows, a.screenerRow(c, now, polyAutoStateDropped, nil, false, 0, nil))
		delete(a.cands, tok)
		log.Info().Str("exchange", "polymarket").Str("subject", polyAutoSubject).
			Str("event_slug", c.eventSlug).Str("token_id", tok).Str("reason", reason).
			Msg("auto candidate dropped")
	}
	return rows
}

// scoreWindow folds this window's top-of-book observations into each
// candidate's ring buffer, runs the screener over the observation window and
// advances the hysteresis. Returns one screener row per watched token.
func (a *polyAuto) scoreWindow(tokens []string, books map[string]*polyTopOfBook, now time.Time) []storage.PolymarketScreener {
	nowMs := now.UnixMilli()
	t0Ms := nowMs - int64(a.windowSec)*1000
	// Keep a margin of samples BEFORE the window so the window's first grid
	// point snapshots a real book instead of an empty one (the canon replays
	// the whole stream up to t1; we only keep a ring buffer).
	keepFromMs := t0Ms - 2*int64(a.pollIntSec)*1000

	rows := make([]storage.PolymarketScreener, 0, len(tokens))

	a.mu.Lock()
	defer a.mu.Unlock()
	for _, tok := range tokens {
		c := a.cands[tok]
		if c == nil {
			continue // retired between the snapshot and now
		}

		// A token missing from the batch response is an observation too: we
		// looked and there was no book. Recording it as an empty sample is what
		// keeps a market that went dead from coasting on stale good samples.
		s := screener.Sample{TsMs: nowMs}
		if b := books[tok]; b != nil {
			s = b.sample(nowMs)
			if b.tickSize > 0 {
				c.tickSize = b.tickSize // authoritative, and it can change over a market's life
			}
		}
		if len(c.samples) == 0 {
			c.firstSeen = now
		}
		c.samples = append(c.samples, s)
		if i := sort.Search(len(c.samples), func(i int) bool { return c.samples[i].TsMs >= keepFromMs }); i > 1 {
			// Keep one sample before the cutoff as the as-of seed for grid point 0.
			c.samples = append(c.samples[:0], c.samples[i-1:]...)
		}

		tick := c.tickSize
		if tick <= 0 {
			tick = 0.001 // Polymarket's finest grid; only used if Gamma and CLOB both omitted it
		}

		// Judge only once the BUFFER actually covers an observation window.
		// Measured from the oldest retained sample, not from when the market was
		// first seen: a candidate is discovered by the scan up to a scan interval
		// before its first poll, and counting that gap as observed time would make
		// the first window look like a book outage and fail the "книга" gate.
		if nowMs-c.samples[0].TsMs < int64(a.windowSec)*1000 {
			rows = append(rows, a.screenerRow(c, now, c.state, nil, false, 0, nil))
			continue
		}

		m := screener.TokenMetrics(c.samples, t0Ms, nowMs, tick)
		var passed bool
		var score float64
		var fails []string
		if m == nil {
			// Fewer than 8 two-sided grid points: the canon returns None and
			// skips the token. That is exactly the "книга" gate failing (with
			// n≥9 grid points, two_sided is necessarily below 0.9), so record it
			// as such rather than inventing metrics.
			fails = []string{screener.FailBook}
		} else {
			passed, score, fails = screener.GatesAndScoreWith(m, a.gates)
		}

		if passed {
			c.passStreak++
			c.failStreak = 0
		} else {
			c.failStreak++
			c.passStreak = 0
		}
		if c.passStreak >= a.hystK {
			c.inPassList = true
		}
		if c.failStreak >= a.hystM {
			c.inPassList = false
		}
		// §118: the final phase of a dated market is forced out of the pass list
		// regardless of how good the metrics look — that is where the loss lives.
		// The STATE is untouched: a RECORDING market keeps recording to resolution.
		if a.inFinalPhase(c, now) {
			c.inPassList = false
		}
		c.lastScore = score

		rows = append(rows, a.screenerRow(c, now, c.state, m, passed, score, fails))
	}
	return rows
}

// inFinalPhase reports whether now falls in the final FinalPhaseFrac of the
// market's lifetime (REVIEW.md §118: the last ~10% of a weekly — its final day
// — carries the flow, the volatility and the loss). Without a start date the
// rule degrades to "the last day before expiry".
func (a *polyAuto) inFinalPhase(c *autoCandidate, now time.Time) bool {
	if c.expiryTs.IsZero() {
		return false
	}
	if c.createdTs.IsZero() || !c.expiryTs.After(c.createdTs) {
		return now.After(c.expiryTs.Add(-polyAutoNoLifetimeFinalPhase))
	}
	lifetime := c.expiryTs.Sub(c.createdTs)
	return now.After(c.expiryTs.Add(-time.Duration(a.finalPhaseFrac * float64(lifetime))))
}

// screenerRow renders one polymarket_screener row. m may be nil (window not yet
// judged, or not enough data), in which case the metric columns stay zero.
func (a *polyAuto) screenerRow(c *autoCandidate, now time.Time, state string, m *screener.Metrics, passed bool, score float64, fails []string) storage.PolymarketScreener {
	row := storage.PolymarketScreener{
		Ts:          now,
		Subject:     polyAutoSubject,
		Category:    c.category,
		EventID:     c.eventID,
		EventSlug:   c.eventSlug,
		ConditionID: c.conditionID,
		TokenID:     c.yesTokenID,
		ExpiryTs:    c.expiryTs,
		State:       state,
		TickSize:    c.tickSize,
		Score:       score,
		Fails:       screener.FailsCSV(fails),
	}
	if passed {
		row.Passed = 1
	}
	if c.inPassList {
		row.InPassList = 1
	}
	if m != nil {
		row.Mid = m.MidLast
		row.SpreadTicks = m.SpreadMed
		row.Depth = m.DepthMed
		row.TwoSided = m.TwoSided
		row.R2 = m.R2
		row.ResStdT = m.ResStdT
		row.CurvT = m.CurvT
		row.JumpRate = m.JumpRate
	}
	return row
}

// --- Promotion ------------------------------------------------------------- //

// selectForPromotion returns the candidates that earned promotion this window,
// best score first and capped by the budget. Overflow blocks NEW promotions
// only — markets already RECORDING are never evicted, they run to resolution.
func (a *polyAuto) selectForPromotion() []*autoCandidate {
	a.mu.Lock()
	defer a.mu.Unlock()

	slots := a.maxRecording - a.recordingCountLocked()
	if slots <= 0 {
		return nil
	}
	var eligible []*autoCandidate
	for _, c := range a.cands {
		if c.state == polyAutoStateCandidate && c.inPassList {
			eligible = append(eligible, c)
		}
	}
	if len(eligible) == 0 {
		return nil
	}
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].lastScore != eligible[j].lastScore {
			return eligible[i].lastScore > eligible[j].lastScore
		}
		return eligible[i].yesTokenID < eligible[j].yesTokenID // stable
	})
	if len(eligible) > slots {
		log.Info().Str("exchange", "polymarket").Str("subject", polyAutoSubject).
			Int("eligible", len(eligible)).Int("slots", slots).Int("budget", a.maxRecording).
			Msg("auto promotion budget reached; promoting the best-scoring candidates, the rest stay under observation")
		eligible = eligible[:slots]
	}
	return eligible
}

func (a *polyAuto) recordingCountLocked() int {
	n := 0
	for _, c := range a.cands {
		if c.state == polyAutoStateRecording {
			n++
		}
	}
	return n
}

// promote hands a candidate to the normal recording pipeline: it upserts the
// metadata rows for BOTH outcome tokens and subscribes them on the shared
// websocket, after which the existing reader / anchor / resolution loops treat
// them exactly like a configured subject's tokens.
func (a *polyAuto) promote(ctx context.Context, c *autoCandidate, now time.Time) error {
	p := a.conn()
	rows := c.marketRows(now)
	if err := p.upsertMarkets(ctx, polyAutoSubject, rows); err != nil {
		return err
	}
	if err := p.subscribeAuto(rows); err != nil {
		return err
	}

	a.mu.Lock()
	c.state = polyAutoStateRecording
	a.mu.Unlock()

	log.Info().Str("exchange", "polymarket").Str("subject", polyAutoSubject).
		Str("category", c.category).Str("event_slug", c.eventSlug).
		Str("condition_id", c.conditionID).Str("market_type", c.marketType).
		Float64("score", c.lastScore).Time("expiry", c.expiryTs).
		Msg("auto market promoted to RECORDING (recorded until resolution)")
	return nil
}

// marketRows renders the polymarket_market rows for BOTH outcome tokens. The
// reader needs Yes AND No to build the effective book (see REAL_DATA_SCHEMA),
// so a promotion always records the pair even though only the Yes token is
// screened.
func (c *autoCandidate) marketRows(now time.Time) []storage.PolymarketMarket {
	rows := make([]storage.PolymarketMarket, 0, 2)
	for idx, tokenID := range []string{c.yesTokenID, c.noTokenID} {
		outcomeName := defaultOutcomeName(idx)
		if idx < len(c.outcomes) && c.outcomes[idx] != "" {
			outcomeName = c.outcomes[idx]
		}
		rows = append(rows, storage.PolymarketMarket{
			Subject:        polyAutoSubject,
			EventID:        c.eventID,
			EventSlug:      c.eventSlug,
			ConditionID:    c.conditionID,
			TokenID:        tokenID,
			TokenIndex:     uint8(idx),
			Question:       c.question,
			OutcomeName:    outcomeName,
			MarketType:     c.marketType,
			PriceLow:       c.priceLow,
			PriceHigh:      c.priceHigh,
			TickSize:       c.tickSize,
			MinOrderSize:   c.minOrderSze,
			CreatedTs:      c.createdTs,
			ExpiryTs:       c.expiryTs,
			Resolved:       0,
			WinningOutcome: nil,
			UpdatedTs:      now,
			Category:       c.category,
		})
	}
	return rows
}

// releaseResolved is called by the resolution loop when an auto market settles:
// the recording is complete, so the budget slot is freed and the watch entry
// retired. Returns the candidate's final DROPPED row, if it was still tracked.
func (a *polyAuto) releaseResolved(tokenIDs []string) []storage.PolymarketScreener {
	now := time.Now().UTC()
	var rows []storage.PolymarketScreener
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, tok := range tokenIDs {
		c := a.cands[tok]
		if c == nil {
			continue // the No token, or already retired
		}
		rows = append(rows, a.screenerRow(c, now, polyAutoStateDropped, nil, false, 0, nil))
		delete(a.cands, tok)
		log.Info().Str("exchange", "polymarket").Str("subject", polyAutoSubject).
			Str("event_slug", c.eventSlug).Str("condition_id", c.conditionID).
			Msg("auto market resolved; recording complete, budget slot freed")
	}
	return rows
}

// --- Storage --------------------------------------------------------------- //

// commitScreener writes the window's metric rows to the storages configured for
// the auto subject, chunked by each storage's market_commit_buffer.
func (a *polyAuto) commitScreener(ctx context.Context, rows []storage.PolymarketScreener) error {
	if len(rows) == 0 {
		return nil
	}
	p := a.conn()
	cc := p.subjectCfg[polyAutoSubject]
	if cc == nil {
		return nil
	}
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
			if err := d.store.CommitPolymarketScreener(ctx, rows[start:end]); err != nil {
				if !errors.Is(err, ctx.Err()) {
					logErrStack(err)
				}
				return err
			}
		}
	}
	return nil
}

// --- CLOB batch top-of-book ------------------------------------------------ //

// polyTopOfBook is one token's book as returned by POST /books, reduced to the
// touch. Sizes come from the best level only — that is what the canon's
// depth_med measures (bb.size + aa.size).
type polyTopOfBook struct {
	bid, ask         float64
	bidSize, askSize float64
	hasBid, hasAsk   bool
	tickSize         float64
}

func (b *polyTopOfBook) sample(tsMs int64) screener.Sample {
	return screener.Sample{
		TsMs:    tsMs,
		Bid:     b.bid,
		Ask:     b.ask,
		BidSize: b.bidSize,
		AskSize: b.askSize,
		HasBid:  b.hasBid,
		HasAsk:  b.hasAsk,
	}
}

// polyBooksEntry is one element of the POST /books response.
type polyBooksEntry struct {
	AssetID  string          `json:"asset_id"`
	Market   string          `json:"market"`
	Bids     []polyBookLevel `json:"bids"`
	Asks     []polyBookLevel `json:"asks"`
	TickSize flexFloat       `json:"tick_size"`
}

// fetchBooks batch-fetches top-of-book for every watched token. One request per
// polyAutoBooksChunk tokens, so the whole watch list costs a single-digit number
// of calls per window — orders of magnitude inside the rate limit.
func (a *polyAuto) fetchBooks(ctx context.Context, tokens []string) (map[string]*polyTopOfBook, error) {
	out := make(map[string]*polyTopOfBook, len(tokens))
	for start := 0; start < len(tokens); start += polyAutoBooksChunk {
		end := start + polyAutoBooksChunk
		if end > len(tokens) {
			end = len(tokens)
		}
		chunk := tokens[start:end]

		body := make([]map[string]string, 0, len(chunk))
		for _, tok := range chunk {
			body = append(body, map[string]string{"token_id": tok})
		}
		var entries []polyBooksEntry
		if err := a.postJSONRetry(ctx, config.PolymarketCLOBBaseURL+"books", body, &entries); err != nil {
			return nil, err
		}
		for i := range entries {
			e := &entries[i]
			if e.AssetID == "" {
				continue
			}
			out[e.AssetID] = topOfBook(e)
		}
	}
	return out, nil
}

// topOfBook reduces a full book to its touch. The CLOB returns bids ascending
// and asks descending, but rather than depend on that the best level is taken
// by value (highest bid / lowest ask among levels with size > 0) — the same
// thing the canon does after sorting, and immune to an ordering change.
func topOfBook(e *polyBooksEntry) *polyTopOfBook {
	b := &polyTopOfBook{tickSize: float64(e.TickSize)}
	for _, l := range e.Bids {
		price, size, ok := parseLevel(l)
		if !ok {
			continue
		}
		if !b.hasBid || price > b.bid {
			b.bid, b.bidSize, b.hasBid = price, size, true
		}
	}
	for _, l := range e.Asks {
		price, size, ok := parseLevel(l)
		if !ok {
			continue
		}
		if !b.hasAsk || price < b.ask {
			b.ask, b.askSize, b.hasAsk = price, size, true
		}
	}
	return b
}

// parseLevel parses a decimal-string book level. A level with size <= 0 is an
// absent level (the canon's _sorted_levels drops them), not a zero-size one.
func parseLevel(l polyBookLevel) (price, size float64, ok bool) {
	p, err := strconv.ParseFloat(strings.TrimSpace(l.Price), 64)
	if err != nil {
		return 0, 0, false
	}
	s, err := strconv.ParseFloat(strings.TrimSpace(l.Size), 64)
	if err != nil || s <= 0 {
		return 0, 0, false
	}
	return p, s, true
}

// --- HTTP with backoff ----------------------------------------------------- //

// getJSONRetry is getJSON with backoff for rate limiting (429) and transient
// 5xx. Client errors (including Gamma's 422 offset cap) are returned at once —
// retrying them would only burn quota.
func (a *polyAuto) getJSONRetry(ctx context.Context, url string, target interface{}) error {
	var err error
	for attempt := 0; attempt <= polyAutoMaxRetries; attempt++ {
		if attempt > 0 {
			if werr := polyAutoBackoff(ctx, attempt); werr != nil {
				return werr
			}
		}
		err = a.conn().getJSON(ctx, url, target)
		if err == nil {
			return nil
		}
		if errors.Is(err, ctx.Err()) || !isRetryableHTTPErr(err) {
			return err
		}
	}
	return err
}

func (a *polyAuto) postJSONRetry(ctx context.Context, url string, body, target interface{}) error {
	var err error
	for attempt := 0; attempt <= polyAutoMaxRetries; attempt++ {
		if attempt > 0 {
			if werr := polyAutoBackoff(ctx, attempt); werr != nil {
				return werr
			}
		}
		err = a.postJSON(ctx, url, body, target)
		if err == nil {
			return nil
		}
		if errors.Is(err, ctx.Err()) || !isRetryableHTTPErr(err) {
			return err
		}
	}
	return err
}

// polyAutoBackoff waits out an exponential backoff, respecting cancellation.
func polyAutoBackoff(ctx context.Context, attempt int) error {
	d := time.Duration(polyAutoBackoffBaseMs<<(attempt-1)) * time.Millisecond
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// isRetryableHTTPErr reports whether an error from getJSON/postJSON is worth
// retrying: rate limiting, server-side failures, or a transport error.
func isRetryableHTTPErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	if strings.Contains(s, "status 429") {
		return true
	}
	for _, code := range []string{"status 500", "status 502", "status 503", "status 504"} {
		if strings.Contains(s, code) {
			return true
		}
	}
	// No "status N" at all means the request never completed (dial/timeout).
	return !strings.Contains(s, "status ")
}

func (a *polyAuto) postJSON(ctx context.Context, url string, body, target interface{}) error {
	payload, err := jsoniter.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.conn().http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("polymarket POST %s: status %d: %s", url, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return jsoniter.NewDecoder(resp.Body).Decode(target)
}
