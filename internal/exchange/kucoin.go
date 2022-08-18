package exchange

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	kcn "github.com/Kucoin/kucoin-go-sdk"
	jsoniter "github.com/json-iterator/go"
	"github.com/milkywaybrain/cryptogalaxy/internal/config"
	"github.com/milkywaybrain/cryptogalaxy/internal/connector"
	"github.com/milkywaybrain/cryptogalaxy/internal/storage"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// StartKucoin is for starting kucoin exchange functions.
func StartKucoin(appCtx context.Context, markets []config.Market, retry *config.Retry, connCfg *config.Connection) error {

	// If any error occurs or connection is lost, retry the exchange functions with a time gap till it reaches
	// a configured number of retry.
	// Retry counter will be reset back to zero if the elapsed time since the last retry is greater than the configured one.
	var retryCount int
	lastRetryTime := time.Now()

	for {
		err := newKucoin(appCtx, markets, connCfg)
		if err != nil {
			log.Error().Err(err).Str("exchange", "kucoin").Msg("error occurred")
			if retry.Number == 0 {
				return errors.New("not able to connect kucoin exchange. please check the log for details")
			}
			if retry.ResetSec == 0 || time.Since(lastRetryTime).Seconds() < float64(retry.ResetSec) {
				retryCount++
			} else {
				retryCount = 1
			}
			lastRetryTime = time.Now()
			if retryCount > retry.Number {
				err = fmt.Errorf("not able to connect kucoin exchange even after %d retry", retry.Number)
				log.Error().Err(err).Str("exchange", "kucoin").Msg("")
				return err
			}

			log.Error().Str("exchange", "kucoin").Int("retry", retryCount).Msg(fmt.Sprintf("retrying functions in %d seconds", retry.GapSec))
			tick := time.NewTicker(time.Duration(retry.GapSec) * time.Second)
			select {
			case <-tick.C:
				tick.Stop()

			// Return, if there is any error from another exchange.
			case <-appCtx.Done():
				log.Error().Str("exchange", "kucoin").Msg("ctx canceled, return from StartKucoin")
				return appCtx.Err()
			}
		}
	}
}

type wsTickerData struct {
	price       string
	bestAsk     string
	bestAskSize string
	bestBid     string
	bestBidSize string
}

type kucoin struct {
	ws                     connector.Websocket
	rest                   *connector.REST
	connCfg                *config.Connection
	cfgMap                 map[cfgLookupKey]cfgLookupVal
	channelIds             map[int][2]string
	ter                    *storage.Terminal
	clickhouse             *storage.ClickHouse
	wsTerTickers           chan []storage.Ticker
	wsTerTrades            chan []storage.Trade
	wsTerLevel2            chan []storage.Level2
	wsTerOrdersBook        chan []storage.OrdersBook
	wsClickHouseTickers    chan []storage.Ticker
	wsClickHouseTrades     chan []storage.Trade
	wsClickHouseLevel2     chan []storage.Level2
	wsClickHouseOrdersBook chan []storage.OrdersBook
	wsPingIntSec           uint64
}

type wsSubKucoin struct {
	ID             int    `json:"id"`
	Type           string `json:"type"`
	Topic          string `json:"topic"`
	PrivateChannel bool   `json:"privateChannel"`
	Response       bool   `json:"response"`
}

type respKucoin struct {
	ID            string      `json:"id"`
	Topic         string      `json:"topic"`
	Data          interface{} `json:"data"`
	Type          string      `json:"type"`
	mktCommitName string
	data          string
}

type wsConnectRespKucoin struct {
	Code string `json:"code"`
	Data struct {
		Token           string `json:"token"`
		Instanceservers []struct {
			Endpoint          string `json:"endpoint"`
			Protocol          string `json:"protocol"`
			PingintervalMilli int    `json:"pingInterval"`
		} `json:"instanceServers"`
	} `json:"data"`
}

func newKucoin(appCtx context.Context, markets []config.Market, connCfg *config.Connection) error {

	// If any exchange function fails, force all the other functions to stop and return.
	kucoinErrGroup, ctx := errgroup.WithContext(appCtx)

	k := kucoin{connCfg: connCfg}

	err := k.cfgLookup(markets)
	if err != nil {
		return err
	}

	var (
		wsCount   int
		restCount int
		threshold int
	)

	for _, market := range markets {
		for _, info := range market.Info {
			switch info.Connector {
			case "websocket":
				if wsCount == 0 {

					err = k.connectWs(ctx)
					if err != nil {
						return err
					}

					kucoinErrGroup.Go(func() error {
						return k.closeWsConnOnError(ctx)
					})

					kucoinErrGroup.Go(func() error {
						return k.pingWs(ctx)
					})

					kucoinErrGroup.Go(func() error {
						return k.readWs(ctx)
					})

					if k.ter != nil {
						kucoinErrGroup.Go(func() error {
							return WsTickersToStorage(ctx, k.ter, k.wsTerTickers)
						})
						kucoinErrGroup.Go(func() error {
							return WsTradesToStorage(ctx, k.ter, k.wsTerTrades)
						})
						kucoinErrGroup.Go(func() error {
							return WsLevel2ToStorage(ctx, k.ter, k.wsTerLevel2)
						})
					}

					if k.clickhouse != nil {
						kucoinErrGroup.Go(func() error {
							return WsTickersToStorage(ctx, k.clickhouse, k.wsClickHouseTickers)
						})
						kucoinErrGroup.Go(func() error {
							return WsTradesToStorage(ctx, k.clickhouse, k.wsClickHouseTrades)
						})
						kucoinErrGroup.Go(func() error {
							return WsLevel2ToStorage(ctx, k.clickhouse, k.wsClickHouseLevel2)
						})
					}

				}

				key := cfgLookupKey{market: market.ID, channel: info.Channel}
				val := k.cfgMap[key]
				err = k.subWsChannel(market.ID, info.Channel, val.id)
				if err != nil {
					return err
				}

				wsCount++

				// Maximum messages sent to a websocket connection per 10 sec is 100.
				// So on a safer side, this will wait for 20 sec before proceeding once it reaches ~90% of the limit.
				// (including 1 ping message so 90-1)
				threshold++
				if threshold == 89 {
					log.Debug().Str("exchange", "kucoin").Int("count", threshold).Msg("subscribe threshold reached, waiting 20 sec")
					time.Sleep(20 * time.Second)
					threshold = 0
				}

			case "rest":
				if restCount == 0 {
					err = k.connectRest()
					if err != nil {
						return err
					}
				}

				var mktCommitName string
				if market.CommitName != "" {
					mktCommitName = market.CommitName
				} else {
					mktCommitName = market.ID
				}
				channel := info.Channel
				restPingIntSec := info.RESTPingIntSec
				kucoinErrGroup.Go(func() error {
					return k.processREST(ctx, mktCommitName, channel, restPingIntSec)
				})

				restCount++
			}
		}
	}

	err = kucoinErrGroup.Wait()
	if err != nil {
		return err
	}
	return nil
}

func (k *kucoin) cfgLookup(markets []config.Market) error {
	var id int

	// Configurations flat map is prepared for easy lookup later in the app.
	k.cfgMap = make(map[cfgLookupKey]cfgLookupVal)
	k.channelIds = make(map[int][2]string)
	for _, market := range markets {
		var marketCommitName string
		if market.CommitName != "" {
			marketCommitName = market.CommitName
		} else {
			marketCommitName = market.ID
		}
		for _, info := range market.Info {
			key := cfgLookupKey{market: market.ID, channel: info.Channel}
			val := cfgLookupVal{}
			val.connector = info.Connector
			val.wsConsiderIntSec = info.WsConsiderIntSec
			for _, str := range info.Storages {
				switch str {
				case "terminal":
					val.terStr = true
					if k.ter == nil {
						k.ter = storage.GetTerminal()
						k.wsTerTickers = make(chan []storage.Ticker, 1)
						k.wsTerTrades = make(chan []storage.Trade, 1)
						k.wsTerLevel2 = make(chan []storage.Level2, 1)
						k.wsTerOrdersBook = make(chan []storage.OrdersBook, 1)
					}

				case "clickhouse":
					val.clickHouseStr = true
					if k.clickhouse == nil {
						k.clickhouse = storage.GetClickHouse()
						k.wsClickHouseTickers = make(chan []storage.Ticker, 1)
						k.wsClickHouseTrades = make(chan []storage.Trade, 1)
						k.wsClickHouseLevel2 = make(chan []storage.Level2, 1)
						k.wsClickHouseOrdersBook = make(chan []storage.OrdersBook, 1)
					}

				}
			}

			// Channel id is used to identify channel in subscribe success message of websocket server.
			id++
			k.channelIds[id] = [2]string{market.ID, info.Channel}
			val.id = id

			val.mktCommitName = marketCommitName
			k.cfgMap[key] = val
		}
	}
	return nil
}

func (k *kucoin) connectWs(ctx context.Context) error {

	// Do a REST POST request to get the websocket server details.
	resp, err := http.Post(config.KucoinRESTBaseURL+"bullet-public", "", nil)
	if err != nil {
		if !errors.Is(err, ctx.Err()) {
			logErrStack(err)
		}
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("code : %v, status : %v", resp.StatusCode, resp.Status)
	}

	r := wsConnectRespKucoin{}
	if err = jsoniter.NewDecoder(resp.Body).Decode(&r); err != nil {
		logErrStack(err)
		resp.Body.Close()
		return err
	}
	resp.Body.Close()
	if r.Code != "200000" || len(r.Data.Instanceservers) < 1 {
		return errors.New("not able to get websocket server details")
	}

	// Connect to websocket.
	ws, err := connector.NewWebsocket(ctx, &k.connCfg.WS, r.Data.Instanceservers[0].Endpoint+"?token="+r.Data.Token)
	if err != nil {
		if !errors.Is(err, ctx.Err()) {
			logErrStack(err)
		}
		return err
	}
	k.ws = ws

	frame, err := k.ws.Read()
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
		return errors.New("not able to connect websocket server")
	}

	wr := respKucoin{}
	err = jsoniter.Unmarshal(frame, &wr)
	if err != nil {
		logErrStack(err)
		return err
	}

	if wr.Type == "welcome" {
		k.wsPingIntSec = uint64(r.Data.Instanceservers[0].PingintervalMilli) / 1000
		log.Info().Str("exchange", "kucoin").Msg("websocket connected")
	} else {
		return errors.New("not able to connect websocket server")
	}
	return nil
}

// closeWsConnOnError closes websocket connection if there is any error in app context.
// This will unblock all read and writes on websocket.
func (k *kucoin) closeWsConnOnError(ctx context.Context) error {
	<-ctx.Done()
	err := k.ws.Conn.Close()
	if err != nil {
		return err
	}
	return ctx.Err()
}

// pingWs sends ping request to websocket server for every required seconds (~10% earlier to required seconds on a safer side).
func (k *kucoin) pingWs(ctx context.Context) error {
	interval := k.wsPingIntSec * 90 / 100
	tick := time.NewTicker(time.Duration(interval) * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			frame, err := jsoniter.Marshal(map[string]string{
				"id":   strconv.FormatInt(time.Now().Unix(), 10),
				"type": "ping",
			})
			if err != nil {
				logErrStack(err)
				return err
			}
			err = k.ws.Write(frame)
			if err != nil {
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

// subWsChannel sends channel subscription requests to the websocket server.
func (k *kucoin) subWsChannel(market string, channel string, id int) error {
	switch channel {
	case "ticker":
		channel = "/market/ticker:" + market
	case "trade":
		channel = "/market/match:" + market
	case "level2":
		channel = "/market/level2:" + market
	}
	sub := wsSubKucoin{
		ID:             id,
		Type:           "subscribe",
		Topic:          channel,
		PrivateChannel: false,
		Response:       true,
	}
	frame, err := jsoniter.Marshal(&sub)
	if err != nil {
		logErrStack(err)
		return err
	}
	err = k.ws.Write(frame)
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			err = errors.New("context canceled")
		} else {
			logErrStack(err)
		}
		return err
	}
	return nil
}

// readWs reads ticker / trade data from websocket channels.
func (k *kucoin) readWs(ctx context.Context) error {

	// To avoid data race, creating a new local lookup map.
	cfgLookup := make(map[cfgLookupKey]cfgLookupVal, len(k.cfgMap))
	for k, v := range k.cfgMap {
		cfgLookup[k] = v
	}

	cd := commitData{
		terTickers:        make([]storage.Ticker, 0, k.connCfg.Terminal.TickerCommitBuf),
		terTrades:         make([]storage.Trade, 0, k.connCfg.Terminal.TradeCommitBuf),
		terLevel2:         make([]storage.Level2, 0, k.connCfg.Terminal.Level2CommitBuf),
		clickHouseTickers: make([]storage.Ticker, 0, k.connCfg.ClickHouse.TickerCommitBuf),
		clickHouseTrades:  make([]storage.Trade, 0, k.connCfg.ClickHouse.TradeCommitBuf),
		clickHouseLevel2:  make([]storage.Level2, 0, k.connCfg.ClickHouse.Level2CommitBuf),
	}

	storeTick := make(map[string]wsTickerData)

	for {
		select {
		default:
			frame, err := k.ws.Read()
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

			wr := respKucoin{}
			err = jsoniter.Unmarshal(frame, &wr)
			if err != nil {
				logErrStack(err)
				return err
			}

			switch wr.Type {
			case "pong":
			case "ack":
				id, err := strconv.Atoi(wr.ID)
				if err != nil {
					logErrStack(err)
					return err
				}
				log.Debug().Str("exchange", "kucoin").Str("func", "readWs").Str("market", k.channelIds[id][0]).Str("channel", k.channelIds[id][1]).Msg("channel subscribed")
				continue
			case "message":
				s := strings.Split(wr.Topic, ":")
				if len(s) < 2 {
					continue
				}

				switch s[0] {
				case "/market/ticker":
					wr.Topic = "ticker"
				case "/market/match":
					wr.Topic = "trade"
				case "/market/level2":
					wr.Topic = "level2"
				}

				if wr.Topic == "ticker" || wr.Topic == "trade" || wr.Topic == "level2" {

					key := cfgLookupKey{market: s[1], channel: wr.Topic}
					val := cfgLookup[key]
					if val.wsConsiderIntSec == 0 || time.Since(val.wsLastUpdated).Seconds() >= float64(val.wsConsiderIntSec) {
						val.wsLastUpdated = time.Now()
						wr.mktCommitName = s[1]
						cfgLookup[key] = val
					} else {
						continue
					}

					// Consider frame only in configured interval, otherwise ignore it.
					switch wr.Topic {
					case "ticker":

						tick := wr.Data.(map[string]interface{})

						price := tick["price"].(string)
						bestAsk := tick["bestAsk"].(string)
						bestAskSize := tick["bestAskSize"].(string)
						bestBid := tick["bestBid"].(string)
						bestBidSize := tick["bestBidSize"].(string)

						sTick, ok := storeTick[wr.mktCommitName]
						if ok && sTick.price == price && sTick.bestAsk == bestAsk && sTick.bestAskSize == bestAskSize && sTick.bestBid == bestBid && sTick.bestBidSize == bestBidSize {
							continue
						}

						storeTick[wr.mktCommitName] = wsTickerData{
							price:       price,
							bestAsk:     bestAsk,
							bestAskSize: bestAskSize,
							bestBid:     bestBid,
							bestBidSize: bestBidSize,
						}

					case "level2":

						d := wr.Data.(map[string]interface{})
						dd := d["changes"].(map[string]interface{})
						bids := reflect.ValueOf(dd["bids"])
						asks := reflect.ValueOf(dd["asks"])

						badBid := false
						switch bids.Len() {
						case 0:
							badBid = true
						case 1:
							v := reflect.ValueOf(reflect.ValueOf(bids.Index(0).Interface()).Index(0).Interface()).Interface().(string)
							if v == "0" {
								badBid = true
							}
						}

						badAsk := false
						switch asks.Len() {
						case 0:
							badAsk = true
						case 1:
							v := reflect.ValueOf(reflect.ValueOf(asks.Index(0).Interface()).Index(0).Interface()).Interface().(string)
							if v == "0" {
								badAsk = true
							}
						}

						if badBid && badAsk {
							continue
						}
					}

					var data string
					data, err = jsoniter.MarshalToString(wr.Data)
					if err != nil {
						logErrStack(err)
						return err
					}
					wr.data = data

					err := k.processWs(ctx, &wr, &cd)
					if err != nil {
						return err
					}
				}
			}

		// Return, if there is any error from another function or exchange.
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// processWs receives ticker / trade data,
// transforms it to a common ticker / trade store format,
// buffers the same in memory and
// then sends it to different storage systems for commit through go channels.
func (k *kucoin) processWs(ctx context.Context, wr *respKucoin, cd *commitData) error {
	switch wr.Topic {
	case "ticker":
		ticker := storage.Ticker{}
		ticker.MktCommitName = wr.mktCommitName
		ticker.Data = wr.data
		ticker.Timestamp = time.Now().UTC()

		key := cfgLookupKey{market: ticker.MktCommitName, channel: "ticker"}
		val := k.cfgMap[key]
		if val.terStr {
			cd.terTickersCount++
			cd.terTickers = append(cd.terTickers, ticker)
			if cd.terTickersCount == k.connCfg.Terminal.TickerCommitBuf {
				select {
				case k.wsTerTickers <- cd.terTickers:
				case <-ctx.Done():
					return ctx.Err()
				}
				cd.terTickersCount = 0
				cd.terTickers = nil
			}
		}
		if val.clickHouseStr {
			cd.clickHouseTickersCount++
			cd.clickHouseTickers = append(cd.clickHouseTickers, ticker)
			if cd.clickHouseTickersCount == k.connCfg.ClickHouse.TickerCommitBuf {
				select {
				case k.wsClickHouseTickers <- cd.clickHouseTickers:
				case <-ctx.Done():
					return ctx.Err()
				}
				cd.clickHouseTickersCount = 0
				cd.clickHouseTickers = nil
			}
		}
	case "trade":
		trade := storage.Trade{}
		trade.MktCommitName = wr.mktCommitName
		trade.Data = wr.data
		trade.Timestamp = time.Now().UTC()

		key := cfgLookupKey{market: trade.MktCommitName, channel: "trade"}
		val := k.cfgMap[key]
		if val.terStr {
			cd.terTradesCount++
			cd.terTrades = append(cd.terTrades, trade)
			if cd.terTradesCount == k.connCfg.Terminal.TradeCommitBuf {
				select {
				case k.wsTerTrades <- cd.terTrades:
				case <-ctx.Done():
					return ctx.Err()
				}
				cd.terTradesCount = 0
				cd.terTrades = nil
			}
		}
		if val.clickHouseStr {
			cd.clickHouseTradesCount++
			cd.clickHouseTrades = append(cd.clickHouseTrades, trade)
			if cd.clickHouseTradesCount == k.connCfg.ClickHouse.TradeCommitBuf {
				select {
				case k.wsClickHouseTrades <- cd.clickHouseTrades:
				case <-ctx.Done():
					return ctx.Err()
				}
				cd.clickHouseTradesCount = 0
				cd.clickHouseTrades = nil
			}
		}
	case "level2":
		level2 := storage.Level2{}
		level2.MktCommitName = wr.mktCommitName
		level2.Data = wr.data
		level2.Timestamp = time.Now().UTC()

		key := cfgLookupKey{market: level2.MktCommitName, channel: "level2"}
		val := k.cfgMap[key]
		if val.terStr {
			cd.terLevel2Count++
			cd.terLevel2 = append(cd.terLevel2, level2)
			if cd.terLevel2Count == k.connCfg.Terminal.Level2CommitBuf {
				select {
				case k.wsTerLevel2 <- cd.terLevel2:
				case <-ctx.Done():
					return ctx.Err()
				}
				cd.terLevel2Count = 0
				cd.terLevel2 = nil
			}
		}
		if val.clickHouseStr {
			cd.clickHouseLevel2Count++
			cd.clickHouseLevel2 = append(cd.clickHouseLevel2, level2)
			if cd.clickHouseLevel2Count == k.connCfg.ClickHouse.Level2CommitBuf {
				select {
				case k.wsClickHouseLevel2 <- cd.clickHouseLevel2:
				case <-ctx.Done():
					return ctx.Err()
				}
				cd.clickHouseLevel2Count = 0
				cd.clickHouseLevel2 = nil
			}
		}
	}
	return nil
}

func (k *kucoin) connectRest() error {
	rest, err := connector.GetREST()
	if err != nil {
		logErrStack(err)
		return err
	}
	k.rest = rest
	log.Info().Str("exchange", "kucoin").Msg("REST connection setup is done")
	return nil
}

// processREST queries exchange for ticker / trade data through REST API in configured intervals,
// transforms it to a common ticker / trade store format,
// buffers the same in memory and
// then sends it to different storage systems for commit through go channels.
func (k *kucoin) processREST(ctx context.Context, mktCommitName string, channel string, interval int) error {

	cd := commitData{
		terOrdersBooks:       make([]storage.OrdersBook, 0, k.connCfg.Terminal.OrdersBookCommitBuf),
		clickHouseOrdersBook: make([]storage.OrdersBook, 0, k.connCfg.ClickHouse.OrdersBookCommitBuf),
	}

	tick := time.NewTicker(time.Duration(interval) * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:

			switch channel {
			case "ordersbook":
				rsp, err := k.rest.KucoinService.AggregatedFullOrderBookV3(mktCommitName)
				if err != nil {
					logErrStack(err)
					return err
				}

				orbk := kcn.FullOrderBookModel{}
				if err := rsp.ReadData(&orbk); err != nil {
					logErrStack(err)
					return err
				}

				var bids string
				bids, err = jsoniter.MarshalToString(orbk.Bids)
				if err != nil {
					logErrStack(err)
					return err
				}

				var asks string
				asks, err = jsoniter.MarshalToString(orbk.Asks)
				if err != nil {
					logErrStack(err)
					return err
				}

				ordersbook := storage.OrdersBook{
					MktCommitName: mktCommitName,
					Sequence:      orbk.Sequence,
					Bids:          bids,
					Asks:          asks,
					Timestamp:     time.Unix(0, orbk.Time).UTC(),
				}

				key := cfgLookupKey{market: ordersbook.MktCommitName, channel: "ordersbook"}
				val := k.cfgMap[key]
				if val.terStr {
					cd.terOrdersBookCount++
					cd.terOrdersBooks = append(cd.terOrdersBooks, ordersbook)
					if cd.terOrdersBookCount == k.connCfg.Terminal.OrdersBookCommitBuf {
						err := k.ter.CommitOrdersBook(ctx, cd.terOrdersBooks)
						if err != nil {
							if !errors.Is(err, ctx.Err()) {
								logErrStack(err)
							}
							return err
						}
						cd.terOrdersBookCount = 0
						cd.terOrdersBooks = nil
					}
				}

				if val.clickHouseStr {
					cd.clickHouseOrdersBookCount++
					cd.clickHouseOrdersBook = append(cd.clickHouseOrdersBook, ordersbook)
					if cd.clickHouseOrdersBookCount == k.connCfg.ClickHouse.OrdersBookCommitBuf {
						err := k.clickhouse.CommitOrdersBook(ctx, cd.clickHouseOrdersBook)
						if err != nil {
							if !errors.Is(err, ctx.Err()) {
								logErrStack(err)
							}
							return err
						}
						cd.clickHouseOrdersBookCount = 0
						cd.clickHouseOrdersBook = nil
					}
				}

			}

		// Return, if there is any error from another function or exchange.
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
