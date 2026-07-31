//go:build clickhousereplay

// Historic-replay acceptance harness for the Go screener port.
//
// Golden vectors (screener_test.go) pin the FORMULAS. This pins the port
// end-to-end against REAL recorded data: it replays the raw CLOB streams
// already in polymarket_book — the exact same rows the Python reference
// (mm_engine/screener_validate2.py, screener_subjects.py) reads — rebuilds each
// token's book from its anchors and deltas, and reproduces the reference
// verdicts from mm_engine/REVIEW.md §122b/§123:
//
//	BTC hourly ABOVE : ~4 PASS out of 41 (the screener cuts the loss-making set)
//	WEATHER_LOW      : Paris and Seoul PASS (real quotable conditions)
//	SPY / WTI ABOVE  : all fail
//
// Excluded from normal builds by the tag, since it needs the recorder's
// ClickHouse. Run:
//
//	go test -tags clickhousereplay ./internal/screener/ -run TestHistoricReplay -v
//
// Point it elsewhere with PM_CH_URL / PM_CH_DB.
package screener

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	_ "github.com/ClickHouse/clickhouse-go"
)

const (
	defaultCHURL = "tcp://109.226.194.108:5923"
	defaultCHDB  = "default"
)

func openCH(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("PM_CH_URL")
	if url == "" {
		url = defaultCHURL
	}
	db := os.Getenv("PM_CH_DB")
	if db == "" {
		db = defaultCHDB
	}
	conn, err := sql.Open("clickhouse", url+"?database="+db+"&read_timeout=300&write_timeout=300")
	if err != nil {
		t.Fatalf("open clickhouse: %v", err)
	}
	if err := conn.Ping(); err != nil {
		t.Fatalf("ping clickhouse at %s: %v", url, err)
	}
	return conn
}

// strike is one binary market of an event; only the Yes token is screened,
// matching the reference (screen_event takes st["yes_token"]).
type strike struct {
	conditionID string
	yesToken    string
	tick        float64
}

type replayEvent struct {
	eventID   string
	slug      string
	createdTs int64 // seconds
	expiryTs  int64 // seconds
	strikes   []strike
}

// loadEvents is pm_reader.list_events: resolved events of one subject and market
// type, with their strikes, ordered by expiry.
func loadEvents(t *testing.T, conn *sql.DB, subject, marketType string) []replayEvent {
	t.Helper()
	rows, err := conn.Query(
		"SELECT event_id, event_slug, condition_id, token_id, token_index, tick_size, "+
			"toUnixTimestamp(created_ts), toUnixTimestamp(expiry_ts) "+
			"FROM polymarket_market WHERE market_type = ? AND subject = ? AND resolved = 1 "+
			"ORDER BY expiry_ts, event_id, condition_id, token_index",
		marketType, subject)
	if err != nil {
		t.Fatalf("query markets: %v", err)
	}
	defer rows.Close()

	byID := map[string]*replayEvent{}
	var order []string
	for rows.Next() {
		var eid, slug, cond, tok string
		var tidx uint8
		var tick float64
		var cts, ets int64
		if err := rows.Scan(&eid, &slug, &cond, &tok, &tidx, &tick, &cts, &ets); err != nil {
			t.Fatalf("scan market: %v", err)
		}
		ev := byID[eid]
		if ev == nil {
			ev = &replayEvent{eventID: eid, slug: slug, createdTs: cts, expiryTs: ets}
			byID[eid] = ev
			order = append(order, eid)
		}
		if cts < ev.createdTs {
			ev.createdTs = cts
		}
		if ets > ev.expiryTs {
			ev.expiryTs = ets
		}
		if tidx == 0 {
			ev.strikes = append(ev.strikes, strike{conditionID: cond, yesToken: tok, tick: tick})
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("markets rows: %v", err)
	}

	out := make([]replayEvent, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].expiryTs < out[j].expiryTs })
	return out
}

// loadSamples replays every token of an event and returns, per token, the
// top-of-book series. This is pm_reader's assembler: `book` messages replace the
// book, `price_change` messages patch it (size 0 removes a level), prices are
// normalised to 4 decimals ON THE LEVEL KEY (so two raw prices that round
// together collide, exactly as they do in the reference's dict), and only levels
// with size > 0 exist.
//
// One sample per message would be redundant: TokenMetrics snapshots as-of, so
// only the messages that MOVE the touch can ever be observed. Emitting just
// those is equivalent and keeps a whole event in memory comfortably.
func loadSamples(t *testing.T, conn *sql.DB, tokens []string, endMs int64) map[string][]Sample {
	t.Helper()
	if len(tokens) == 0 {
		return nil
	}
	quoted := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if !isDigits(tok) {
			t.Fatalf("unexpected token id %q (want digits)", tok)
		}
		quoted = append(quoted, "'"+tok+"'")
	}
	q := fmt.Sprintf(
		"SELECT token_id, toUnixTimestamp64Milli(timestamp), msg_type, data "+
			"FROM polymarket_book WHERE token_id IN (%s) AND toUnixTimestamp64Milli(timestamp) <= %d "+
			"ORDER BY token_id, seq", strings.Join(quoted, ","), endMs)
	rows, err := conn.Query(q)
	if err != nil {
		t.Fatalf("query book: %v", err)
	}
	defer rows.Close()

	out := map[string][]Sample{}
	books := map[string]map[float64]float64{} // token -> bids
	asksB := map[string]map[float64]float64{} // token -> asks
	last := map[string]Sample{}

	for rows.Next() {
		var tok, mtype, data string
		var tsMs int64
		if err := rows.Scan(&tok, &tsMs, &mtype, &data); err != nil {
			t.Fatalf("scan book: %v", err)
		}
		bids, asks := books[tok], asksB[tok]
		if bids == nil {
			bids, asks = map[float64]float64{}, map[float64]float64{}
			books[tok], asksB[tok] = bids, asks
		}

		switch mtype {
		case "book":
			var d struct {
				Bids [][2]string `json:"bids"`
				Asks [][2]string `json:"asks"`
			}
			if err := json.Unmarshal([]byte(data), &d); err != nil {
				continue
			}
			bids, asks = map[float64]float64{}, map[float64]float64{}
			for _, l := range d.Bids {
				p, s := parseF(l[0]), parseF(l[1])
				bids[RoundPrice(p)] = s
			}
			for _, l := range d.Asks {
				p, s := parseF(l[0]), parseF(l[1])
				asks[RoundPrice(p)] = s
			}
			books[tok], asksB[tok] = bids, asks
		case "price_change":
			var d [][3]string
			if err := json.Unmarshal([]byte(data), &d); err != nil {
				continue
			}
			for _, ch := range d {
				p, s := RoundPrice(parseF(ch[0])), parseF(ch[1])
				book := asks
				if strings.EqualFold(ch[2], "BUY") {
					book = bids
				}
				if s == 0 {
					delete(book, p)
				} else {
					book[p] = s
				}
			}
		default:
			continue
		}

		s := touch(bids, asks)
		s.TsMs = tsMs
		// Only a change in the touch can ever be observed by an as-of snapshot.
		if prev, ok := last[tok]; ok && sameTouch(prev, s) {
			continue
		}
		last[tok] = s
		out[tok] = append(out[tok], s)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("book rows: %v", err)
	}
	return out
}

// touch reduces the book to its best levels, ignoring levels with size <= 0
// (pm_reader._sorted_levels).
func touch(bids, asks map[float64]float64) Sample {
	var s Sample
	for p, sz := range bids {
		if sz <= 0 {
			continue
		}
		if !s.HasBid || p > s.Bid {
			s.Bid, s.BidSize, s.HasBid = p, sz, true
		}
	}
	for p, sz := range asks {
		if sz <= 0 {
			continue
		}
		if !s.HasAsk || p < s.Ask {
			s.Ask, s.AskSize, s.HasAsk = p, sz, true
		}
	}
	return s
}

func sameTouch(a, b Sample) bool {
	return a.HasBid == b.HasBid && a.HasAsk == b.HasAsk &&
		a.Bid == b.Bid && a.Ask == b.Ask && a.BidSize == b.BidSize && a.AskSize == b.AskSize
}

func parseF(s string) float64 {
	var v float64
	fmt.Sscanf(strings.TrimSpace(s), "%g", &v)
	return v
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// verdict is the screened result of one event: the BEST of its tokens, ranked
// score-if-passing else -1 (screen_event / screener_validate2 semantics).
type verdict struct {
	slug   string
	passed bool
	score  float64
	m      *Metrics
	fails  []string
}

// screenEvents screens each event over the window its reference harness uses.
func screenEvents(t *testing.T, conn *sql.DB, evs []replayEvent, window func(*sql.DB, replayEvent) (int64, int64, bool)) []verdict {
	t.Helper()
	var out []verdict
	for _, ev := range evs {
		w0, w1, ok := window(conn, ev)
		if !ok {
			continue
		}
		tokens := make([]string, 0, len(ev.strikes))
		for _, st := range ev.strikes {
			if st.yesToken != "" {
				tokens = append(tokens, st.yesToken)
			}
		}
		samples := loadSamples(t, conn, tokens, ev.expiryTs*1000+60_000)

		best := verdict{slug: ev.slug, score: -1}
		found := false
		for _, st := range ev.strikes {
			ss := samples[st.yesToken]
			if len(ss) == 0 {
				continue
			}
			tick := st.tick
			if tick <= 0 {
				tick = 0.001
			}
			m := TokenMetrics(ss, w0, w1, tick)
			if m == nil {
				continue
			}
			passed, score, fails := GatesAndScore(m)
			rank := -1.0
			if passed {
				rank = score
			}
			if !found || rank > bestRank(best) {
				best = verdict{slug: ev.slug, passed: passed, score: score, m: m, fails: fails}
				found = true
			}
		}
		if found {
			out = append(out, best)
		}
	}
	return out
}

func bestRank(v verdict) float64 {
	if v.passed {
		return v.score
	}
	return -1
}

func report(t *testing.T, title string, vs []verdict) (pass int) {
	t.Helper()
	t.Logf("=== %s: %d events with data ===", title, len(vs))
	for _, v := range vs {
		label := "fail"
		if v.passed {
			label = "PASS"
			pass++
		}
		t.Logf("  %-44s %s score=%8.0f | spread %5.1ft depth %6.0f jumps %5.1f%% noise %5.1ft r2 %.2f %s",
			trunc(v.slug, 44), label, v.score, v.m.SpreadMed, v.m.DepthMed,
			100*v.m.JumpRate, v.m.ResStdT, v.m.R2, strings.Join(v.fails, ","))
	}
	t.Logf("  --> PASS %d / %d", pass, len(vs))
	return pass
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// hourlyWindow is screener_validate2's window for BTC hourlies:
// [t0 + 20% of life, expiry - 5 min].
func hourlyWindow(_ *sql.DB, ev replayEvent) (int64, int64, bool) {
	t0, t1 := ev.createdTs*1000, ev.expiryTs*1000
	if t1-t0 > 3*3600*1000 { // hourlies only
		return 0, 0, false
	}
	return t0 + (t1-t0)/5, t1 - 5*60_000, true
}

// liquidWindow is screener_subjects' window for the non-BTC subjects: start from
// the event's first price_change (markets are minted long before they trade),
// then [g0 + 10%, expiry - 5 min].
func liquidWindow(conn *sql.DB, ev replayEvent) (int64, int64, bool) {
	t0, t1 := ev.createdTs*1000, ev.expiryTs*1000
	var first sql.NullInt64
	err := conn.QueryRow(
		"SELECT min(toUnixTimestamp64Milli(timestamp)) FROM polymarket_book "+
			"WHERE event_id = ? AND msg_type = 'price_change'", ev.eventID).Scan(&first)
	g0 := t0
	if err == nil && first.Valid && first.Int64 > g0 {
		g0 = first.Int64
	}
	if g0 >= t1 {
		return 0, 0, false
	}
	w0 := g0 + (t1-g0)/10
	w1 := t1 - 5*60_000
	if w1 <= w0 {
		return 0, 0, false
	}
	return w0, w1, true
}

// TestHistoricReplay reproduces the reference verdicts of REVIEW.md §122b/§123
// on the recorder's own data.
func TestHistoricReplay(t *testing.T) {
	conn := openCH(t)
	defer conn.Close()

	t.Run("BTC hourly ABOVE cuts almost everything", func(t *testing.T) {
		vs := screenEvents(t, conn, loadEvents(t, conn, "BTC", "ABOVE"), hourlyWindow)
		pass := report(t, "BTC hourly ABOVE", vs)
		if len(vs) < 30 {
			t.Fatalf("only %d hourlies had data; the reference set was 41", len(vs))
		}
		// §122b: 4 PASS of 41. The screener's job is to cut the loss-making
		// bulk, so what matters is that the pass rate stays in that régime.
		if got := float64(pass) / float64(len(vs)); got > 0.20 {
			t.Errorf("PASS rate %.1f%% (%d/%d); §122b saw ~10%% (4/41) — the port is letting hourlies through",
				100*got, pass, len(vs))
		}
	})

	t.Run("WEATHER_LOW finds the quotable cities", func(t *testing.T) {
		vs := screenEvents(t, conn, loadEvents(t, conn, "WEATHER_LOW", "BUCKET"), liquidWindow)
		report(t, "WEATHER_LOW BUCKET", vs)
		passed := map[string]bool{}
		for _, v := range vs {
			if v.passed {
				passed[strings.ToLower(v.slug)] = true
			}
		}
		// §123: Paris and Seoul are the two PASS cities.
		for _, city := range []string{"paris", "seoul"} {
			found := false
			for slug := range passed {
				if strings.Contains(slug, city) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("§123: %s should PASS but did not", city)
			}
		}
	})

	for _, subject := range []string{"SPY", "WTI"} {
		t.Run(subject+" ABOVE is all fail", func(t *testing.T) {
			vs := screenEvents(t, conn, loadEvents(t, conn, subject, "ABOVE"), liquidWindow)
			pass := report(t, subject+" ABOVE", vs)
			if pass != 0 {
				t.Errorf("§123: %s should be entirely cut, got %d PASS", subject, pass)
			}
		})
	}
}
