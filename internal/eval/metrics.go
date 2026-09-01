// Package eval reproduces the paper's measurements: retrieval on two tasks with
// independent ground truth, context quality at a fixed budget, the capability
// ladder, and scaling.
package eval

import (
	"math"
	"math/rand/v2"
	"sort"
)

// Ranked is one query's result list, as corpus positions in rank order.
type Ranked []int

// Judgments maps a document position to its graded relevance, 0 meaning
// irrelevant.
type Judgments map[int]int

func PrecisionAt(r Ranked, j Judgments, k, minGrade int) float64 {
	if k <= 0 {
		return 0
	}
	hits := 0
	for i, d := range r {
		if i >= k {
			break
		}
		if j[d] >= minGrade && j[d] > 0 {
			hits++
		}
	}
	return float64(hits) / float64(k)
}

func RecallAt(r Ranked, j Judgments, k, minGrade int) float64 {
	total := 0
	for _, g := range j {
		if g >= minGrade && g > 0 {
			total++
		}
	}
	if total == 0 {
		return 0
	}
	hits := 0
	for i, d := range r {
		if i >= k {
			break
		}
		if j[d] >= minGrade && j[d] > 0 {
			hits++
		}
	}
	return float64(hits) / float64(total)
}

// ReciprocalRank is 1/rank of the first relevant document, or 0.
func ReciprocalRank(r Ranked, j Judgments) float64 {
	for i, d := range r {
		if j[d] > 0 {
			return 1 / float64(i+1)
		}
	}
	return 0
}

// NDCGAt uses the standard 2^g - 1 gain with log2 discount. The ideal ranking
// comes from the pooled judgments, so no system is scored against a reference
// derived from its own output.
func NDCGAt(r Ranked, j Judgments, k int) float64 {
	dcg := 0.0
	for i, d := range r {
		if i >= k {
			break
		}
		if g := j[d]; g > 0 {
			dcg += (math.Pow(2, float64(g)) - 1) / math.Log2(float64(i+2))
		}
	}
	grades := make([]int, 0, len(j))
	for _, g := range j {
		if g > 0 {
			grades = append(grades, g)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(grades)))
	idcg := 0.0
	for i, g := range grades {
		if i >= k {
			break
		}
		idcg += (math.Pow(2, float64(g)) - 1) / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// CI is a paired-bootstrap difference with its confidence interval.
type CI struct {
	Diff        float64 `json:"diff"`
	Low         float64 `json:"low"`
	High        float64 `json:"high"`
	Significant bool    `json:"significant"`
}

// BootstrapResamples is the paper's resample count.
const BootstrapResamples = 5000

// PairedBootstrap estimates the difference in means between two systems scored
// on the SAME queries, resampling queries with replacement. Pairing matters:
// per-query scores are highly correlated across systems, so an unpaired test
// would be far too conservative.
func PairedBootstrap(a, b []float64, resamples int) CI {
	n := min(len(a), len(b))
	if n == 0 {
		return CI{}
	}
	if resamples <= 0 {
		resamples = BootstrapResamples
	}
	diffs := make([]float64, n)
	for i := range n {
		diffs[i] = a[i] - b[i]
	}
	observed := Mean(diffs)

	rng := rand.New(rand.NewPCG(13, 17)) // fixed seed: a repeatable measurement
	samples := make([]float64, resamples)
	for s := range resamples {
		var sum float64
		for range n {
			sum += diffs[rng.IntN(n)]
		}
		samples[s] = sum / float64(n)
	}
	sort.Float64s(samples)
	lo := samples[int(0.025*float64(resamples))]
	hi := samples[min(int(0.975*float64(resamples)), resamples-1)]
	return CI{Diff: observed, Low: lo, High: hi, Significant: (lo > 0) == (hi > 0) && lo != 0}
}

// LogLogSlope fits log(y) = a + b*log(x) by least squares and returns b, the
// scaling exponent.
func LogLogSlope(xs, ys []float64) float64 {
	n := min(len(xs), len(ys))
	if n < 2 {
		return 0
	}
	var sx, sy, sxx, sxy float64
	used := 0
	for i := range n {
		if xs[i] <= 0 || ys[i] <= 0 {
			continue
		}
		lx, ly := math.Log(xs[i]), math.Log(ys[i])
		sx += lx
		sy += ly
		sxx += lx * lx
		sxy += lx * ly
		used++
	}
	if used < 2 {
		return 0
	}
	fn := float64(used)
	den := fn*sxx - sx*sx
	if den == 0 {
		return 0
	}
	return (fn*sxy - sx*sy) / den
}
