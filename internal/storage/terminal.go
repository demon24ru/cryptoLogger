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
