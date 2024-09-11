package exchange

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"

	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/milkywaybrain/cryptogalaxy/internal/config"
	"github.com/milkywaybrain/cryptogalaxy/internal/connector"
	"github.com/milkywaybrain/cryptogalaxy/internal/storage"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// StartByBitFutures is for starting bybit exchange functions.
func StartByBitFutures(appCtx context.Context, markets []config.Market, retry *config.Retry, connCfg *config.Connection) error {

	// If any error occurs or connection is lost, retry the exchange functions with a time gap till it reaches
	// a configured number of retry.
	// Retry counter will be reset back to zero if the elapsed time since the last retry is greater than the configured one.
	var retryCount int
	lastRetryTime := time.Now()
	var mks []string

	for _, v := range markets {
		mks = append(mks, v.ID)
	}

	for {
		log.Error().Str("exchange", "bybitFutures").Msg(fmt.Sprintf("start %s", strings.Join(mks[:], ",")))
		err := newByBitFutures(appCtx, markets, connCfg)
		if err != nil {
			log.Error().Err(err).Str("exchange", "bybitFutures").Msg("error occurred")
			if retry.Number == 0 {
				return errors.New("not able to connect bybit exchange. please check the log for details")
			}
			if retry.ResetSec == 0 || time.Since(lastRetryTime).Seconds() < float64(retry.ResetSec) {
				retryCount++
			} else {
				retryCount = 1
			}
			lastRetryTime = time.Now()
			if retryCount > retry.Number {
				err = fmt.Errorf("not able to connect bybit exchange even after %d retry", retry.Number)
				log.Error().Err(err).Str("exchange", "bybitFutures").Msg("")
				return err
			}

			log.Error().Str("exchange", "bybitFutures").Int("retry", retryCount).Msg(fmt.Sprintf("retrying functions in %d seconds %s", retry.GapSec, strings.Join(mks[:], ",")))
			tick := time.NewTicker(time.Duration(retry.GapSec) * time.Second)
			select {
			case <-tick.C:
				tick.Stop()

			// Return, if there is any error from another exchange.
			case <-appCtx.Done():
				log.Error().Str("exchange", "bybitFutures").Msg("ctx canceled, return from Startbybit")
				return appCtx.Err()
			}
		}
	}
}

type bybitFutures struct {
	ws                      connector.Websocket
	rest                    *connector.REST
	connCfg                 *config.Connection
	cfgMap                  map[cfgLookupKey]cfgLookupVal
	channelIds              map[int][2]string
	ter                     *storage.Terminal
	clickhouse              *storage.ClickHouse
	wsTerTickers            chan []storage.Ticker
	wsTerTrades             chan []storage.Trade
	wsTerLevel2             chan []storage.Level2
	wsTerOrdersBooks        chan []storage.OrdersBook
	wsClickHouseTickers     chan []storage.Ticker
	wsClickHouseTrades      chan []storage.Trade
	wsClickHouseLevel2      chan []storage.Level2
	wsClickHouseOrdersBooks chan []storage.OrdersBook
}

//type restRespByBit struct {
//	TradeID uint64 `json:"id"`
//	Maker   bool   `json:"isBuyerMaker"`
//	Qty     string `json:"qty"`
//	Price   string `json:"price"`
//	Time    int64  `json:"time"`
//}

func newByBitFutures(appCtx context.Context, markets []config.Market, connCfg *config.Connection) error {

	// If any exchange function fails, force all the other functions to stop and return.
	bybitErrGroup, ctx := errgroup.WithContext(appCtx)

	b := bybitFutures{connCfg: connCfg}

	err := b.cfgLookup(markets)
	if err != nil {
		return err
	}

	var (
		wsCount int
		//restCount int
		threshold int
	)

	for _, market := range markets {
		for _, info := range market.Info {
			switch info.Connector {
			case "websocket":
				if wsCount == 0 {

					err = b.connectWs(ctx)
					if err != nil {
						return err
					}

					bybitErrGroup.Go(func() error {
						return b.closeWsConnOnError(ctx)
					})

					bybitErrGroup.Go(func() error {
						return b.pingWs(ctx)
					})

					bybitErrGroup.Go(func() error {
						return b.readWs(ctx)
					})

					if b.ter != nil {
						bybitErrGroup.Go(func() error {
							return WsTickersToStorage(ctx, b.ter, b.wsTerTickers)
						})
						bybitErrGroup.Go(func() error {
							return WsTradesToStorage(ctx, b.ter, b.wsTerTrades)
						})
						bybitErrGroup.Go(func() error {
							return WsLevel2ToStorage(ctx, b.ter, b.wsTerLevel2)
						})
						bybitErrGroup.Go(func() error {
							return WsOrdersBookToStorage(ctx, b.ter, b.wsTerOrdersBooks)
						})
					}

					if b.clickhouse != nil {
						bybitErrGroup.Go(func() error {
							return WsTickersToStorage(ctx, b.clickhouse, b.wsClickHouseTickers)
						})
						bybitErrGroup.Go(func() error {
							return WsTradesToStorage(ctx, b.clickhouse, b.wsClickHouseTrades)
						})
						bybitErrGroup.Go(func() error {
							return WsLevel2ToStorage(ctx, b.clickhouse, b.wsClickHouseLevel2)
						})
						bybitErrGroup.Go(func() error {
							return WsOrdersBookToStorage(ctx, b.clickhouse, b.wsClickHouseOrdersBooks)
						})
					}

				}

				key := cfgLookupKey{market: market.ID, channel: info.Channel}
				val := b.cfgMap[key]
				err = b.subWsChannel(market.ID, info.Channel, val.id)
				if err != nil {
					return err
				}
				wsCount++

				// Maximum messages sent to a websocket connection per sec is 5.
				// So on a safer side, this will wait for 2 sec before proceeding once it reaches ~90% of the limit.
				// (including 1 pong frame (sent by ws library), so 4-1)
				threshold++
				if threshold == 3 {
					log.Debug().Str("exchange", "bybitFutures").Int("count", threshold).Msg("subscribe threshold reached, waiting 2 sec")
					time.Sleep(2 * time.Second)
					threshold = 0
				}

				//case "rest":
				//	if restCount == 0 {
				//		err = b.connectRest()
				//		if err != nil {
				//			return err
				//		}
				//	}
				//
				//	var mktCommitName string
				//	if market.CommitName != "" {
				//		mktCommitName = market.CommitName
				//	} else {
				//		mktCommitName = market.ID
				//	}
				//	mktID := market.ID
				//	channel := info.Channel
				//	restPingIntSec := info.RESTPingIntSec
				//	bybitErrGroup.Go(func() error {
				//		return b.processREST(ctx, mktID, mktCommitName, channel, restPingIntSec)
				//	})
				//
				//	restCount++
			}
		}
	}

	err = bybitErrGroup.Wait()
	if err != nil {
		return err
	}
	return nil
}

func (b *bybitFutures) cfgLookup(markets []config.Market) error {
	var id int

	// Configurations flat map is prepared for easy lookup later in the app.
	b.cfgMap = make(map[cfgLookupKey]cfgLookupVal)
	b.channelIds = make(map[int][2]string)
	for _, market := range markets {
		var mktCommitName string
		if market.CommitName != "" {
			mktCommitName = market.CommitName
		} else {
			mktCommitName = market.ID
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
					if b.ter == nil {
						b.ter = storage.GetTerminal()
						b.wsTerTickers = make(chan []storage.Ticker, 1)
						b.wsTerTrades = make(chan []storage.Trade, 1)
						b.wsTerLevel2 = make(chan []storage.Level2, 1)
						b.wsTerOrdersBooks = make(chan []storage.OrdersBook, 1)
					}
				case "clickhouse":
					val.clickHouseStr = true
					if b.clickhouse == nil {
						b.clickhouse = storage.GetClickHouse()
						b.wsClickHouseTickers = make(chan []storage.Ticker, 1)
						b.wsClickHouseTrades = make(chan []storage.Trade, 1)
						b.wsClickHouseLevel2 = make(chan []storage.Level2, 1)
						b.wsClickHouseOrdersBooks = make(chan []storage.OrdersBook, 1)
					}
				}
			}

			// Channel id is used to identify channel in subscribe success message of websocket server.
			id++
			b.channelIds[id] = [2]string{market.ID, info.Channel}
			val.id = id

			val.mktCommitName = mktCommitName
			b.cfgMap[key] = val
		}
	}
	return nil
}

func (b *bybitFutures) connectWs(ctx context.Context) error {
	ws, err := connector.NewWebsocket(ctx, &b.connCfg.WS, config.BybitWebsocketURL+"/linear")
	if err != nil {
		if !errors.Is(err, ctx.Err()) {
			logErrStack(err)
		}
		return err
	}
	b.ws = ws
	log.Info().Str("exchange", "bybitFutures").Msg("websocket connected")
	return nil
}

// closeWsConnOnError closes websocket connection if there is any error in app context.
// This will unblock all read and writes on websocket.
func (b *bybitFutures) closeWsConnOnError(ctx context.Context) error {
	<-ctx.Done()
	err := b.ws.Conn.Close()
	if err != nil {
		return err
	}
	return ctx.Err()
}

// subWsChannel sends channel subscription requests to the websocket server.
func (b *bybitFutures) subWsChannel(market string, channel string, id int) error {
	if channel == "trade" {
		channel = "publicTrade"
	}
	if channel == "ticker" {
		channel = "orderbook.1"
	}
	if channel == "ordersbook" {
		channel = "orderbook.50"
	}
	channel = channel + "." + strings.ToUpper(market)
	sub := wsSubByBit{
		Op:   "subscribe",
		Args: [1]string{channel},
		ID:   id,
	}
	frame, err := jsoniter.Marshal(&sub)
	if err != nil {
		logErrStack(err)
		return err
	}
	err = b.ws.Write(frame)
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

// pingWs sends ping request to websocket server for every required seconds (~10% earlier to required seconds on a safer side).
func (b *bybitFutures) pingWs(ctx context.Context) error {
	tick := time.NewTicker(time.Duration(20) * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			frame, err := jsoniter.Marshal(map[string]string{
				"req_id": strconv.FormatInt(time.Now().Unix(), 10),
				"op":     "ping",
			})
			if err != nil {
				logErrStack(err)
				return err
			}
			err = b.ws.Write(frame)
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

// readWs reads ticker / trade data from websocket channels.
func (b *bybitFutures) readWs(ctx context.Context) error {

	// To avoid data race, creating a new local lookup map.
	cfgLookup := make(map[cfgLookupKey]cfgLookupVal, len(b.cfgMap))
	for k, v := range b.cfgMap {
		cfgLookup[k] = v
	}

	cd := commitData{
		terTickers:           make([]storage.Ticker, 0, b.connCfg.Terminal.TickerCommitBuf),
		terTrades:            make([]storage.Trade, 0, b.connCfg.Terminal.TradeCommitBuf),
		terLevel2:            make([]storage.Level2, 0, b.connCfg.Terminal.Level2CommitBuf),
		terOrdersBooks:       make([]storage.OrdersBook, 0, b.connCfg.Terminal.OrdersBookCommitBuf),
		clickHouseTickers:    make([]storage.Ticker, 0, b.connCfg.ClickHouse.TickerCommitBuf),
		clickHouseTrades:     make([]storage.Trade, 0, b.connCfg.ClickHouse.TradeCommitBuf),
		clickHouseLevel2:     make([]storage.Level2, 0, b.connCfg.ClickHouse.Level2CommitBuf),
		clickHouseOrdersBook: make([]storage.OrdersBook, 0, b.connCfg.ClickHouse.OrdersBookCommitBuf),
	}

	storeTick := make(map[string]storeTickerData)

	for {
		select {
		default:
			frame, err := b.ws.Read()
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

			wr := wsRespByBit{}
			err = jsoniter.Unmarshal(frame, &wr)
			if err != nil {
				log.Debug().Str("exchange", "bybitFutures").Str("func", "readWs").Msg(string(frame))
				logErrStack(err)
				return err
			}

			stArr := strings.Split(wr.Event, ".")
			wr.Symbol = stArr[len(stArr)-1]

			switch stArr[0] {
			case "orderbook":
				if stArr[1] == "1" {
					wr.Event = "ticker"
				} else {
					wr.Event = "ordersbook"
				}
			case "publicTrade":
				wr.Event = "trade"
			}

			// Consider frame only in configured interval, otherwise ignore it.
			if wr.Event == "ticker" || wr.Event == "trade" || wr.Event == "ordersbook" {

				key := cfgLookupKey{market: wr.Symbol, channel: wr.Event}
				val := cfgLookup[key]
				if val.wsConsiderIntSec == 0 || time.Since(val.wsLastUpdated).Seconds() >= float64(val.wsConsiderIntSec) {
					val.wsLastUpdated = time.Now()
					wr.mktCommitName = wr.Symbol
					cfgLookup[key] = val
				} else {
					continue
				}

				switch wr.Event {
				case "ticker":
					wrt := wsRespLevelBookByBit{}
					err = jsoniter.Unmarshal(frame, &wrt)
					if err != nil {
						log.Debug().Str("exchange", "bybitFutures").Str("func", "readWs").Msg(string(frame))
						logErrStack(err)
						return err
					}

					var BestAsk, BestAskSize, BestBid, BestBidSize string
					sTick, ok := storeTick[wr.mktCommitName]
					if ok {
						BestAsk = sTick.bestAsk
						BestAskSize = sTick.bestAskSize
						BestBid = sTick.bestBid
						BestBidSize = sTick.bestBidSize
					}

					if len(wrt.Data.Bids) > 0 {
						if len(wrt.Data.Bids) == 2 {
							if wrt.Data.Bids[0][1] != "0" {
								BestBid = wrt.Data.Bids[0][0]
								BestBidSize = wrt.Data.Bids[0][1]
							} else {
								BestBid = wrt.Data.Bids[1][0]
								BestBidSize = wrt.Data.Bids[1][1]
							}
						} else {
							BestBid = wrt.Data.Bids[0][0]
							BestBidSize = wrt.Data.Bids[0][1]
						}
					}

					if len(wrt.Data.Asks) > 0 {
						if len(wrt.Data.Asks) == 2 {
							if wrt.Data.Asks[0][1] != "0" {
								BestAsk = wrt.Data.Asks[0][0]
								BestAskSize = wrt.Data.Asks[0][1]
							} else {
								BestAsk = wrt.Data.Asks[1][0]
								BestAskSize = wrt.Data.Asks[1][1]
							}
						} else {
							BestAsk = wrt.Data.Asks[0][0]
							BestAskSize = wrt.Data.Asks[0][1]
						}
					}

					if ok && sTick.bestAsk == BestAsk && sTick.bestAskSize == BestAskSize && sTick.bestBid == BestBid && sTick.bestBidSize == BestBidSize {
						continue
					}

					storeTick[wr.mktCommitName] = storeTickerData{
						bestAsk:     BestAsk,
						bestAskSize: BestAskSize,
						bestBid:     BestBid,
						bestBidSize: BestBidSize,
					}

					wr.data, err = jsoniter.MarshalToString(commitTicker{
						BestAsk:     BestAsk,
						BestAskSize: BestAskSize,
						BestBid:     BestBid,
						BestBidSize: BestBidSize,
					})
					if err != nil {
						logErrStack(err)
						return err
					}
				case "trade":
					wrt := wsRespTradeByBit{}
					err = jsoniter.Unmarshal(frame, &wrt)
					if err != nil {
						log.Debug().Str("exchange", "bybitFutures").Str("func", "readWs").Msg(string(frame))
						logErrStack(err)
						return err
					}

					rt := []commitTrade{}
					for _, item := range wrt.Data {
						rt = append(rt, commitTrade{
							Side:  strings.ToLower(item.Side),
							Size:  item.Size,
							Price: item.Price,
						})
					}

					wr.data, err = jsoniter.MarshalToString(rt)
					if err != nil {
						logErrStack(err)
						return err
					}
				case "ordersbook":
					wrt := wsRespLevelBookByBit{}
					err = jsoniter.Unmarshal(frame, &wrt)
					if err != nil {
						log.Debug().Str("exchange", "bybitFutures").Str("func", "readWs").Msg(string(frame))
						logErrStack(err)
						return err
					}
					if wrt.Type == "delta" {
						wr.Event = "level2"
					}

					wr.data, err = jsoniter.MarshalToString(wrt.Data.Bids)
					if err != nil {
						logErrStack(err)
						return err
					}
					wr.dataAsk, err = jsoniter.MarshalToString(wrt.Data.Asks)
					if err != nil {
						logErrStack(err)
						return err
					}
				}

				err := b.processWs(ctx, &wr, &cd)
				if err != nil {
					return err
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
func (b *bybitFutures) processWs(ctx context.Context, wr *wsRespByBit, cd *commitData) error {
	switch wr.Event {
	case "ticker":
		ticker := storage.Ticker{}
		ticker.ExchangeName = "bybit"
		ticker.MktCommitName = wr.mktCommitName + "F"
		ticker.Data = wr.data
		ticker.Timestamp = time.Now().UTC()

		key := cfgLookupKey{market: wr.mktCommitName, channel: "ticker"}
		val := b.cfgMap[key]
		if val.terStr {
			cd.terTickersCount++
			cd.terTickers = append(cd.terTickers, ticker)
			if cd.terTickersCount == b.connCfg.Terminal.TickerCommitBuf {
				select {
				case b.wsTerTickers <- cd.terTickers:
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
			if cd.clickHouseTickersCount == b.connCfg.ClickHouse.TickerCommitBuf {
				select {
				case b.wsClickHouseTickers <- cd.clickHouseTickers:
				case <-ctx.Done():
					return ctx.Err()
				}
				cd.clickHouseTickersCount = 0
				cd.clickHouseTickers = nil
			}
		}
	case "trade":
		trade := storage.Trade{}
		trade.ExchangeName = "bybit"
		trade.MktCommitName = wr.mktCommitName + "F"
		trade.Data = wr.data
		trade.Timestamp = time.Now().UTC()

		key := cfgLookupKey{market: wr.mktCommitName, channel: "trade"}
		val := b.cfgMap[key]
		if val.terStr {
			cd.terTradesCount++
			cd.terTrades = append(cd.terTrades, trade)
			if cd.terTradesCount == b.connCfg.Terminal.TradeCommitBuf {
				select {
				case b.wsTerTrades <- cd.terTrades:
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
			if cd.clickHouseTradesCount == b.connCfg.ClickHouse.TradeCommitBuf {
				select {
				case b.wsClickHouseTrades <- cd.clickHouseTrades:
				case <-ctx.Done():
					return ctx.Err()
				}
				cd.clickHouseTradesCount = 0
				cd.clickHouseTrades = nil
			}
		}
	case "ordersbook":
		ordersbook := storage.OrdersBook{}
		ordersbook.ExchangeName = "bybit"
		ordersbook.MktCommitName = wr.mktCommitName + "F"
		ordersbook.Bids = wr.data
		ordersbook.Asks = wr.dataAsk
		ordersbook.Timestamp = time.Now().UTC()

		key := cfgLookupKey{market: wr.mktCommitName, channel: "ordersbook"}
		val := b.cfgMap[key]
		if val.terStr {
			cd.terOrdersBookCount++
			cd.terOrdersBooks = append(cd.terOrdersBooks, ordersbook)
			if cd.terOrdersBookCount == b.connCfg.Terminal.OrdersBookCommitBuf {
				select {
				case b.wsTerOrdersBooks <- cd.terOrdersBooks:
				case <-ctx.Done():
					return ctx.Err()
				}
				cd.terOrdersBookCount = 0
				cd.terOrdersBooks = nil
			}
		}
		if val.clickHouseStr {
			cd.clickHouseOrdersBookCount++
			cd.clickHouseOrdersBook = append(cd.clickHouseOrdersBook, ordersbook)
			if cd.clickHouseOrdersBookCount == b.connCfg.ClickHouse.OrdersBookCommitBuf {
				select {
				case b.wsClickHouseOrdersBooks <- cd.clickHouseOrdersBook:
				case <-ctx.Done():
					return ctx.Err()
				}
				cd.clickHouseOrdersBookCount = 0
				cd.clickHouseOrdersBook = nil
			}
		}
	case "level2":
		level2 := storage.Level2{}
		level2.ExchangeName = "bybit"
		level2.MktCommitName = wr.mktCommitName + "F"
		level2.Bids = wr.data
		level2.Asks = wr.dataAsk
		level2.Timestamp = time.Now().UTC()

		key := cfgLookupKey{market: wr.mktCommitName, channel: "ordersbook"}
		val := b.cfgMap[key]
		if val.terStr {
			cd.terLevel2Count++
			cd.terLevel2 = append(cd.terLevel2, level2)
			if cd.terLevel2Count == b.connCfg.Terminal.Level2CommitBuf {
				select {
				case b.wsTerLevel2 <- cd.terLevel2:
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
			if cd.clickHouseLevel2Count == b.connCfg.ClickHouse.Level2CommitBuf {
				select {
				case b.wsClickHouseLevel2 <- cd.clickHouseLevel2:
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

//func (b *bybitFutures) connectRest() error {
//	rest, err := connector.GetREST()
//	if err != nil {
//		logErrStack(err)
//		return err
//	}
//	b.rest = rest
//	log.Info().Str("exchange", "bybitFutures").Msg("REST connection setup is done")
//	return nil
//}

// processREST queries exchange for ticker / trade data through REST API in configured intervals,
// transforms it to a common ticker / trade store format,
// buffers the same in memory and
// then sends it to different storage systems for commit through go channels.
//func (b *bybitFutures) processREST(ctx context.Context, mktID string, mktCommitName string, channel string, interval int) error {
//	var (
//		req *http.Request
//		q   url.Values
//		err error
//	)
//
//	cd := commitData{
//		terTickers:        make([]storage.Ticker, 0, b.connCfg.Terminal.TickerCommitBuf),
//		terTrades:         make([]storage.Trade, 0, b.connCfg.Terminal.TradeCommitBuf),
//		clickHouseTickers: make([]storage.Ticker, 0, b.connCfg.ClickHouse.TickerCommitBuf),
//		clickHouseTrades:  make([]storage.Trade, 0, b.connCfg.ClickHouse.TradeCommitBuf),
//	}
//
//	switch channel {
//	case "ticker":
//		req, err = b.rest.Request(ctx, "GET", config.BybitRESTBaseURL+"ticker/price")
//		if err != nil {
//			if !errors.Is(err, ctx.Err()) {
//				logErrStack(err)
//			}
//			return err
//		}
//		q = req.URL.Query()
//		q.Add("symbol", mktID)
//	case "trade":
//		req, err = b.rest.Request(ctx, "GET", config.BybitRESTBaseURL+"trades")
//		if err != nil {
//			if !errors.Is(err, ctx.Err()) {
//				logErrStack(err)
//			}
//			return err
//		}
//		q = req.URL.Query()
//		q.Add("symbol", mktID)
//
//		// Querying for 100 trades.
//		// If the configured interval gap is big, then maybe it will not return all the trades
//		// and if the gap is too small, maybe it will return duplicate ones.
//		// Better to use websocket.
//		q.Add("limit", strconv.Itoa(100))
//	}
//
//	tick := time.NewTicker(time.Duration(interval) * time.Second)
//	defer tick.Stop()
//	for {
//		select {
//		case <-tick.C:
//
//			switch channel {
//			case "ticker":
//				req.URL.RawQuery = q.Encode()
//				resp, err := b.rest.Do(req)
//				if err != nil {
//					if !errors.Is(err, ctx.Err()) {
//						logErrStack(err)
//					}
//					return err
//				}
//
//				rr := restRespByBit{}
//				if err = jsoniter.NewDecoder(resp.Body).Decode(&rr); err != nil {
//					logErrStack(err)
//					resp.Body.Close()
//					return err
//				}
//				resp.Body.Close()
//
//				price, err := strconv.ParseFloat(rr.Price, 64)
//				if err != nil {
//					logErrStack(err)
//					return err
//				}
//
//				ticker := storage.Ticker{
//					Exchange:      "bybitFutures",
//					MktID:         mktID,
//					MktCommitName: mktCommitName,
//					Price:         price,
//					Timestamp:     time.Now().UTC(),
//				}
//
//				key := cfgLookupKey{market: ticker.MktID, channel: "ticker"}
//				val := b.cfgMap[key]
//				if val.terStr {
//					cd.terTickersCount++
//					cd.terTickers = append(cd.terTickers, ticker)
//					if cd.terTickersCount == b.connCfg.Terminal.TickerCommitBuf {
//						err := b.ter.CommitTickers(ctx, cd.terTickers)
//						if err != nil {
//							if !errors.Is(err, ctx.Err()) {
//								logErrStack(err)
//							}
//							return err
//						}
//						cd.terTickersCount = 0
//						cd.terTickers = nil
//					}
//				}
//				if val.clickHouseStr {
//					cd.clickHouseTickersCount++
//					cd.clickHouseTickers = append(cd.clickHouseTickers, ticker)
//					if cd.clickHouseTickersCount == b.connCfg.ClickHouse.TickerCommitBuf {
//						err := b.clickhouse.CommitTickers(ctx, cd.clickHouseTickers)
//						if err != nil {
//							return err
//						}
//						cd.clickHouseTickersCount = 0
//						cd.clickHouseTickers = nil
//					}
//				}
//			case "trade":
//				req.URL.RawQuery = q.Encode()
//				resp, err := b.rest.Do(req)
//				if err != nil {
//					if !errors.Is(err, ctx.Err()) {
//						logErrStack(err)
//					}
//					return err
//				}
//
//				rr := []restRespByBit{}
//				if err := jsoniter.NewDecoder(resp.Body).Decode(&rr); err != nil {
//					logErrStack(err)
//					resp.Body.Close()
//					return err
//				}
//				resp.Body.Close()
//
//				for i := range rr {
//					r := rr[i]
//					var side string
//					if r.Maker {
//						side = "buy"
//					} else {
//						side = "sell"
//					}
//
//					size, err := strconv.ParseFloat(r.Qty, 64)
//					if err != nil {
//						logErrStack(err)
//						return err
//					}
//
//					price, err := strconv.ParseFloat(r.Price, 64)
//					if err != nil {
//						logErrStack(err)
//						return err
//					}
//
//					// Time sent is in milliseconds.
//					timestamp := time.Unix(0, r.Time*int64(time.Millisecond)).UTC()
//
//					trade := storage.Trade{
//						Exchange:      "bybitFutures",
//						MktID:         mktID,
//						MktCommitName: mktCommitName,
//						TradeID:       strconv.FormatUint(r.TradeID, 10),
//						Side:          side,
//						Size:          size,
//						Price:         price,
//						Timestamp:     timestamp,
//					}
//
//					key := cfgLookupKey{market: trade.MktID, channel: "trade"}
//					val := b.cfgMap[key]
//					if val.terStr {
//						cd.terTradesCount++
//						cd.terTrades = append(cd.terTrades, trade)
//						if cd.terTradesCount == b.connCfg.Terminal.TradeCommitBuf {
//							err := b.ter.CommitTrades(ctx, cd.terTrades)
//							if err != nil {
//								if !errors.Is(err, ctx.Err()) {
//									logErrStack(err)
//								}
//								return err
//							}
//							cd.terTradesCount = 0
//							cd.terTrades = nil
//						}
//					}
//					if val.clickHouseStr {
//						cd.clickHouseTradesCount++
//						cd.clickHouseTrades = append(cd.clickHouseTrades, trade)
//						if cd.clickHouseTradesCount == b.connCfg.ClickHouse.TradeCommitBuf {
//							err := b.clickhouse.CommitTrades(ctx, cd.clickHouseTrades)
//							if err != nil {
//								if !errors.Is(err, ctx.Err()) {
//									logErrStack(err)
//								}
//								return err
//							}
//							cd.clickHouseTradesCount = 0
//							cd.clickHouseTrades = nil
//						}
//					}
//				}
//			}
//
//		// Return, if there is any error from another function or exchange.
//		case <-ctx.Done():
//			return ctx.Err()
//		}
//	}
//}
