package exchange

import (
	"testing"
	"time"

	"github.com/milkywaybrain/cryptogalaxy/internal/config"
	"github.com/milkywaybrain/cryptogalaxy/internal/screener"
)

// newTestAuto builds an auto manager on defaults with a connector that owns no
// tokens, which is all the state-machine tests need.
func newTestAuto(t *testing.T, cfg config.PolymarketAuto) *polyAuto {
	t.Helper()
	a := newPolyAuto(cfg)
	a.attach(&polymarket{
		subjectCfg: map[string]*polySubjectCfg{},
		meta:       map[string]tokenMeta{},
		subscribed: map[string]struct{}{},
		known:      map[string]*knownMarket{},
	})
	return a
}

// TestPolyAutoDefaults locks in that an empty config block yields the documented
// defaults, and that gate overrides land on the canon values field by field.
func TestPolyAutoDefaults(t *testing.T) {
	a := newTestAuto(t, config.PolymarketAuto{})
	if a.pollIntSec != polyAutoDefaultPollIntSec || a.windowSec != polyAutoDefaultWindowSec {
		t.Errorf("poll/window defaults = %d/%d", a.pollIntSec, a.windowSec)
	}
	if a.maxRecording != polyAutoDefaultMaxRecording || a.hystK != polyAutoDefaultHysteresisK || a.hystM != polyAutoDefaultHysteresisM {
		t.Errorf("budget/hysteresis defaults = %d/%d/%d", a.maxRecording, a.hystK, a.hystM)
	}
	if a.gates != screener.DefaultGates() {
		t.Errorf("gates default = %+v, want canon %+v", a.gates, screener.DefaultGates())
	}

	// A single override must move only that threshold.
	b := newTestAuto(t, config.PolymarketAuto{Gates: config.PolymarketGates{MinTwoSided: 0.85}})
	want := screener.DefaultGates()
	want.MinTwoSided = 0.85
	if b.gates != want {
		t.Errorf("gates override = %+v, want %+v", b.gates, want)
	}
}

// TestCoarseKeepMarket covers the per-market half of the coarse filter: only
// quotable markets, on a fine enough grid, with room for a full observation
// window before expiry.
func TestCoarseKeepMarket(t *testing.T) {
	a := newTestAuto(t, config.PolymarketAuto{})
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	iso := func(d time.Duration) string { return now.Add(d).Format(time.RFC3339) }

	live := func() *gammaMarket {
		return &gammaMarket{
			Active: true, Closed: false, EnableOrderBook: true, AcceptingOrders: true,
			OrderPriceMinTickSize: 0.001,
			EndDate:               iso(48 * time.Hour),
		}
	}

	cases := []struct {
		name string
		mut  func(*gammaMarket)
		want bool
	}{
		{"live market", func(*gammaMarket) {}, true},
		{"not accepting orders", func(m *gammaMarket) { m.AcceptingOrders = false }, false},
		{"order book disabled", func(m *gammaMarket) { m.EnableOrderBook = false }, false},
		{"closed", func(m *gammaMarket) { m.Closed = true }, false},
		{"inactive", func(m *gammaMarket) { m.Active = false }, false},
		{"tick missing", func(m *gammaMarket) { m.OrderPriceMinTickSize = 0 }, false},
		{"tick too coarse", func(m *gammaMarket) { m.OrderPriceMinTickSize = 0.05 }, false},
		{"no end date", func(m *gammaMarket) { m.EndDate = "" }, false},
		{"expires too soon", func(m *gammaMarket) { m.EndDate = iso(2 * time.Hour) }, false},
		{"expires too far out", func(m *gammaMarket) { m.EndDate = iso(40 * 24 * time.Hour) }, false},
		// Past the minimum hours but not long enough to fit an observation window.
		{"window does not fit", func(m *gammaMarket) {
			a.minHoursToExpiry = 0.25
			m.EndDate = iso(30 * time.Minute)
		}, false},
	}
	for _, c := range cases {
		a.minHoursToExpiry = polyAutoDefaultMinHoursToExpiry
		m := live()
		c.mut(m)
		if got := a.coarseKeepMarket(m, now); got != c.want {
			t.Errorf("%s: coarseKeepMarket = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestConsiderEventCoarseFilter checks the event-level gates and that a market
// with no recognisable price form is still watched, as GENERIC.
func TestConsiderEventCoarseFilter(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	end := now.Add(48 * time.Hour).Format(time.RFC3339)

	ev := func(liq, vol float64) *gammaEvent {
		return &gammaEvent{
			ID: "12345", Slug: "will-x-happen",
			Tags:       []gammaTag{{ID: "2", Label: "Presidential Election"}, {ID: "1", Label: "Politics"}},
			Liquidity:  flexFloat(liq),
			Volume24hr: flexFloat(vol),
			Markets: []gammaMarket{{
				ConditionID: "0xabc", Question: "Will X win the election?",
				ClobTokenIDs: `["yes-token","no-token"]`,
				Outcomes:     `["Yes","No"]`,
				Active:       true, EnableOrderBook: true, AcceptingOrders: true,
				OrderPriceMinTickSize: 0.001, OrderMinSize: 5, EndDate: end,
			}},
		}
	}

	t.Run("illiquid event is skipped", func(t *testing.T) {
		a := newTestAuto(t, config.PolymarketAuto{})
		if n := a.considerEvent(a.conn(), ev(100, 0), now); n != 0 || len(a.cands) != 0 {
			t.Errorf("added %d candidates from an illiquid event", n)
		}
	})

	t.Run("volume floor is applied when set", func(t *testing.T) {
		a := newTestAuto(t, config.PolymarketAuto{MinVolume24hr: 10000})
		if n := a.considerEvent(a.conn(), ev(50000, 500), now); n != 0 {
			t.Errorf("added %d candidates below the volume floor", n)
		}
	})

	t.Run("liquid event becomes a GENERIC candidate", func(t *testing.T) {
		a := newTestAuto(t, config.PolymarketAuto{})
		if n := a.considerEvent(a.conn(), ev(50000, 20000), now); n != 1 {
			t.Fatalf("added %d candidates, want 1", n)
		}
		c := a.cands["yes-token"]
		if c == nil {
			t.Fatal("candidate not keyed by its Yes token")
		}
		// A non-price question has no bounds to parse: GENERIC, low/high nil.
		if c.marketType != "GENERIC" || c.priceLow != nil || c.priceHigh != nil {
			t.Errorf("type=%s low=%v high=%v, want GENERIC/nil/nil", c.marketType, c.priceLow, c.priceHigh)
		}
		if c.noTokenID != "no-token" || c.state != polyAutoStateCandidate {
			t.Errorf("noToken=%q state=%q", c.noTokenID, c.state)
		}
		// Category prefers a known top-level label over the first tag.
		if c.category != "Politics" {
			t.Errorf("category = %q, want Politics", c.category)
		}
	})

	t.Run("a configured subject's token is never taken", func(t *testing.T) {
		a := newTestAuto(t, config.PolymarketAuto{})
		a.conn().meta["yes-token"] = tokenMeta{subject: "BTC"}
		if n := a.considerEvent(a.conn(), ev(50000, 20000), now); n != 0 || len(a.cands) != 0 {
			t.Errorf("auto mode claimed a token owned by subject BTC")
		}
	})

	t.Run("watch list is capped", func(t *testing.T) {
		a := newTestAuto(t, config.PolymarketAuto{MaxCandidates: 1})
		a.cands["other"] = &autoCandidate{state: polyAutoStateCandidate}
		if n := a.considerEvent(a.conn(), ev(50000, 20000), now); n != 0 {
			t.Errorf("added %d candidates past the cap", n)
		}
	})
}

// TestAutoClassify guards the auto mode against parseMarket's numbers-as-levels
// reading, which is safe for a tag-narrowed price feed but not for the whole
// universe: a scoreline or a tweet count must never become a price range.
func TestAutoClassify(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	cases := []struct {
		name     string
		git      string
		question string
		wantType string
		wantLow  *float64
		wantHigh *float64
	}{
		// Real price/level markets keep their classification and bounds.
		{"price ladder", "60,000", "Will the price of Bitcoin be above $60,000 on July 23?", "ABOVE", f(60000), nil},
		{"price range", "56,000-58,000", "Will the price of Bitcoin be between $56,000 and $58,000 on July 23?", "RANGE", f(56000), f(58000)},
		{"touch up", "↑ 100,000", "Will Bitcoin reach $100,000 in July?", "TOUCH", nil, f(100000)},
		{"commodity touch", "↑ $130", "Will WTI Crude Oil (WTI) hit (HIGH) $130 in July?", "TOUCH", nil, f(130)},
		{"weather bucket", "34°C", "Will the highest temperature in Chengdu be 34°C on July 26?", "BUCKET", f(34), f(34)},

		// Numbers WITHOUT a unit are not levels. These are the live-feed cases
		// that were being written as bogus price ranges.
		{"football scoreline", "3 - 2", "Exact Score: Club Puebla 3 - 2 CD Guadalajara?", "GENERIC", nil, nil},
		{"tweet count range", "140-159", "Will Elon Musk post 140-159 tweets from July 28 to August 4, 2026?", "GENERIC", nil, nil},
		{"plain question", "", "Will X win the election?", "GENERIC", nil, nil},
		{"seat count", "50-59", "How many seats will the party win?", "GENERIC", nil, nil},
	}
	for _, c := range cases {
		low, high, mtype := autoClassify(&gammaMarket{GroupItemTitle: c.git, Question: c.question})
		if mtype != c.wantType {
			t.Errorf("%s: type = %s, want %s", c.name, mtype, c.wantType)
			continue
		}
		if !eqFloatPtr(low, c.wantLow) || !eqFloatPtr(high, c.wantHigh) {
			t.Errorf("%s: bounds = %v/%v, want %v/%v", c.name, deref(low), deref(high), deref(c.wantLow), deref(c.wantHigh))
		}
	}
}

func eqFloatPtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func deref(p *float64) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

// TestTopOfBook checks the touch is taken by VALUE (best bid = highest, best ask
// = lowest) rather than by trusting the CLOB's level ordering, and that
// zero-size levels are treated as absent — both required for canon parity.
func TestTopOfBook(t *testing.T) {
	// Deliberately shuffled: /books returns bids ascending and asks descending.
	e := &polyBooksEntry{
		AssetID:  "tok",
		TickSize: 0.001,
		Bids:     []polyBookLevel{{"0.40", "10"}, {"0.44", "25"}, {"0.42", "5"}},
		Asks:     []polyBookLevel{{"0.50", "8"}, {"0.46", "30"}, {"0.48", "12"}},
	}
	b := topOfBook(e)
	if !b.hasBid || b.bid != 0.44 || b.bidSize != 25 {
		t.Errorf("best bid = %v/%v, want 0.44/25", b.bid, b.bidSize)
	}
	if !b.hasAsk || b.ask != 0.46 || b.askSize != 30 {
		t.Errorf("best ask = %v/%v, want 0.46/30", b.ask, b.askSize)
	}
	if b.tickSize != 0.001 {
		t.Errorf("tick size = %v", b.tickSize)
	}

	// size "0" is a removed level, not a zero-size best price.
	z := topOfBook(&polyBooksEntry{AssetID: "tok", Bids: []polyBookLevel{{"0.44", "0"}, {"0.40", "3"}}})
	if z.bid != 0.40 || z.bidSize != 3 {
		t.Errorf("zero-size level was taken as the touch: %v/%v", z.bid, z.bidSize)
	}

	// An empty book must read as an absent side, not as a 0/0 two-sided book.
	empty := topOfBook(&polyBooksEntry{AssetID: "tok"})
	if empty.hasBid || empty.hasAsk {
		t.Error("empty book reported a side")
	}
}

// TestInFinalPhase covers §118: the final slice of a dated market's lifetime is
// forced out of the pass list. Without a start date the rule falls back to the
// last day before expiry.
func TestInFinalPhase(t *testing.T) {
	a := newTestAuto(t, config.PolymarketAuto{})
	expiry := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)
	created := expiry.Add(-7 * 24 * time.Hour) // a weekly: 10% ~ the last 16.8h

	weekly := &autoCandidate{createdTs: created, expiryTs: expiry}
	if a.inFinalPhase(weekly, expiry.Add(-48*time.Hour)) {
		t.Error("two days out must not be the final phase")
	}
	if !a.inFinalPhase(weekly, expiry.Add(-6*time.Hour)) {
		t.Error("six hours out must be the final phase of a weekly")
	}

	noStart := &autoCandidate{expiryTs: expiry}
	if a.inFinalPhase(noStart, expiry.Add(-30*time.Hour)) {
		t.Error("without a start date, 30h out must not be the final phase")
	}
	if !a.inFinalPhase(noStart, expiry.Add(-2*time.Hour)) {
		t.Error("without a start date, the last day must be the final phase")
	}

	if a.inFinalPhase(&autoCandidate{}, expiry) {
		t.Error("a market with no expiry has no final phase")
	}
}

// TestScoreWindowHysteresis drives the state machine through a full cycle on
// synthetic books: warm-up, K consecutive passes to enter the pass list, a
// failing window to leave it, and the §118 forced fail.
func TestScoreWindowHysteresis(t *testing.T) {
	cfg := config.PolymarketAuto{PollIntSec: 300, ObservationWindowSec: 7200, HysteresisK: 2, HysteresisM: 1}
	a := newTestAuto(t, cfg)

	start := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	c := &autoCandidate{
		yesTokenID: "tok", noTokenID: "tok-no", conditionID: "0xabc",
		tickSize: 0.001, state: polyAutoStateCandidate,
		expiryTs: start.Add(30 * 24 * time.Hour), createdTs: start.Add(-30 * 24 * time.Hour),
	}
	a.cands["tok"] = c

	// A "pinned calm" book: 4-tick spread, deep, and it never moves — the
	// archetype the screener exists to find.
	pass := map[string]*polyTopOfBook{"tok": {
		bid: 0.498, ask: 0.502, bidSize: 500, askSize: 500, hasBid: true, hasAsk: true, tickSize: 0.001,
	}}

	// Warm-up: until the buffer spans an observation window nothing is judged.
	poll := time.Duration(cfg.PollIntSec) * time.Second
	now := start
	for i := 0; i < 24; i++ {
		rows := a.scoreWindow([]string{"tok"}, pass, now)
		if len(rows) != 1 {
			t.Fatalf("window %d produced %d rows", i, len(rows))
		}
		if c.inPassList {
			t.Fatalf("window %d entered the pass list during warm-up", i)
		}
		now = now.Add(poll)
	}

	// First judged window: passes, but K=2 means not yet in the pass list.
	a.scoreWindow([]string{"tok"}, pass, now)
	if c.passStreak != 1 || c.inPassList {
		t.Fatalf("after 1 passing window: streak=%d inPassList=%v, want 1/false", c.passStreak, c.inPassList)
	}
	now = now.Add(poll)

	// Second consecutive pass: hysteresis satisfied.
	rows := a.scoreWindow([]string{"tok"}, pass, now)
	if c.passStreak != 2 || !c.inPassList {
		t.Fatalf("after 2 passing windows: streak=%d inPassList=%v, want 2/true", c.passStreak, c.inPassList)
	}
	if rows[0].Passed != 1 || rows[0].InPassList != 1 || rows[0].Score <= 0 {
		t.Fatalf("passing row = %+v", rows[0])
	}
	if rows[0].State != polyAutoStateCandidate || rows[0].TokenID != "tok" {
		t.Fatalf("row state/token = %s/%s", rows[0].State, rows[0].TokenID)
	}
	now = now.Add(poll)

	// It is now eligible for promotion.
	if got := a.selectForPromotion(); len(got) != 1 || got[0] != c {
		t.Fatalf("selectForPromotion returned %d candidates, want the passing one", len(got))
	}

	// ANTI-FLAP: one missed observation is 1 grid point out of 24, so two_sided
	// stays above the gate and the pass list does not wobble. This is the
	// property that keeps the promoted set stable across a transient CLOB blip.
	dead := map[string]*polyTopOfBook{}
	rows = a.scoreWindow([]string{"tok"}, dead, now)
	if rows[0].Passed != 1 || !c.inPassList {
		t.Fatalf("a single missed observation flapped the pass list: %+v", rows[0])
	}
	now = now.Add(poll)

	// Sustained book loss does fail the "книга" gate, and with M=1 the pass list
	// is left on the very first failing window.
	var failedAt int
	for i := 0; i < 10; i++ {
		rows = a.scoreWindow([]string{"tok"}, dead, now)
		now = now.Add(poll)
		if rows[0].Passed == 0 {
			failedAt = i + 1
			break
		}
		if !c.inPassList {
			t.Fatalf("left the pass list on window %d without failing the gates", i+1)
		}
	}
	if failedAt == 0 {
		t.Fatal("sustained book loss never failed the gates")
	}
	if c.inPassList || rows[0].InPassList != 0 || c.failStreak != 1 || c.passStreak != 0 {
		t.Fatalf("M=1: must leave the pass list on the first failing window; got inPassList=%v pass=%d fail=%d", c.inPassList, c.passStreak, c.failStreak)
	}
	if fails := rows[0].Fails; fails != screener.FailBook {
		t.Errorf("fails = %q, want the canon's book gate %q", fails, screener.FailBook)
	}
	if len(a.selectForPromotion()) != 0 {
		t.Error("a candidate outside the pass list must not be promoted")
	}
}

// TestScoreWindowFinalPhaseForcesFail is §118 end to end: a RECORDING market in
// its final phase leaves the pass list even though its metrics still pass, and
// its state stays RECORDING — it is recorded to resolution regardless.
func TestScoreWindowFinalPhaseForcesFail(t *testing.T) {
	cfg := config.PolymarketAuto{PollIntSec: 300, ObservationWindowSec: 7200, HysteresisK: 1, HysteresisM: 1}
	a := newTestAuto(t, cfg)

	start := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	// Expiry lands just after the observation warm-up, so the judged windows sit
	// inside the final 10% of the lifetime.
	c := &autoCandidate{
		yesTokenID: "tok", tickSize: 0.001, state: polyAutoStateRecording,
		createdTs: start.Add(-7 * 24 * time.Hour),
		expiryTs:  start.Add(150 * time.Minute),
	}
	a.cands["tok"] = c

	pass := map[string]*polyTopOfBook{"tok": {
		bid: 0.498, ask: 0.502, bidSize: 500, askSize: 500, hasBid: true, hasAsk: true, tickSize: 0.001,
	}}
	poll := time.Duration(cfg.PollIntSec) * time.Second
	now := start
	for i := 0; i < 25; i++ {
		a.scoreWindow([]string{"tok"}, pass, now)
		now = now.Add(poll)
	}
	rows := a.scoreWindow([]string{"tok"}, pass, now)

	if rows[0].Passed != 1 {
		t.Fatal("metrics should still pass the gates in the final phase")
	}
	if c.inPassList || rows[0].InPassList != 0 {
		t.Error("§118: the final phase must force in_pass_list=0 regardless of metrics")
	}
	if c.state != polyAutoStateRecording || rows[0].State != polyAutoStateRecording {
		t.Error("§118: losing the pass list must NOT stop the recording")
	}
}

// TestSelectForPromotionBudget checks the budget blocks NEW promotions and picks
// the best scores, while markets already recording are never evicted.
func TestSelectForPromotionBudget(t *testing.T) {
	a := newTestAuto(t, config.PolymarketAuto{MaxRecording: 3})
	a.cands["rec1"] = &autoCandidate{yesTokenID: "rec1", state: polyAutoStateRecording, lastScore: 1}
	a.cands["rec2"] = &autoCandidate{yesTokenID: "rec2", state: polyAutoStateRecording, lastScore: 2}
	for id, score := range map[string]float64{"c-low": 10, "c-high": 900, "c-mid": 100} {
		a.cands[id] = &autoCandidate{yesTokenID: id, state: polyAutoStateCandidate, inPassList: true, lastScore: score}
	}

	// One slot left (3 - 2 recording): the highest score takes it.
	got := a.selectForPromotion()
	if len(got) != 1 || got[0].yesTokenID != "c-high" {
		t.Fatalf("got %d promotions (%v), want just c-high", len(got), promotionIDs(got))
	}

	// Budget full: no new promotions, and the recordings are untouched.
	a.cands["rec3"] = &autoCandidate{yesTokenID: "rec3", state: polyAutoStateRecording, lastScore: 3}
	if got := a.selectForPromotion(); len(got) != 0 {
		t.Errorf("promoted %v with the budget full", promotionIDs(got))
	}
	if a.recordingCountLocked() != 3 {
		t.Errorf("recording count = %d, want 3 (recordings are never evicted)", a.recordingCountLocked())
	}
}

func promotionIDs(cs []*autoCandidate) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.yesTokenID)
	}
	return out
}

// TestRetireStale checks what leaves the watch list: expired candidates and
// tokens a configured subject has claimed. A healthy RECORDING market stays.
func TestRetireStale(t *testing.T) {
	a := newTestAuto(t, config.PolymarketAuto{})
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)

	a.cands["expired"] = &autoCandidate{yesTokenID: "expired", state: polyAutoStateCandidate, expiryTs: now.Add(-time.Hour)}
	a.cands["live"] = &autoCandidate{yesTokenID: "live", state: polyAutoStateCandidate, expiryTs: now.Add(48 * time.Hour)}
	a.cands["recording"] = &autoCandidate{yesTokenID: "recording", state: polyAutoStateRecording, expiryTs: now.Add(-time.Hour)}
	a.cands["claimed"] = &autoCandidate{yesTokenID: "claimed", state: polyAutoStateCandidate, expiryTs: now.Add(48 * time.Hour)}
	a.cands["stuck"] = &autoCandidate{yesTokenID: "stuck", state: polyAutoStateRecording, expiryTs: now.Add(-30 * 24 * time.Hour)}
	a.conn().meta["claimed"] = tokenMeta{subject: "BTC"}

	rows := a.retireStale(now)

	if _, ok := a.cands["expired"]; ok {
		t.Error("an expired candidate must be dropped")
	}
	if _, ok := a.cands["claimed"]; ok {
		t.Error("a token claimed by a configured subject must be dropped")
	}
	if _, ok := a.cands["recording"]; !ok {
		t.Error("a market past expiry but still RECORDING must be kept until it resolves")
	}
	if _, ok := a.cands["stuck"]; ok {
		t.Error("a recording long past expiry must release its budget slot")
	}
	if _, ok := a.cands["live"]; !ok {
		t.Error("a live candidate must be kept")
	}
	if len(rows) != 3 {
		t.Fatalf("got %d DROPPED rows, want 3", len(rows))
	}
	for _, r := range rows {
		if r.State != polyAutoStateDropped || r.Subject != polyAutoSubject {
			t.Errorf("row = %s/%s, want DROPPED/AUTO", r.State, r.Subject)
		}
	}
}

// TestGammaCategory checks a known top-level label wins over an incidental tag,
// with the first label as the fallback.
func TestGammaCategory(t *testing.T) {
	cases := []struct {
		name string
		tags []gammaTag
		want string
	}{
		{"known label wins", []gammaTag{{Label: "NBA"}, {Label: "Sports"}}, "Sports"},
		{"case insensitive", []gammaTag{{Label: "crypto"}}, "Crypto"},
		{"falls back to first", []gammaTag{{Label: "Fed Rates"}, {Label: "Recession"}}, "Fed Rates"},
		{"no tags", nil, ""},
		{"skips blanks", []gammaTag{{Label: "  "}, {Label: "Mentions"}}, "Mentions"},
	}
	for _, c := range cases {
		if got := gammaCategory(&gammaEvent{Tags: c.tags}); got != c.want {
			t.Errorf("%s: gammaCategory = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestIsRetryableHTTPErr keeps the backoff aimed at rate limits and transient
// server failures — never at Gamma's 422 offset cap, which must stop the sweep.
func TestIsRetryableHTTPErr(t *testing.T) {
	cases := []struct {
		err  string
		want bool
	}{
		{"polymarket GET x: status 429: slow down", true},
		{"polymarket GET x: status 503: unavailable", true},
		{"dial tcp: i/o timeout", true},
		{"polymarket GET x: status 422: offset too large, use /events/keyset", false},
		{"polymarket GET x: status 404: No orderbook exists", false},
	}
	for _, c := range cases {
		if got := isRetryableHTTPErr(errString(c.err)); got != c.want {
			t.Errorf("%q: retryable = %v, want %v", c.err, got, c.want)
		}
	}
	if isRetryableHTTPErr(nil) {
		t.Error("nil error is not retryable")
	}
	if !isGammaOffsetLimit(errString("status 422: offset too large")) {
		t.Error("the Gamma offset cap must be recognised so the sweep stops cleanly")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
