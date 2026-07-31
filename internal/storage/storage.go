package storage

import (
	"context"
	"time"
)

// Ticker represents final form of market ticker info received from exchange
// ready to store.
type Ticker struct {
	ExchangeName  string
	MktCommitName string
	Timestamp     time.Time
	Data          string
}

// Trade represents final form of market trade info received from exchange
// ready to store.
type Trade struct {
	ExchangeName  string
	MktCommitName string
	Timestamp     time.Time
	Data          string
}

// Level2 market Order book represents from exchange
// ready to store.
type Level2 struct {
	ExchangeName  string
	MktCommitName string
	Timestamp     time.Time
	Bids          string
	Asks          string
}

// OrdersBook market Order book represents from exchange
// ready to store.
type OrdersBook struct {
	ExchangeName  string
	MktCommitName string
	Timestamp     time.Time
	Bids          string
	Asks          string
}

// PolymarketBook represents a single RAW CLOB stream message of one Polymarket
// outcome token, ready to store (table polymarket_book). The writer does NOT
// assemble the book — it persists the stream as-is; the reader reconstructs the
// book by replaying anchors + deltas and verifying BookHash. See REAL_DATA_SCHEMA.md.
//
//   - MsgType "book":         Data = {"bids":[[price,size],...],"asks":[[...]]} FULL depth.
//   - MsgType "price_change": Data = [[price,size,side],...] changed levels (side BUY/SELL,
//     size "0" = level removed).
//
// Seq is a per-token monotonic ordering counter (survives reconnects; time-seeded so
// it stays monotonic across process restarts too). BookHash is the Polymarket book
// checksum AFTER this message.
type PolymarketBook struct {
	Subject     string // configured markets[].id (e.g. "BTC", "SPY", "WEATHER_HIGH")
	EventID     string
	ConditionID string
	TokenID     string
	Timestamp   time.Time
	Seq         uint64
	MsgType     string
	Data        string
	BookHash    string
}

// PolymarketMarket represents metadata + resolution of a single Polymarket
// outcome token, ready to store (table polymarket_market). The resolution
// fields (Resolved, WinningOutcome) are updated once the market settles;
// the row is versioned by UpdatedTs (ReplacingMergeTree). PriceLow/PriceHigh
// and WinningOutcome are nullable (nil => NULL). See REAL_DATA_SCHEMA.md.
type PolymarketMarket struct {
	Subject        string // configured markets[].id (e.g. "BTC", "SPY", "WEATHER_HIGH")
	EventID        string
	EventSlug      string
	ConditionID    string
	TokenID        string
	TokenIndex     uint8 // 0 = Yes/Up, 1 = No/Down
	Question       string
	OutcomeName    string
	MarketType     string
	PriceLow       *float64
	PriceHigh      *float64
	TickSize       float64
	MinOrderSize   float64
	CreatedTs      time.Time
	ExpiryTs       time.Time
	Resolved       uint8
	WinningOutcome *uint8
	UpdatedTs      time.Time
	// Category is the Gamma category of the event (politics/sports/crypto/...).
	// Empty for the configured (non-auto) subjects.
	Category string
}

// PolymarketScreener is one screener measurement window for one outcome token
// (table polymarket_screener). Metrics follow the canonical screener
// (mm_engine/screener_zero_curvature.py); State is CANDIDATE/RECORDING/DROPPED
// and InPassList carries the hysteresis-smoothed verdict used as a bot signal.
type PolymarketScreener struct {
	Ts          time.Time
	Subject     string
	Category    string
	EventID     string
	EventSlug   string
	ConditionID string
	TokenID     string
	ExpiryTs    time.Time
	State       string
	Mid         float64
	SpreadTicks float64
	Depth       float64
	TickSize    float64
	TwoSided    float64
	R2          float64
	ResStdT     float64
	CurvT       float64
	JumpRate    float64
	Passed      uint8
	Score       float64
	Fails       string
	InPassList  uint8
}

// Storage represents different storage options where the ticker and trade data can be stored.
type Storage interface {
	CommitTickers(context.Context, []Ticker) error
	CommitTrades(context.Context, []Trade) error
	CommitLevel2(context.Context, []Level2) error
	CommitOrdersBook(context.Context, []OrdersBook) error
	CommitPolymarketBook(context.Context, []PolymarketBook) error
	CommitPolymarketMarket(context.Context, []PolymarketMarket) error
	CommitPolymarketScreener(context.Context, []PolymarketScreener) error
}
