// Package analyze answers the two questions a ranked list cannot: what
// vocabulary is emerging, and where are the gaps.
package analyze

import (
	"math"
	"sort"

	"github.com/yothgewalt/reharvester/internal/index"
	"github.com/yothgewalt/reharvester/internal/paper"
)

// TermStats is one term's document counts, sliced by year and by community.
// Counting documents rather than occurrences is what makes prevalence
// comparable across windows of different size.
//
// Years is aligned to Analytics.Years and Comms to community ids 0..NumComms-1,
// so a window recomputation is a sum over a slice rather than a re-scan of the
// corpus. That is what keeps GET /trends/recalculate inside its budget.
type TermStats struct {
	Term  string  `json:"t"`
	Years []int32 `json:"y"`
	Comms []int32 `json:"c"`
	Total int32   `json:"n"`
}

// Analytics is the persisted term table. It is built once per corpus; every
// trend query is an aggregation over it.
type Analytics struct {
	Years      []int       `json:"years"`
	DocsByYear []int32     `json:"docsByYear"`
	NumComms   int         `json:"numComms"`
	Terms      []TermStats `json:"terms"`
}

// MinTermDocs is the floor on corpus-wide document frequency for a term to be
// tracked. Below it a "trend" is a handful of papers and the smoothing
// dominates the signal.
func MinTermDocs(n int) int {
	m := n / 500
	if m < 5 {
		m = 5
	}
	return m
}

// BuildAnalytics extracts n-grams up to maxN and tabulates them by year and
// community. community may be nil, in which case specificity is unavailable and
// every term is treated as maximally specific.
func BuildAnalytics(papers []paper.Paper, community []int32, numComms, maxN int) *Analytics {
	yearSet := map[int]struct{}{}
	for _, p := range papers {
		yearSet[p.Year()] = struct{}{}
	}
	years := make([]int, 0, len(yearSet))
	for y := range yearSet {
		years = append(years, y)
	}
	sort.Ints(years)
	yearIdx := make(map[int]int, len(years))
	for i, y := range years {
		yearIdx[y] = i
	}
	if numComms < 1 {
		numComms = 1
	}

	a := &Analytics{Years: years, DocsByYear: make([]int32, len(years)), NumComms: numComms}
	type acc struct {
		years []int32
		comms []int32
		total int32
	}
	table := make(map[string]*acc)
	for i, p := range papers {
		yi := yearIdx[p.Year()]
		a.DocsByYear[yi]++
		// An unclustered document (ci < 0) contributes to the year counts but to
		// no community bucket. Folding it into community 0 would inflate that
		// community's share of every term and skew the specificity filter.
		ci := -1
		if community != nil && i < len(community) && community[i] >= 0 && int(community[i]) < numComms {
			ci = int(community[i])
		}
		// Distinct terms per document: this counts documents, not occurrences.
		seen := make(map[string]struct{})
		for _, t := range index.Terms(p.Abstract, maxN) {
			if _, dup := seen[t]; dup {
				continue
			}
			seen[t] = struct{}{}
			e := table[t]
			if e == nil {
				e = &acc{years: make([]int32, len(years)), comms: make([]int32, numComms)}
				table[t] = e
			}
			e.years[yi]++
			if ci >= 0 {
				e.comms[ci]++
			}
			e.total++
		}
	}
	min := int32(MinTermDocs(len(papers)))
	for t, e := range table {
		if e.total < min {
			continue
		}
		a.Terms = append(a.Terms, TermStats{Term: t, Years: e.years, Comms: e.comms, Total: e.total})
	}
	sort.Slice(a.Terms, func(i, j int) bool { return a.Terms[i].Term < a.Terms[j].Term })
	return a
}

// Alpha is the pseudo-document count used to smooth the prevalence ratio. An
// unsmoothed log-ratio is infinite for any term absent from the early window —
// exactly the interesting case — so the smoothing is what makes a genuinely new
// phrase rankable at all.
const Alpha = 0.5

// SpecificityMax is the retained ceiling on a term's normalised entropy over
// communities. A term spread evenly over every community approaches 1; a
// topical term concentrates. The paper reports a median unigram specificity of
// 0.778, so 0.80 is a mild filter that still removes the worst discourse
// vocabulary.
const SpecificityMax = 0.80

// Trend is one term's movement between two windows.
type Trend struct {
	Term        string  `json:"term"`
	EarlyDocs   int     `json:"earlyDocs"`
	LateDocs    int     `json:"lateDocs"`
	Log2Lift    float64 `json:"log2Lift"`
	Specificity float64 `json:"specificity"`
	Burst       float64 `json:"burst"`
	Prevalence  float64 `json:"prevalence"` // share of late-window documents
}

// Window is an inclusive year range.
type Window struct{ From, To int }

func (w Window) contains(y int) bool { return y >= w.From && y <= w.To }

// Specificity is the normalised entropy of a term's distribution over
// communities: S(t) = -sum p(c|t) log p(c|t) / log |C|.
func Specificity(comms []int32) float64 {
	var total float64
	nonzero := 0
	for _, c := range comms {
		if c > 0 {
			total += float64(c)
			nonzero++
		}
	}
	if total == 0 || len(comms) < 2 {
		return 0
	}
	if nonzero <= 1 {
		return 0
	}
	var h float64
	for _, c := range comms {
		if c <= 0 {
			continue
		}
		p := float64(c) / total
		h -= p * math.Log(p)
	}
	return h / math.Log(float64(len(comms)))
}

// Trends compares prevalence between two windows, keeping only terms that pass
// the specificity filter. Raw document counts travel with every row so a reader
// can tell a genuinely new phrase from a merely growing one.
func (a *Analytics) Trends(early, late Window) []Trend {
	var earlyDocs, lateDocs float64
	for i, y := range a.Years {
		if early.contains(y) {
			earlyDocs += float64(a.DocsByYear[i])
		}
		if late.contains(y) {
			lateDocs += float64(a.DocsByYear[i])
		}
	}
	if earlyDocs == 0 || lateDocs == 0 {
		return nil
	}
	out := make([]Trend, 0, len(a.Terms))
	for i := range a.Terms {
		t := &a.Terms[i]
		var e, l float64
		for j, y := range a.Years {
			if early.contains(y) {
				e += float64(t.Years[j])
			}
			if late.contains(y) {
				l += float64(t.Years[j])
			}
		}
		if e == 0 && l == 0 {
			continue
		}
		spec := Specificity(t.Comms)
		if spec > SpecificityMax {
			continue
		}
		lift := ((l + Alpha) / lateDocs) / ((e + Alpha) / earlyDocs)
		out = append(out, Trend{
			Term:        t.Term,
			EarlyDocs:   int(e),
			LateDocs:    int(l),
			Log2Lift:    math.Log2(lift),
			Specificity: spec,
			Burst:       a.burst(t),
			Prevalence:  l / lateDocs,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Log2Lift != out[j].Log2Lift {
			return out[i].Log2Lift > out[j].Log2Lift
		}
		return out[i].Term < out[j].Term
	})
	return out
}

// Prevalence returns a term's per-year share of documents, for the trajectory
// view that makes substitution legible rather than only net growth.
func (a *Analytics) Prevalence(term string) map[int]float64 {
	for i := range a.Terms {
		if a.Terms[i].Term != term {
			continue
		}
		out := make(map[int]float64, len(a.Years))
		for j, y := range a.Years {
			if a.DocsByYear[j] > 0 {
				out[y] = float64(a.Terms[i].Years[j]) / float64(a.DocsByYear[j])
			}
		}
		return out
	}
	return nil
}
