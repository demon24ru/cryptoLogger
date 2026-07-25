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
		// ABOVE ladder: bare-number title with an "above" question.
		{"above-daily", "54,000", "Will the price of Bitcoin be above $54,000 on July 23?", "ABOVE", f(54000), nil},
		{"above-hourly", "68,400", "Bitcoin above 68,400 on July 22, 2PM ET?", "ABOVE", f(68400), nil},

		// RANGE ladder: middle buckets ("A-B") and the two open-ended tails.
		{"between", "56,000-58,000", "Will the price of Bitcoin be between $56,000 and $58,000 on July 23?", "RANGE", f(56000), f(58000)},
		{"between-endash", "66,000–68,000", "Will the price of Bitcoin be between $66,000 and $68,000 on July 23?", "RANGE", f(66000), f(68000)},
		{"range-bottom-tail", "<56,000", "Will the price of Bitcoin be less than $56,000 on July 23?", "RANGE", nil, f(56000)},
		// ">X" / "greater than" is the OPEN-ENDED TOP of a range ladder, NOT ABOVE.
		{"range-top-tail", ">74,000", "Will the price of Bitcoin be greater than $74,000 on July 30?", "RANGE", f(74000), nil},

		// TOUCH: "↑ X" = reach up to X (high=barrier), "↓ X" = dip down to X (low=barrier).
		{"touch-up", "↑ 100,000", "Will Bitcoin reach $100,000 in July?", "TOUCH", nil, f(100000)},
		{"touch-down", "↓ 60,000", "Will Bitcoin dip to $60,000 in July?", "TOUCH", f(60000), nil},
		{"touch-up-dollar", "↑ $130", "Will WTI Crude Oil (WTI) hit (HIGH) $130 in July?", "TOUCH", nil, f(130)},

		// Finance ABOVE: "closes above $X" with a $-prefixed bare-number title.
		{"finance-above", "$95", "WTI Crude Oil (WTI) closes above $95 on July 27?", "ABOVE", f(95), nil},

		// WEATHER BUCKET: exact "N°C", and the "or below"/"or higher" tails.
		{"weather-exact", "34°C", "Will the highest temperature in Chengdu be 34°C on July 26?", "BUCKET", f(34), f(34)},
		{"weather-below", "33°C or below", "Will the highest temperature in Chengdu be 33°C or below on July 26?", "BUCKET", nil, f(33)},
		{"weather-higher", "43°C or higher", "Will the highest temperature in Chengdu be 43°C or higher on July 26?", "BUCKET", f(43), nil},

		// Out of scope -> skipped (coin is filtered separately at the event level).
		{"date-target", "by September 30, 2025", "Will Bitcoin hit $150k by September 30?", "", nil, nil},
		{"volindex-touch", "↓ 40", "Will the Bitcoin Volatility Index dip to 40 by July 31?", "", nil, nil},
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

// TestEventHasAllTags locks in the per-subject require-tag AND filter.
func TestEventHasAllTags(t *testing.T) {
	ev := &gammaEvent{Tags: []gammaTag{{ID: "104466"}, {ID: "309"}, {ID: "120"}}}
	if !eventHasAllTags(ev, []int{104466, 309}) {
		t.Errorf("expected all tags present")
	}
	if eventHasAllTags(ev, []int{104466, 104166}) {
		t.Errorf("expected missing tag 104166 to fail")
	}
	if !eventHasAllTags(ev, nil) {
		t.Errorf("empty require set should always pass")
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
