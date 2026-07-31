//go:build polymarketlive

// Live smoke test for the auto mode's two API integrations: the bounded Gamma
// universe sweep (coarse filter) and the batched CLOB top-of-book poll. It hits
// the real Polymarket APIs read-only — no orders, no websocket, no database —
// so it is excluded from normal builds by the tag.
//
//	go test -tags polymarketlive ./internal/exchange/ -run TestAutoLive -v
package exchange

import (
	"context"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/milkywaybrain/cryptogalaxy/internal/config"
)

func newLiveAuto(t *testing.T, cfg config.PolymarketAuto) *polyAuto {
	t.Helper()
	a := newPolyAuto(cfg)
	a.attach(&polymarket{
		subjectCfg: map[string]*polySubjectCfg{},
		meta:       map[string]tokenMeta{},
		subscribed: map[string]struct{}{},
		known:      map[string]*knownMarket{},
		http:       &http.Client{Timeout: 30 * time.Second},
	})
	return a
}

// TestAutoLiveScanAndPoll runs one real universe sweep and then one real batched
// top-of-book poll over whatever it found, reporting what the coarse filter and
// the CLOB actually return. This is the check that the live run depends on:
// candidates are found, and their books come back readable.
func TestAutoLiveScanAndPoll(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// A short sweep: enough to prove pagination, ordering and the filter work.
	a := newLiveAuto(t, config.PolymarketAuto{ScanMaxPages: 3, MaxCandidates: 500})

	if err := a.scanUniverse(ctx); err != nil {
		t.Fatalf("universe scan failed: %v", err)
	}
	a.mu.Lock()
	tokens := make([]string, 0, len(a.cands))
	byCategory := map[string]int{}
	byType := map[string]int{}
	for tok, c := range a.cands {
		tokens = append(tokens, tok)
		byCategory[c.category]++
		byType[c.marketType]++
	}
	a.mu.Unlock()

	if len(tokens) == 0 {
		t.Fatal("the coarse filter kept nothing from the top-by-liquidity slice; thresholds are too tight or the Gamma fields moved")
	}
	t.Logf("candidates after the coarse filter: %d", len(tokens))
	t.Logf("by category: %v", sortedCounts(byCategory))
	t.Logf("by market_type: %v", sortedCounts(byType))

	// One batched poll over every candidate — the per-window cost of the mode.
	start := time.Now()
	books, err := a.fetchBooks(ctx, tokens)
	if err != nil {
		t.Fatalf("batched top-of-book poll failed: %v", err)
	}
	elapsed := time.Since(start)

	var twoSided, oneSided, empty, missing int
	for _, tok := range tokens {
		b := books[tok]
		switch {
		case b == nil:
			missing++
		case b.hasBid && b.hasAsk:
			twoSided++
		case b.hasBid || b.hasAsk:
			oneSided++
		default:
			empty++
		}
	}
	t.Logf("POST /books: %d tokens in %v (%d requests) -> two-sided %d, one-sided %d, empty %d, absent from response %d",
		len(tokens), elapsed.Round(time.Millisecond),
		(len(tokens)+polyAutoBooksChunk-1)/polyAutoBooksChunk,
		twoSided, oneSided, empty, missing)

	if twoSided == 0 {
		t.Error("no candidate came back with a two-sided book; the /books response shape may have changed")
	}

	// Spot-check that a returned touch is sane — a screener fed nonsense here
	// would silently produce nonsense metrics.
	for _, tok := range tokens {
		b := books[tok]
		if b == nil || !b.hasBid || !b.hasAsk {
			continue
		}
		if b.bid <= 0 || b.ask <= 0 || b.bid >= b.ask || b.ask > 1 {
			t.Errorf("token %s: implausible touch bid=%v ask=%v", tok, b.bid, b.ask)
		}
		if b.bidSize <= 0 || b.askSize <= 0 {
			t.Errorf("token %s: zero size at the touch (%v/%v)", tok, b.bidSize, b.askSize)
		}
		if b.tickSize <= 0 || b.tickSize > 0.1 {
			t.Errorf("token %s: implausible tick size %v", tok, b.tickSize)
		}
	}
}

func sortedCounts(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] })
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if k == "" {
			k = "(none)"
		}
		out = append(out, k+"="+itoa(m[k]))
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
