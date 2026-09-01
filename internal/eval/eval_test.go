package eval

import (
	"math"
	"testing"

	"github.com/yothgewalt/reharvester/internal/paper"
)

func TestNDCGAt(t *testing.T) {
	j := Judgments{10: 3, 11: 2, 12: 1}
	// The ideal ordering scores 1 by definition.
	if got := NDCGAt(Ranked{10, 11, 12}, j, 10); math.Abs(got-1) > 1e-9 {
		t.Errorf("ideal ranking nDCG = %v, want 1", got)
	}
	// Reversing it must score strictly less, but above zero.
	rev := NDCGAt(Ranked{12, 11, 10}, j, 10)
	if rev >= 1 || rev <= 0 {
		t.Errorf("reversed nDCG = %v, want strictly between 0 and 1", rev)
	}
	// Retrieving nothing relevant scores zero.
	if got := NDCGAt(Ranked{99, 98}, j, 10); got != 0 {
		t.Errorf("irrelevant ranking nDCG = %v, want 0", got)
	}
	// The cutoff must bind: a relevant hit past k contributes nothing.
	if got := NDCGAt(Ranked{99, 10}, j, 1); got != 0 {
		t.Errorf("hit beyond k counted: %v", got)
	}
}

func TestPrecisionRecallAndMRR(t *testing.T) {
	j := Judgments{1: 3, 2: 2, 3: 1, 4: 0}
	r := Ranked{4, 1, 2, 3}
	if got := PrecisionAt(r, j, 4, 1); math.Abs(got-0.75) > 1e-9 {
		t.Errorf("P@4 = %v, want 0.75", got)
	}
	// P@10 with minGrade 2 counts only the high-relevance hits.
	if got := PrecisionAt(r, j, 4, 2); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("P@4(grade>=2) = %v, want 0.5", got)
	}
	if got := RecallAt(r, j, 2, 1); math.Abs(got-1.0/3.0) > 1e-9 {
		t.Errorf("R@2 = %v, want 1/3", got)
	}
	if got := ReciprocalRank(r, j); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("MRR = %v, want 0.5 (first relevant at rank 2)", got)
	}
	if got := ReciprocalRank(Ranked{4}, j); got != 0 {
		t.Errorf("MRR with no relevant hit = %v, want 0", got)
	}
}

// The paper's grading ladder over arXiv's own category labels.
func TestGradeTaskB(t *testing.T) {
	mk := func(cats ...string) paper.Paper { return paper.Paper{Categories: cats} }
	for _, tc := range []struct {
		name string
		a, b paper.Paper
		want int
	}{
		{"same primary plus a shared secondary", mk("cs.IR", "cs.CL"), mk("cs.IR", "cs.CL"), 3},
		{"same primary only", mk("cs.IR", "cs.CL"), mk("cs.IR", "cs.DB"), 2},
		{"two shared secondaries, different primary", mk("cs.DL", "cs.IR", "cs.CL"), mk("cs.SI", "cs.IR", "cs.CL"), 1},
		{"one shared label is not relevant", mk("cs.DL", "cs.IR"), mk("cs.SI", "cs.IR"), 0},
		{"nothing shared", mk("cs.DL"), mk("cs.DB"), 0},
		{"empty categories", mk(), mk("cs.IR"), 0},
	} {
		if got := GradeTaskB(tc.a, tc.b); got != tc.want {
			t.Errorf("%s: grade = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestPairedBootstrap(t *testing.T) {
	// A consistent per-query advantage must come out significant.
	a := make([]float64, 200)
	b := make([]float64, 200)
	for i := range a {
		a[i] = 0.5 + float64(i%7)*0.01
		b[i] = a[i] - 0.05
	}
	ci := PairedBootstrap(a, b, 2000)
	if math.Abs(ci.Diff-0.05) > 1e-9 {
		t.Errorf("diff = %v, want 0.05", ci.Diff)
	}
	if !ci.Significant || ci.Low <= 0 {
		t.Errorf("a consistent advantage should be significant, got %+v", ci)
	}
	// Identical systems must not be.
	if ci := PairedBootstrap(a, a, 2000); ci.Significant || ci.Diff != 0 {
		t.Errorf("identical systems reported a difference: %+v", ci)
	}
}

// The scaling exponents in the paper come from this fit.
func TestLogLogSlope(t *testing.T) {
	xs := []float64{2000, 5000, 10000, 20000, 34397}
	quad := make([]float64, len(xs))
	lin := make([]float64, len(xs))
	for i, x := range xs {
		quad[i] = x * x * 1e-9
		lin[i] = x * 1e-4
	}
	if got := LogLogSlope(xs, quad); math.Abs(got-2) > 1e-6 {
		t.Errorf("quadratic exponent = %v, want 2", got)
	}
	if got := LogLogSlope(xs, lin); math.Abs(got-1) > 1e-6 {
		t.Errorf("linear exponent = %v, want 1", got)
	}
}

// Pooling must never judge the query's own source document: retrieving it is
// trivially correct and would inflate every system equally.
func TestPoolExcludesSeed(t *testing.T) {
	papers := []paper.Paper{mkCat("cs.IR", "cs.CL"), mkCat("cs.IR", "cs.CL"), mkCat("cs.DB")}
	q := Query{Seed: 0, Judge: Judgments{}}
	Pool(&q, papers, []Ranked{{0, 1, 2}}, 50)
	if _, ok := q.Judge[0]; ok {
		t.Error("the seed document was judged")
	}
	if q.Judge[1] != 3 || q.Judge[2] != 0 {
		t.Errorf("pooled judgments = %v", q.Judge)
	}
}

func mkCat(cats ...string) paper.Paper { return paper.Paper{Categories: cats} }
