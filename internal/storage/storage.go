package storage

import (
	"context"
	"time"
)

// Ticker represents final form of market ticker info received from exchange
// ready to store.
type Ticker struct {
	MktCommitName string
	Timestamp     time.Time
	Data          string
}

// Trade represents final form of market trade info received from exchange
// ready to store.
type Trade struct {
	MktCommitName string
	Timestamp     time.Time
	Data          string
}

// Level2 market Order book represents from exchange
// ready to store.
type Level2 struct {
	MktCommitName string
	Timestamp     time.Time
	Data          string
}

// OrdersBook market Order book represents from exchange
// ready to store.
type OrdersBook struct {
	MktCommitName string
	Timestamp     time.Time
	Bids          string
	Asks		  string
}

// Storage represents different storage options where the ticker and trade data can be stored.
type Storage interface {
	CommitTickers(context.Context, []Ticker) error
	CommitTrades(context.Context, []Trade) error
	CommitLevel2(context.Context, []Level2) error
	CommitOrdersBook(context.Context, []OrdersBook) error
}
