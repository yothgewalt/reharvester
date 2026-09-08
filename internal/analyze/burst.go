package analyze

import "math"

// BurstScale is the rate multiplier of the elevated state, and BurstGamma the
// cost of entering it. Together they set how much evidence a term needs before
// a year counts as bursty rather than noisy.
const (
	BurstScale = 2.0
	BurstGamma = 1.0
)

// burst scores a term with Kleinberg's two-state automaton over the yearly
// document stream: a base state at the term's own corpus-wide rate and an
// elevated state at BurstScale times that rate, with a transition cost that
// stops the model flipping on a single noisy year. The returned weight is the
// total log-likelihood advantage the elevated state holds over the base state
// across the years it is entered, and is zero for a term that never bursts.
//
// This locates the moment a topic emerges, which prevalence lift alone cannot:
// lift compares two windows and is blind to when inside them the rise happened.
func (a *Analytics) burst(t *TermStats) float64 {
	n := len(a.Years)
	if n < 2 {
		return 0
	}
	var totalR, totalD float64
	for i := range n {
		totalR += float64(t.Years[i])
		totalD += float64(a.DocsByYear[i])
	}
	if totalD == 0 || totalR == 0 {
		return 0
	}
	p0 := totalR / totalD
	p1 := math.Min(0.999, p0*BurstScale)
	if p1 <= p0 {
		return 0
	}
	trans := BurstGamma * math.Log(float64(n))

	// cost[j][i] is the negative log-likelihood of year i under state j. The
	// binomial coefficient is common to both states, so it is dropped.
	cost := func(j, i int) float64 {
		p := p0
		if j == 1 {
			p = p1
		}
		r := float64(t.Years[i])
		d := float64(a.DocsByYear[i])
		if d == 0 {
			return 0
		}
		if r > d {
			r = d
		}
		return -(r*math.Log(p) + (d-r)*math.Log(1-p))
	}

	// Viterbi over the two states.
	var prev [2]float64
	path := make([][2]int, n)
	prev[0] = cost(0, 0)
	prev[1] = cost(1, 0) + trans // entering the elevated state costs from the start
	for i := 1; i < n; i++ {
		var cur [2]float64
		for j := range 2 {
			best, arg := math.Inf(1), 0
			for k := range 2 {
				c := prev[k]
				if k != j && j == 1 {
					c += trans // only entering the burst state is penalised
				}
				if c < best {
					best, arg = c, k
				}
			}
			cur[j] = best + cost(j, i)
			path[i][j] = arg
		}
		prev = cur
	}
	end := 0
	if prev[1] < prev[0] {
		end = 1
	}
	states := make([]int, n)
	states[n-1] = end
	for i := n - 1; i > 0; i-- {
		states[i-1] = path[i][states[i]]
	}
	var weight float64
	for i := range n {
		if states[i] == 1 {
			weight += cost(0, i) - cost(1, i)
		}
	}
	if weight < 0 {
		return 0
	}
	return weight
}
