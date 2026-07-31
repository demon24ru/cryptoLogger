package screener

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// goldenPath points at the canon-side vectors. Kept as a var so the location can
// be overridden via SCREENER_GOLDEN when the analysis repo lives elsewhere.
var goldenPath = filepath.FromSlash(`C:/AI/A_chat_mm/mm_engine/data/screener_golden.json`)

const tol = 1e-9

type goldenFile struct {
	Canon string       `json:"canon"`
	Cases []goldenCase `json:"cases"`
}

type goldenCase struct {
	Name   string            `json:"name"`
	Tick   float64           `json:"tick"`
	T0     int64             `json:"t0"`
	T1     int64             `json:"t1"`
	Series [][]interface{}   `json:"series"` // [ts, bid|null, ask|null, bsz, asz]
	Expect *goldenExpectJSON `json:"expect"`
}

type goldenExpectJSON struct {
	TwoSided  float64  `json:"two_sided"`
	SpreadMed float64  `json:"spread_med"`
	DepthMed  float64  `json:"depth_med"`
	R2        float64  `json:"r2"`
	ResStdT   float64  `json:"res_std_t"`
	CurvT     float64  `json:"curv_t"`
	JumpRate  float64  `json:"jump_rate"`
	MidLast   float64  `json:"mid_last"`
	Passed    bool     `json:"passed"`
	Score     float64  `json:"score"`
	Fails     []string `json:"fails"`
}

// TestGoldenParity is the anti-fork guard for this cross-language port: the Go
// screener must reproduce the Python canon's metrics, gates and score on every
// golden profile. If the canon changes, the vectors are regenerated and this
// test fails, surfacing the drift instead of letting the two sides diverge.
func TestGoldenParity(t *testing.T) {
	if p := os.Getenv("SCREENER_GOLDEN"); p != "" {
		goldenPath = p
	}
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Skipf("golden vectors unavailable (%v); set SCREENER_GOLDEN to run parity test", err)
	}
	var gf goldenFile
	if err := json.Unmarshal(raw, &gf); err != nil {
		t.Fatalf("parse golden: %v", err)
	}
	if len(gf.Cases) == 0 {
		t.Fatal("golden file has no cases")
	}

	for _, c := range gf.Cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			samples := make([]Sample, 0, len(c.Series))
			for i, row := range c.Series {
				if len(row) < 5 {
					t.Fatalf("series[%d]: want 5 fields, got %d", i, len(row))
				}
				s := Sample{TsMs: int64(toF(row[0]))}
				if row[1] != nil && row[2] != nil {
					s.Bid, s.Ask = toF(row[1]), toF(row[2])
					s.HasBid, s.HasAsk = true, true
				}
				s.BidSize, s.AskSize = toF(row[3]), toF(row[4])
				samples = append(samples, s)
			}

			got := TokenMetrics(samples, c.T0, c.T1, c.Tick)
			if c.Expect == nil {
				if got != nil {
					t.Fatalf("expected nil metrics, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("got nil metrics, expected values")
			}

			close(t, "two_sided", got.TwoSided, c.Expect.TwoSided)
			close(t, "spread_med", got.SpreadMed, c.Expect.SpreadMed)
			close(t, "depth_med", got.DepthMed, c.Expect.DepthMed)
			close(t, "r2", got.R2, c.Expect.R2)
			close(t, "res_std_t", got.ResStdT, c.Expect.ResStdT)
			close(t, "curv_t", got.CurvT, c.Expect.CurvT)
			close(t, "jump_rate", got.JumpRate, c.Expect.JumpRate)
			close(t, "mid_last", got.MidLast, c.Expect.MidLast)

			passed, score, fails := GatesAndScore(got)
			if passed != c.Expect.Passed {
				t.Errorf("passed = %v, want %v (fails=%v)", passed, c.Expect.Passed, fails)
			}
			close(t, "score", score, c.Expect.Score)
			if FailsCSV(fails) != FailsCSV(c.Expect.Fails) {
				t.Errorf("fails = %q, want %q", FailsCSV(fails), FailsCSV(c.Expect.Fails))
			}
		})
	}
}

func close(t *testing.T, name string, got, want float64) {
	t.Helper()
	// Absolute tolerance on tiny values, relative on large ones — the canon's
	// float64 arithmetic reorders slightly across languages.
	d := math.Abs(got - want)
	if d <= tol {
		return
	}
	if scale := math.Abs(want); scale > 1 && d/scale <= tol {
		return
	}
	t.Errorf("%s = %.17g, want %.17g (diff %.3g)", name, got, want, d)
}

func toF(v interface{}) float64 {
	f, _ := v.(float64)
	return f
}
