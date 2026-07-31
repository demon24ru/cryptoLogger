// Package screener is the Go port of the "zero curvature" market screener that
// selects Polymarket markets worth recording for ladder market-making.
//
// CANON: mm_engine/screener_zero_curvature.py (token_metrics + gates_and_score).
// This package must reproduce it exactly; parity is enforced by the golden
// vectors in mm_engine/data/screener_golden.json (see screener_test.go, 1e-9
// tolerance). Formulas may only change in the Python canon first — then the
// vectors are regenerated (mm_engine/export_screener_golden.py) and this port is
// updated, which the golden test will demand.
package screener

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

// Canon parameters (screener_zero_curvature.py module constants).
const (
	StepMin   = 5   // mid grid step, minutes
	JumpTicks = 2.0 // |dmid| above this many ticks counts as a repricing jump
	MinDepth  = 50.0
)

// Gate thresholds (gates_and_score).
const (
	GateTwoSided  = 0.9
	GateSpreadMed = 2.0
	GateJumpRate  = 0.05
	GateResStdT   = 3.0
)

// Gates are the screening thresholds. The zero value is NOT usable — build one
// with DefaultGates (the canon values above) and override individual fields from
// config. Thresholds are the only part of the screener the operator may tune;
// the FORMULAS have a single source of truth (the Python canon + golden vectors).
type Gates struct {
	MinTwoSided  float64 // two_sided must be >= this
	MinSpreadMed float64 // spread_med (ticks) must be >= this
	MinDepthMed  float64 // depth_med must be >= this
	MaxJumpRate  float64 // jump_rate must be <= this
	MaxResStdT   float64 // res_std_t (ticks) must be <= this
}

// DefaultGates returns the canon thresholds (screener_zero_curvature.py
// gates_and_score, REVIEW.md §122).
func DefaultGates() Gates {
	return Gates{
		MinTwoSided:  GateTwoSided,
		MinSpreadMed: GateSpreadMed,
		MinDepthMed:  MinDepth,
		MaxJumpRate:  GateJumpRate,
		MaxResStdT:   GateResStdT,
	}
}

// Fail reasons, verbatim from the canon so screener rows and Python output agree.
const (
	FailBook       = "книга"
	FailSpread     = "спред<2т"
	FailDepth      = "глубина"
	FailRepricings = "переоценки"
	FailNoise      = "шум>3т"
)

// Sample is one top-of-book observation of a token. Bid/Ask are prices in [0,1];
// BidSize/AskSize are the sizes resting at those prices. A side that is absent
// (empty book) is expressed by HasBid/HasAsk being false — matching the canon,
// where a missing side makes the sample not count toward two_sided.
type Sample struct {
	TsMs    int64
	Bid     float64
	Ask     float64
	BidSize float64
	AskSize float64
	HasBid  bool
	HasAsk  bool
}

// Metrics mirrors the dict returned by canon token_metrics.
type Metrics struct {
	TwoSided  float64
	SpreadMed float64
	DepthMed  float64
	R2        float64
	ResStdT   float64
	CurvT     float64
	JumpRate  float64
	MidLast   float64
}

// TokenMetrics computes the canon metrics for one token over [t0Ms, t1Ms].
//
// It reproduces token_metrics: build an n-point grid over the window, take an
// as-of snapshot of the book at each grid point (state after all samples with
// ts <= grid point), then measure. Returns nil when fewer than 8 grid points
// have a two-sided book — the canon's "not enough data" case.
//
// samples must be sorted by TsMs ascending.
func TokenMetrics(samples []Sample, t0Ms, t1Ms int64, tick float64) *Metrics {
	// n = max(8, int((t1 - t0) / (STEP_MIN * 60_000)))
	n := int((t1Ms - t0Ms) / int64(StepMin*60_000))
	if n < 8 {
		n = 8
	}
	// ts = [int(t0 + (t1 - t0) * i / (n - 1)) for i in range(n)]
	grid := make([]int64, n)
	for i := 0; i < n; i++ {
		grid[i] = t0Ms + (t1Ms-t0Ms)*int64(i)/int64(n-1)
	}

	snaps := snapshotAtTicks(samples, grid)

	mids := make([]float64, 0, n) // compacted: only two-sided grid points
	spreads := make([]float64, 0, n)
	depths := make([]float64, 0, n)
	two := 0
	for _, s := range snaps {
		// Canon: a level exists only with size > 0 (_sorted_levels), and its
		// price is normalized to the tick grid on parse (_r = round(p, 4)).
		if !s.HasBid || !s.HasAsk || s.BidSize <= 0 || s.AskSize <= 0 {
			continue // canon appends None to mids and skips; None is dropped later
		}
		bid, ask := RoundPrice(s.Bid), RoundPrice(s.Ask)
		two++
		mids = append(mids, (bid+ask)/2.0)
		spreads = append(spreads, (ask-bid)/tick)
		depths = append(depths, s.BidSize+s.AskSize)
	}

	twoSided := float64(two) / math.Max(float64(n), 1)
	if len(mids) < 8 {
		return nil
	}

	// Least-squares line over the COMPACTED index (canon: t_idx = arange(len(m))).
	k, b0 := polyfit1(mids)
	resid := make([]float64, len(mids))
	for i, m := range mids {
		resid[i] = m - (k*float64(i) + b0)
	}
	ssRes := 0.0
	for _, r := range resid {
		ssRes += r * r
	}
	mean := 0.0
	for _, m := range mids {
		mean += m
	}
	mean /= float64(len(mids))
	ssTot := 0.0
	for _, m := range mids {
		ssTot += (m - mean) * (m - mean)
	}
	r2 := 1.0
	if ssTot > 1e-12 {
		r2 = 1.0 - ssRes/ssTot
	}

	d1 := diff(mids)
	d2 := diff(d1)

	m := &Metrics{
		TwoSided: twoSided,
		R2:       r2,
		ResStdT:  stdPop(resid) / tick,
		MidLast:  mids[len(mids)-1],
	}
	if len(spreads) > 0 {
		m.SpreadMed = median(spreads)
	}
	if len(depths) > 0 {
		m.DepthMed = median(depths)
	}
	if len(d2) > 0 {
		sum := 0.0
		for _, v := range d2 {
			sum += math.Abs(v)
		}
		m.CurvT = (sum / float64(len(d2))) / tick
	}
	if len(d1) > 0 {
		hits := 0
		for _, v := range d1 {
			if math.Abs(v) > JumpTicks*tick {
				hits++
			}
		}
		m.JumpRate = float64(hits) / float64(len(d1))
	}
	return m
}

// GatesAndScore applies the canon gates and ranking score.
// Returns (passed, score, failed-gate reasons).
func GatesAndScore(m *Metrics) (bool, float64, []string) {
	return GatesAndScoreWith(m, DefaultGates())
}

// GatesAndScoreWith is GatesAndScore with operator-tuned thresholds. The gate
// COMPARISONS, the fail labels and the score formula are the canon's; only the
// numbers move. Called with DefaultGates it is bit-for-bit the canon, which is
// what the golden test exercises.
func GatesAndScoreWith(m *Metrics, g Gates) (bool, float64, []string) {
	var fails []string
	if m.TwoSided < g.MinTwoSided {
		fails = append(fails, FailBook)
	}
	if m.SpreadMed < g.MinSpreadMed {
		fails = append(fails, FailSpread)
	}
	if m.DepthMed < g.MinDepthMed {
		fails = append(fails, FailDepth)
	}
	if m.JumpRate > g.MaxJumpRate {
		fails = append(fails, FailRepricings)
	}
	if m.ResStdT > g.MaxResStdT {
		fails = append(fails, FailNoise)
	}
	score := 0.0
	if len(fails) == 0 {
		score = m.SpreadMed * math.Min(m.DepthMed, 500.0) * math.Max(0.0, 1.0-5.0*m.JumpRate)
	}
	return len(fails) == 0, score, fails
}

// FailsCSV renders fail reasons for the screener table's `fails` column.
func FailsCSV(fails []string) string { return strings.Join(fails, ",") }

// RoundPrice normalizes a price to the Polymarket tick grid the way the canon
// does (pm_reader._r = round(float(x), 4) — 4 decimals is finer than the 0.001
// tick). Formatting to 4 decimals and re-parsing reproduces Python's
// round-half-to-even on the exact binary value, which naive x*1e4 arithmetic
// does not. On real CLOB books (already tick-aligned) this is a no-op; it
// matters for exact golden parity.
func RoundPrice(x float64) float64 {
	v, err := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 4, 64), 64)
	if err != nil {
		return x
	}
	return v
}

// snapshotAtTicks replays samples onto the grid, returning for each grid point
// the last sample with ts <= grid point (canon snapshot_at_ticks semantics:
// state AFTER all messages at or before the tick). Grid points before the first
// sample yield an empty book.
func snapshotAtTicks(samples []Sample, grid []int64) []Sample {
	out := make([]Sample, len(grid))
	cur := Sample{} // empty book until the first sample lands
	si := 0
	for gi, g := range grid {
		for si < len(samples) && samples[si].TsMs <= g {
			cur = samples[si]
			si++
		}
		out[gi] = cur
	}
	return out
}

// polyfit1 is numpy.polyfit(x, y, 1) for x = 0..n-1: ordinary least squares.
func polyfit1(y []float64) (slope, intercept float64) {
	n := float64(len(y))
	var sx, sy, sxx, sxy float64
	for i, v := range y {
		x := float64(i)
		sx += x
		sy += v
		sxx += x * x
		sxy += x * v
	}
	den := n*sxx - sx*sx
	if den == 0 {
		return 0, sy / n
	}
	slope = (n*sxy - sx*sy) / den
	intercept = (sy - slope*sx) / n
	return slope, intercept
}

// median is numpy.median: average of the two middle values for even length.
func median(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2.0
}

// stdPop is numpy.std (population, ddof=0).
func stdPop(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	mean := 0.0
	for _, x := range v {
		mean += x
	}
	mean /= float64(len(v))
	acc := 0.0
	for _, x := range v {
		d := x - mean
		acc += d * d
	}
	return math.Sqrt(acc / float64(len(v)))
}

func diff(v []float64) []float64 {
	if len(v) < 2 {
		return nil
	}
	out := make([]float64, len(v)-1)
	for i := 1; i < len(v); i++ {
		out[i-1] = v[i] - v[i-1]
	}
	return out
}
