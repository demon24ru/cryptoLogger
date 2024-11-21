package exchange

import (
	"context"
	"time"

	"github.com/milkywaybrain/cryptogalaxy/internal/storage"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// cfgLookupKey is a key in the config lookup map.
type cfgLookupKey struct {
	market  string
	channel string
}

// cfgLookupVal is a value in the config lookup map.
type cfgLookupVal struct {
	connector        string
	wsConsiderIntSec int
	wsLastUpdated    time.Time
	terStr           bool
	mysqlStr         bool
	esStr            bool
	influxStr        bool
	natsStr          bool
	clickHouseStr    bool
	s3Str            bool
	id               int
	mktCommitName    string
}

type commitData struct {
	terTickersCount           int
	terTradesCount            int
	terLevel2Count            int
	terOrdersBookCount        int
	clickHouseTickersCount    int
	clickHouseTradesCount     int
	clickHouseLevel2Count     int
	clickHouseOrdersBookCount int
	terTickers                []storage.Ticker
	terTrades                 []storage.Trade
	terLevel2                 []storage.Level2
	terOrdersBooks            []storage.OrdersBook
	clickHouseTickers         []storage.Ticker
	clickHouseTrades          []storage.Trade
	clickHouseLevel2          []storage.Level2
	clickHouseOrdersBook      []storage.OrdersBook
}

type storeTickerData struct {
	bestAsk     string
	bestAskSize string
	bestBid     string
	bestBidSize string
}

type commitTicker struct {
	BestAsk     string `json:"ba"`
	BestAskSize string `json:"bas"`
	BestBid     string `json:"bb"`
	BestBidSize string `json:"bbs"`
}

type commitTrade struct {
	Side  string `json:"t"`
	Size  string `json:"s"`
	Price string `json:"p"`
}

type cacheOB struct {
	bids map[string]string
	asks map[string]string
}

// WsTickersToStorage batch inserts input ticker data from websocket to specified storage.
func WsTickersToStorage(ctx context.Context, str storage.Storage, tickers <-chan []storage.Ticker) error {
	for {
		select {
		case data := <-tickers:
			err := str.CommitTickers(ctx, data)
			if err != nil {
				if !errors.Is(err, ctx.Err()) {
					logErrStack(err)
				}
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// WsTradesToStorage batch inserts input trade data from websocket to specified storage.
func WsTradesToStorage(ctx context.Context, str storage.Storage, trades <-chan []storage.Trade) error {
	for {
		select {
		case data := <-trades:
			err := str.CommitTrades(ctx, data)
			if err != nil {
				if !errors.Is(err, ctx.Err()) {
					logErrStack(err)
				}
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// WsLevel2ToStorage batch inserts input trade data from websocket to specified storage.
func WsLevel2ToStorage(ctx context.Context, str storage.Storage, level2 <-chan []storage.Level2) error {
	for {
		select {
		case data := <-level2:
			err := str.CommitLevel2(ctx, data)
			if err != nil {
				if !errors.Is(err, ctx.Err()) {
					logErrStack(err)
				}
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// WsOrdersBookToStorage batch inserts input orders book data from websocket to specified storage.
func WsOrdersBookToStorage(ctx context.Context, str storage.Storage, ordersBook <-chan []storage.OrdersBook) error {
	for {
		select {
		case data := <-ordersBook:
			err := str.CommitOrdersBook(ctx, data)
			if err != nil {
				if !errors.Is(err, ctx.Err()) {
					logErrStack(err)
				}
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// logErrStack logs error with stack trace.
func logErrStack(err error) {
	log.Error().Stack().Err(errors.WithStack(err)).Msg("")
}
