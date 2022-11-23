package exchange

import (
	"context"
	"fmt"
	"io"
	"net"

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

// StartBinanceFutures is for starting binance exchange functions.
func StartBinanceFutures(appCtx context.Context, markets []config.Market, retry *config.Retry, connCfg *config.Connection) error {

	// If any error occurs or connection is lost, retry the exchange functions with a time gap till it reaches
	// a configured number of retry.
	// Retry counter will be reset back to zero if the elapsed time since the last retry is greater than the configured one.
	var retryCount int
	lastRetryTime := time.Now()

	for {
		err := newBinanceFutures(appCtx, markets, connCfg)
		if err != nil {
			log.Error().Err(err).Str("exchange", "binanceFutures").Msg("error occurred")
			if retry.Number == 0 {
				return errors.New("not able to connect binance exchange. please check the log for details")
			}
			if retry.ResetSec == 0 || time.Since(lastRetryTime).Seconds() < float64(retry.ResetSec) {
				retryCount++
			} else {
				retryCount = 1
			}
			lastRetryTime = time.Now()
			if retryCount > retry.Number {
				err = fmt.Errorf("not able to connect binance exchange even after %d retry", retry.Number)
				log.Error().Err(err).Str("exchange", "binanceFutures").Msg("")
				return err
			}

			log.Error().Str("exchange", "binanceFutures").Int("retry", retryCount).Msg(fmt.Sprintf("retrying functions in %d seconds", retry.GapSec))
			tick := time.NewTicker(time.Duration(retry.GapSec) * time.Second)
			select {
			case <-tick.C:
				tick.Stop()

			// Return, if there is any error from another exchange.
			case <-appCtx.Done():
				log.Error().Str("exchange", "binanceFutures").Msg("ctx canceled, return from StartBinance")
				return appCtx.Err()
			}
		}
	}
}

type binanceFutures struct {
	ws                  connector.Websocket
	rest                *connector.REST
	connCfg             *config.Connection
	cfgMap              map[cfgLookupKey]cfgLookupVal
	channelIds          map[int][2]string
	ter                 *storage.Terminal
	clickhouse          *storage.ClickHouse
	wsTerTickers        chan []storage.Ticker
	wsTerTrades         chan []storage.Trade
	wsClickHouseTickers chan []storage.Ticker
	wsClickHouseTrades  chan []storage.Trade
}

type wsRespBestPricesBinanceFutures struct {
	BestBid     string `json:"b"`
	BestBidSize string `json:"B"`
	BestAsk     string `json:"a"`
	BestAskSize string `json:"A"`
}

type storeBestPricesBinanceFutures struct {
	bestBid     string
	bestBidSize string
	bestAsk     string
	bestAskSize string
}

type wsRespTickerBinanceFutures struct {
	TickerPrice string `json:"c"`
	TickerTime  int64  `json:"E"`

	Cdel int64  `json:"C"`
	Edel string `json:"e"`
}

type wsRespTradeBinanceFutures struct {
	Symbol     string `json:"s"`
	TradeID    uint64 `json:"a"`
	Maker      bool   `json:"m"`
	Qty        string `json:"q"`
	TradePrice string `json:"p"`
	TradeTime  int64  `json:"T"`
}

//type restRespBinance struct {
//	TradeID uint64 `json:"id"`
//	Maker   bool   `json:"isBuyerMaker"`
//	Qty     string `json:"qty"`
//	Price   string `json:"price"`
//	Time    int64  `json:"time"`
//}

func newBinanceFutures(appCtx context.Context, markets []config.Market, connCfg *config.Connection) error {

	// If any exchange function fails, force all the other functions to stop and return.
	binanceErrGroup, ctx := errgroup.WithContext(appCtx)

	b := binanceFutures{connCfg: connCfg}

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

					binanceErrGroup.Go(func() error {
						return b.closeWsConnOnError(ctx)
					})

					binanceErrGroup.Go(func() error {
						return b.readWs(ctx)
					})

					if b.ter != nil {
						binanceErrGroup.Go(func() error {
							return WsTickersToStorage(ctx, b.ter, b.wsTerTickers)
						})
						binanceErrGroup.Go(func() error {
							return WsTradesToStorage(ctx, b.ter, b.wsTerTrades)
						})
					}

					if b.clickhouse != nil {
						binanceErrGroup.Go(func() error {
							return WsTickersToStorage(ctx, b.clickhouse, b.wsClickHouseTickers)
						})
						binanceErrGroup.Go(func() error {
							return WsTradesToStorage(ctx, b.clickhouse, b.wsClickHouseTrades)
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
					log.Debug().Str("exchange", "binanceFutures").Int("count", threshold).Msg("subscribe threshold reached, waiting 2 sec")
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
				//	binanceErrGroup.Go(func() error {
				//		return b.processREST(ctx, mktID, mktCommitName, channel, restPingIntSec)
				//	})
				//
				//	restCount++
			}
		}
	}

	err = binanceErrGroup.Wait()
	if err != nil {
		return err
	}
	return nil
}

func (b *binanceFutures) cfgLookup(markets []config.Market) error {
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
					}
				case "clickhouse":
					val.clickHouseStr = true
					if b.clickhouse == nil {
						b.clickhouse = storage.GetClickHouse()
						b.wsClickHouseTickers = make(chan []storage.Ticker, 1)
						b.wsClickHouseTrades = make(chan []storage.Trade, 1)
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

func (b *binanceFutures) connectWs(ctx context.Context) error {
	ws, err := connector.NewWebsocket(ctx, &b.connCfg.WS, config.BinanceFuturesWebsocketURL)
	if err != nil {
		if !errors.Is(err, ctx.Err()) {
			logErrStack(err)
		}
		return err
	}
	b.ws = ws
	log.Info().Str("exchange", "binanceFutures").Msg("websocket connected")
	return nil
}

// closeWsConnOnError closes websocket connection if there is any error in app context.
// This will unblock all read and writes on websocket.
func (b *binanceFutures) closeWsConnOnError(ctx context.Context) error {
	<-ctx.Done()
	err := b.ws.Conn.Close()
	if err != nil {
		return err
	}
	return ctx.Err()
}

// subWsChannel sends channel subscription requests to the websocket server.
func (b *binanceFutures) subWsChannel(market string, channel string, id int) error {
	if channel == "trade" {
		channel = "aggTrade"
	}
	channel = strings.ToLower(market) + "@" + channel
	sub := wsSubBinance{
		Method: "SUBSCRIBE",
		Params: [1]string{channel},
		ID:     id,
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

	if channel == strings.ToLower(market)+"@ticker" {
		err = b.subWsChannel(market, "bookTicker", id+100)
		if err != nil {
			logErrStack(err)
			return err
		}
	}

	return nil
}

// Ping pong Ws
func (b *binanceFutures) pong() error {
	err := b.ws.Write([]byte("pong frame"))
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
func (b *binanceFutures) readWs(ctx context.Context) error {

	// To avoid data race, creating a new local lookup map.
	cfgLookup := make(map[cfgLookupKey]cfgLookupVal, len(b.cfgMap))
	for k, v := range b.cfgMap {
		cfgLookup[k] = v
	}

	cd := commitData{
		terTickers:        make([]storage.Ticker, 0, b.connCfg.Terminal.TickerCommitBuf),
		terTrades:         make([]storage.Trade, 0, b.connCfg.Terminal.TradeCommitBuf),
		clickHouseTickers: make([]storage.Ticker, 0, b.connCfg.ClickHouse.TickerCommitBuf),
		clickHouseTrades:  make([]storage.Trade, 0, b.connCfg.ClickHouse.TradeCommitBuf),
	}

	storeTick := make(map[string]wsTickerData)
	storeBestPrices := make(map[string]storeBestPricesBinanceFutures)

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

			if string(frame[:4]) == "ping" {
				log.Info().Str("exchange", "binanceFutures").Str("func", "ping").Msg(string(frame))
				err = b.pong()
				if err != nil {
					return err
				}
				continue
			}

			wr := wsRespBinance{}
			err = jsoniter.Unmarshal(frame, &wr)
			if err != nil {
				log.Debug().Str("exchange", "binanceFutures").Str("func", "readWs").Msg(string(frame))
				logErrStack(err)
				return err
			}

			if wr.ID != 0 {
				log.Debug().Str("exchange", "binanceFutures").Str("func", "readWs").Str("market", b.channelIds[wr.ID][0]).Str("channel", b.channelIds[wr.ID][1]).Msg("channel subscribed")
				continue
			}
			if wr.Msg != "" {
				log.Error().Str("exchange", "binanceFutures").Str("func", "readWs").Int("code", wr.Code).Str("msg", wr.Msg).Msg("")
				return errors.New("binanceFutures websocket error")
			}

			if wr.Event == "bookTicker" {
				wrbp := wsRespBestPricesBinanceFutures{}
				err = jsoniter.Unmarshal(frame, &wrbp)
				if err != nil {
					log.Debug().Str("exchange", "binanceFutures").Str("func", "readWs").Msg(string(frame))
					logErrStack(err)
					return err
				}
				storeBestPrices[wr.Symbol] = storeBestPricesBinanceFutures{
					bestAsk:     wrbp.BestAsk,
					bestAskSize: wrbp.BestAskSize,
					bestBid:     wrbp.BestBid,
					bestBidSize: wrbp.BestBidSize,
				}
				continue
			}

			switch wr.Event {
			case "24hrTicker":
				wr.Event = "ticker"
			case "aggTrade":
				wr.Event = "trade"
			}

			// Consider frame only in configured interval, otherwise ignore it.
			if wr.Event == "ticker" || wr.Event == "trade" {

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
					wrti := wsRespTickerBinanceFutures{}
					err = jsoniter.Unmarshal(frame, &wrti)
					if err != nil {
						log.Debug().Str("exchange", "binanceFutures").Str("func", "readWs").Msg(string(frame))
						logErrStack(err)
						return err
					}

					st, oks := storeBestPrices[wr.Symbol]
					if oks {
						sTick, ok := storeTick[wr.mktCommitName]
						if ok && sTick.price == wrti.TickerPrice && sTick.bestAsk == st.bestAsk && sTick.bestAskSize == st.bestAskSize && sTick.bestBid == st.bestBid && sTick.bestBidSize == st.bestBidSize {
							continue
						}

						storeTick[wr.mktCommitName] = wsTickerData{
							price:       wrti.TickerPrice,
							bestAsk:     st.bestAsk,
							bestAskSize: st.bestAskSize,
							bestBid:     st.bestBid,
							bestBidSize: st.bestBidSize,
						}

						wr.data, err = jsoniter.MarshalToString(storeBinanceTicker{
							Price:       wrti.TickerPrice,
							Time:        wrti.TickerTime,
							BestAsk:     st.bestAsk,
							BestAskSize: st.bestAskSize,
							BestBid:     st.bestBid,
							BestBidSize: st.bestBidSize,
						})
					} else {
						continue
					}
				case "trade":
					wrt := wsRespTradeBinanceFutures{}
					err = jsoniter.Unmarshal(frame, &wrt)
					if err != nil {
						log.Debug().Str("exchange", "binanceFutures").Str("func", "readWs").Msg(string(frame))
						logErrStack(err)
						return err
					}
					side := "buy"
					if wrt.Maker {
						side = "sell"
					}
					wr.data, err = jsoniter.MarshalToString(storeBinanceTrade{
						Symbol:  wrt.Symbol,
						Time:    wrt.TradeTime,
						TradeId: wrt.TradeID,
						Side:    side,
						Size:    wrt.Qty,
						Price:   wrt.TradePrice,
					})
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
func (b *binanceFutures) processWs(ctx context.Context, wr *wsRespBinance, cd *commitData) error {
	switch wr.Event {
	case "ticker":
		ticker := storage.Ticker{}
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
	}
	return nil
}

//func (b *binanceFutures) connectRest() error {
//	rest, err := connector.GetREST()
//	if err != nil {
//		logErrStack(err)
//		return err
//	}
//	b.rest = rest
//	log.Info().Str("exchange", "binanceFutures").Msg("REST connection setup is done")
//	return nil
//}

// processREST queries exchange for ticker / trade data through REST API in configured intervals,
// transforms it to a common ticker / trade store format,
// buffers the same in memory and
// then sends it to different storage systems for commit through go channels.
//func (b *binanceFutures) processREST(ctx context.Context, mktID string, mktCommitName string, channel string, interval int) error {
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
//		req, err = b.rest.Request(ctx, "GET", config.BinanceRESTBaseURL+"ticker/price")
//		if err != nil {
//			if !errors.Is(err, ctx.Err()) {
//				logErrStack(err)
//			}
//			return err
//		}
//		q = req.URL.Query()
//		q.Add("symbol", mktID)
//	case "trade":
//		req, err = b.rest.Request(ctx, "GET", config.BinanceRESTBaseURL+"trades")
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
//				rr := restRespBinance{}
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
//					Exchange:      "binanceFutures",
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
//				rr := []restRespBinance{}
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
//						Exchange:      "binanceFutures",
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
