package analyze

import (
	"math"
	"math/rand/v2"
	"sort"

	"github.com/yothgewalt/reharvester/internal/graph"
)

// DefaultPermutations is how many times community labels are shuffled to build
// the null. The paper uses 200.
const DefaultPermutations = 200

// GapZMax is the z-score ceiling for a pair to count as under-connected.
const GapZMax = -2.0

// GapPair is two communities whose documents read alike but between which few
// links exist.
type GapPair struct {
	A          int     `json:"a"`
	B          int     `json:"b"`
	LabelA     string  `json:"labelA"`
	LabelB     string  `json:"labelB"`
	Similarity float64 `json:"similarity"`
	Observed   int     `json:"observed"`
	NullMean   float64 `json:"nullMean"`
	NullStd    float64 `json:"nullStd"`
	Z          float64 `json:"z"`
}

// GapReport carries the ranking together with the statistic that qualifies it.
type GapReport struct {
	Pairs []GapPair `json:"pairs"`
	// BelowThreshold and Total record how many of all pairs clear GapZMax.
	// When almost all of them do, the z-score is a weak filter and the ranking
	// is carried by centroid similarity — which is the honest reading of this
	// analysis, not a defect to hide.
	BelowThreshold int `json:"belowThreshold"`
	Total          int `json:"total"`
}

// Gaps scores every community pair against a label-permutation null.
//
// Scoring under-connection as raw cross-edge density conflates a real gap with
// the fact that small communities have few edges, so the observed count is
// compared against a null built by permuting community labels over the fixed
// graph — which preserves both the degree sequence and the community sizes.
//
// The null is deliberately weak and the report says so: permuting labels
// destroys all topical structure, so nearly every real pair looks
// under-connected. This is a shortlist generator, not a hypothesis test.
func Gaps(g *graph.Graph, perms int) GapReport {
	if perms <= 0 {
		perms = DefaultPermutations
	}
	nc := len(g.Communities)
	if nc < 2 {
		return GapReport{}
	}
	// Community ids are dense indices into g.Communities, and -1 means the
	// document was left unclustered.
	comm := make([]int, g.N)
	for i, cid := range g.Community {
		if cid >= 0 && int(cid) < nc {
			comm[i] = int(cid)
		} else {
			comm[i] = -1
		}
	}

	idx := func(a, b int) int {
		if a > b {
			a, b = b, a
		}
		return a*nc + b
	}
	observed := make([]int, nc*nc)
	for _, e := range g.Edges {
		a, b := comm[e.A], comm[e.B]
		if a < 0 || b < 0 || a == b {
			continue
		}
		observed[idx(a, b)]++
	}

	// The permuted assignment preserves community sizes by shuffling the label
	// vector itself rather than resampling it.
	labels := make([]int, 0, g.N)
	for _, c := range comm {
		if c >= 0 {
			labels = append(labels, c)
		}
	}
	nodes := make([]int32, 0, g.N)
	for i, c := range comm {
		if c >= 0 {
			nodes = append(nodes, int32(i))
		}
	}
	sum := make([]float64, nc*nc)
	sumsq := make([]float64, nc*nc)
	shuffled := make([]int, len(labels))
	perm := make([]int, g.N)
	rng := rand.New(rand.NewPCG(7, 11)) // fixed seed: a repeatable search
	counts := make([]int, nc*nc)
	for range perms {
		copy(shuffled, labels)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		for i := range perm {
			perm[i] = -1
		}
		for i, nd := range nodes {
			perm[nd] = shuffled[i]
		}
		for i := range counts {
			counts[i] = 0
		}
		for _, e := range g.Edges {
			a, b := perm[e.A], perm[e.B]
			if a < 0 || b < 0 || a == b {
				continue
			}
			counts[idx(a, b)]++
		}
		for i, c := range counts {
			sum[i] += float64(c)
			sumsq[i] += float64(c) * float64(c)
		}
	}

	rep := GapReport{}
	n := float64(perms)
	for a := range nc {
		for b := a + 1; b < nc; b++ {
			k := idx(a, b)
			mean := sum[k] / n
			variance := sumsq[k]/n - mean*mean
			if variance < 0 {
				variance = 0
			}
			std := math.Sqrt(variance)
			z := 0.0
			if std > 0 {
				z = (float64(observed[k]) - mean) / std
			}
			rep.Total++
			if z < GapZMax {
				rep.BelowThreshold++
			}
			rep.Pairs = append(rep.Pairs, GapPair{
				A: a, B: b,
				LabelA:     g.Communities[a].Label,
				LabelB:     g.Communities[b].Label,
				Similarity: cosineMaps(g.Communities[a].Centroid, g.Communities[b].Centroid),
				Observed:   observed[k],
				NullMean:   mean,
				NullStd:    std,
				Z:          z,
			})
		}
	}
	// Candidates are the pairs that clear the null, ranked by how alike they
	// read. Similarity carries the ranking; the z-score only qualifies.
	keep := rep.Pairs[:0]
	for _, p := range rep.Pairs {
		if p.Z < GapZMax {
			keep = append(keep, p)
		}
	}
	rep.Pairs = keep
	sort.Slice(rep.Pairs, func(i, j int) bool {
		return rep.Pairs[i].Similarity > rep.Pairs[j].Similarity
	})
	return rep
}

// GapNodes returns the documents that sit in the gap: members of a top-ranked
// under-connected pair whose own edge weight most crosses a community boundary.
// The UI wants node ids, while the analysis produces community pairs, and the
// bridge score is what maps one to the other.
func GapNodes(g *graph.Graph, rep GapReport, topPairs, limit int) []int32 {
	if len(rep.Pairs) == 0 {
		return nil
	}
	if topPairs > len(rep.Pairs) {
		topPairs = len(rep.Pairs)
	}
	inPair := make(map[int]struct{})
	for _, p := range rep.Pairs[:topPairs] {
		inPair[p.A] = struct{}{}
		inPair[p.B] = struct{}{}
	}
	type nb struct {
		id int32
		b  float64
	}
	var cands []nb
	for i := range g.N {
		if _, ok := inPair[int(g.Community[i])]; !ok {
			continue
		}
		if g.Bridge[i] > 0 {
			cands = append(cands, nb{int32(i), g.Bridge[i]})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].b != cands[j].b {
			return cands[i].b > cands[j].b
		}
		return cands[i].id < cands[j].id
	})
	if limit > 0 && len(cands) > limit {
		cands = cands[:limit]
	}
	out := make([]int32, len(cands))
	for i, c := range cands {
		out[i] = c.id
	}
	return out
}

func cosineMaps(a, b map[int32]float32) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	var dot, na, nb float64
	for id, av := range a {
		dot += float64(av) * float64(b[id])
		na += float64(av) * float64(av)
	}
	for _, bv := range b {
		nb += float64(bv) * float64(bv)
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
