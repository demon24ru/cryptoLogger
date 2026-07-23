package exchange

import (
	"strconv"
	"testing"
)

// TestParseMarket locks in how Polymarket BTC markets (discovered via the
// Crypto-Prices base tag, which also carries other coins and product types) map
// to (market_type, price_low, price_high). Classification is by question phrasing;
// price levels come from groupItemTitle. Real strings observed on the live API.
func TestParseMarket(t *testing.T) {
	f := func(v float64) *float64 { return &v }

	cases := []struct {
		name     string
		git      string
		question string
		wantType string // "" means ok=false (skip)
		wantLow  *float64
		wantHigh *float64
	}{
		// ABOVE (daily "$" form, hourly no-"$" form, and "greater than").
		{"above-daily", "54,000", "Will the price of Bitcoin be above $54,000 on July 23?", "ABOVE", f(54000), nil},
		{"above-hourly", "68,400", "Bitcoin above 68,400 on July 22, 2PM ET?", "ABOVE", f(68400), nil},
		{"greater-than", ">74,000", "Will the price of Bitcoin be greater than $74,000 on July 23?", "ABOVE", f(74000), nil},

		// RANGE (between) and range-ladder bottom tail (less than).
		{"between", "56,000-58,000", "Will the price of Bitcoin be between $56,000 and $58,000 on July 23?", "RANGE", f(56000), f(58000)},
		{"less-than", "<56,000", "Will the price of Bitcoin be less than $56,000 on July 23?", "RANGE", nil, f(56000)},

		// Out of scope -> skipped (coin is filtered separately at the event level).
		{"touch-hit", "by September 30, 2025", "Will Bitcoin hit $150k by September 30?", "", nil, nil},
		{"touch-dip-volindex", "↓ 40", "Will the Bitcoin Volatility Index dip to 40 by July 31?", "", nil, nil},
		{"updown", "", "Bitcoin Up or Down - July 23, 1:00PM-1:05PM ET", "", nil, nil},
		{"comparison", "", "Will Bitcoin outperform Gold in 2026?", "", nil, nil},
	}

	for _, c := range cases {
		low, high, mtype, ok := parseMarket(c.git, c.question)
		wantOK := c.wantType != ""
		if ok != wantOK {
			t.Errorf("%s: ok=%v want %v", c.name, ok, wantOK)
			continue
		}
		if !ok {
			continue
		}
		if mtype != c.wantType {
			t.Errorf("%s: type=%s want %s", c.name, mtype, c.wantType)
		}
		if !ptrEq(low, c.wantLow) {
			t.Errorf("%s: low=%s want %s", c.name, ptrStr(low), ptrStr(c.wantLow))
		}
		if !ptrEq(high, c.wantHigh) {
			t.Errorf("%s: high=%s want %s", c.name, ptrStr(high), ptrStr(c.wantHigh))
		}
	}
}

// TestEventCoin locks in coin derivation from Gamma event tags (with title
// fallback) — mirroring the sibling project's polymarket_adapter._extract_coin.
func TestEventCoin(t *testing.T) {
	ev := func(title string, tags ...string) *gammaEvent {
		e := &gammaEvent{Title: title}
		for _, l := range tags {
			e.Tags = append(e.Tags, gammaTag{Label: l})
		}
		return e
	}
	cases := []struct {
		name string
		ev   *gammaEvent
		want string
	}{
		{"btc-tag", ev("Bitcoin above $60,000 on July 23?", "Bitcoin", "Crypto", "Crypto Prices"), "BTC"},
		{"sol-tag", ev("Solana above ...", "Solana", "Crypto"), "SOL"},
		{"eth-tag", ev("", "Ethereum"), "ETH"},
		{"ripple-alias", ev("", "Crypto", "Ripple"), "XRP"},
		{"btc-title-fallback", ev("Will Bitcoin outperform Gold in 2026?", "Crypto Prices", "Price Comparison"), "BTC"},
		{"no-coin", ev("Will USDT market cap hit $200B?", "Stablecoins", "Crypto", "Crypto Prices"), ""},
	}
	for _, c := range cases {
		if got := eventCoin(c.ev); got != c.want {
			t.Errorf("%s: eventCoin=%q want %q", c.name, got, c.want)
		}
	}
}

func ptrEq(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func ptrStr(p *float64) string {
	if p == nil {
		return "nil"
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}
