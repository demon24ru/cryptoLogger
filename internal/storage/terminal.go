package storage

import (
	"context"
	"fmt"
	"io"
)

// Terminal is for displaying data on terminal.
type Terminal struct {
	out io.Writer
}

var terminal Terminal

// TerminalTimestamp is used as a format to display only the time.
const TerminalTimestamp = "15:04:05.999"
const TerminalTimestampMicroSec = "15:04:05.999999999"

// InitTerminal initializes terminal display.
// Output writer is always os.Stdout except in case of testing where file will be set as output terminal.
func InitTerminal(out io.Writer) *Terminal {
	if terminal.out == nil {
		terminal = Terminal{
			out: out,
		}
	}
	return &terminal
}

// GetTerminal returns already prepared terminal instance.
func GetTerminal() *Terminal {
	return &terminal
}

// CommitTickers batch outputs input ticker data to terminal.
func (t *Terminal) CommitTickers(_ context.Context, data []Ticker) error {
	for i := range data {
		ticker := data[i]
		fmt.Fprintf(t.out, "%-15s%-15s%-15s%-15s%20s\n\n", "Ticker", ticker.ExchangeName, ticker.MktCommitName, ticker.Data, ticker.Timestamp.Local().Format(TerminalTimestamp))
	}
	return nil
}

// CommitTrades batch outputs input trade data to terminal.
func (t *Terminal) CommitTrades(_ context.Context, data []Trade) error {
	for i := range data {
		trade := data[i]
		fmt.Fprintf(t.out, "%-15s%-15s%-15s%-15s%20s\n\n", "Trade", trade.ExchangeName, trade.MktCommitName, trade.Data, trade.Timestamp.Local().Format(TerminalTimestamp))
	}
	return nil
}

func (t *Terminal) CommitLevel2(_ context.Context, data []Level2) error {
	for i := range data {
		level2 := data[i]
		fmt.Fprintf(t.out, "%-15s%-15s%-15s%-15s%-15s%20s\n\n", "Level2", level2.ExchangeName, level2.MktCommitName, level2.Bids, level2.Asks, level2.Timestamp.Local().Format(TerminalTimestampMicroSec))
	}
	return nil
}

func (t *Terminal) CommitOrdersBook(_ context.Context, data []OrdersBook) error {
	for i := range data {
		ordersBook := data[i]
		fmt.Fprintf(t.out, "%-15s%-15s%-15s%-15s%-15s%20s\n\n", "OrdersBook", ordersBook.ExchangeName, ordersBook.MktCommitName, ordersBook.Bids, ordersBook.Asks, ordersBook.Timestamp.Local().Format(TerminalTimestampMicroSec))
	}
	return nil
}

// CommitPolymarketBook batch outputs input Polymarket raw book stream rows to terminal.
func (t *Terminal) CommitPolymarketBook(_ context.Context, data []PolymarketBook) error {
	for i := range data {
		book := data[i]
		fmt.Fprintf(t.out, "%-12s%-8s%-14s%-25s seq=%-12d %-40s %20s\n\n", "PolyBook", book.Subject, book.MsgType, book.TokenID, book.Seq, book.Data, book.Timestamp.Local().Format(TerminalTimestampMicroSec))
	}
	return nil
}

// CommitPolymarketMarket batch outputs input Polymarket market metadata to terminal.
func (t *Terminal) CommitPolymarketMarket(_ context.Context, data []PolymarketMarket) error {
	for i := range data {
		mkt := data[i]
		low := "null"
		if mkt.PriceLow != nil {
			low = fmt.Sprintf("%g", *mkt.PriceLow)
		}
		high := "null"
		if mkt.PriceHigh != nil {
			high = fmt.Sprintf("%g", *mkt.PriceHigh)
		}
		win := "null"
		if mkt.WinningOutcome != nil {
			win = fmt.Sprintf("%d", *mkt.WinningOutcome)
		}
		fmt.Fprintf(t.out, "%-12s%-8s%-8s%-25s %-10s%-10s%-8s%-28s resolved=%d win=%s  %s\n\n", "PolyMarket", mkt.Subject, mkt.MarketType, mkt.TokenID, low, high, mkt.OutcomeName, mkt.EventSlug, mkt.Resolved, win, mkt.ExpiryTs.Local().Format(TerminalTimestamp))
	}
	return nil
}
