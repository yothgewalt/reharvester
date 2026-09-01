// Package retrieve turns the indexes into ranked results, and encodes the
// capability ladder: which tiers exist is a function of what was actually built,
// not of what failed at runtime.
package retrieve

import (
	"sort"

	"github.com/yothgewalt/reharvester/internal/index"
)

// Tier names the rungs of the capability ladder. Each is a complete system and
// each adds exactly one dependency to the one below it.
type Tier string

const (
	T0 Tier = "T0" // inverted index only — nothing beyond the standard library
	T1 Tier = "T1" // + TF-IDF vectoriser
	T2 Tier = "T2" // + k-NN backbone: navigation, communities, trend analytics
	T3 Tier = "T3" // + neural sentence encoder
)

// Method selects a ranking strategy. The eval compares all of them; the server
// picks one from the tier.
type Method string

const (
	Overlap     Method = "t0"           // boolean candidates, token-overlap ranking
	TFIDF       Method = "tfidf"        //
	BM25        Method = "bm25"         //
	Dense       Method = "dense"        //
	Hybrid      Method = "hybrid"       // reciprocal rank fusion of the three
	HybridGraph Method = "hybrid+graph" // fusion, then one-hop backbone expansion
)

// rrfK is the reciprocal-rank-fusion constant. 60 is the value from the
// original formulation and the one the paper's hybrid rows use.
const rrfK = 60.0

// Neighbours supplies the backbone adjacency used by graph expansion. The
// engine takes it as an interface so retrieval does not depend on the graph
// package, and so a T0/T1 engine can simply hold nil.
type Neighbours interface {
	Neighbors(doc int) []int
}

type Engine struct {
	Inv   *index.Inverted
	TF    *index.TFIDF
	BM    *index.BM25
	Dense *index.Dense
	Graph Neighbours
}

// Tier reports the highest rung this engine actually has the parts for.
func (e *Engine) Tier() Tier {
	switch {
	case e.Dense != nil && e.Dense.Ready():
		return T3
	case e.Graph != nil:
		return T2
	case e.TF != nil:
		return T1
	default:
		return T0
	}
}

// DefaultMethod is the ranking a tier uses when the caller does not ask for
// one. T3 defaults to dense alone rather than the full hybrid: the paper
// measures dense within 0.005 nDCG of hybrid at roughly 1/200 of the latency,
// so hybrid is not the right default even where it is available. T2 defaults to
// the same ranking as T1 because the backbone measurably does not change
// known-item accuracy — it exists for navigation and analytics.
func DefaultMethod(t Tier) Method {
	switch t {
	case T3:
		return Dense
	case T2, T1:
		return TFIDF
	default:
		return Overlap
	}
}

// Search ranks the corpus for a query under the given method, falling back down
// the ladder when a method's index is absent.
func (e *Engine) Search(method Method, q string, k int) []index.Hit {
	switch method {
	case Overlap:
		if e.Inv != nil {
			return e.Inv.Search(q, k)
		}
	case TFIDF:
		if e.TF != nil {
			return e.TF.Search(q, k)
		}
		return e.Search(Overlap, q, k)
	case BM25:
		if e.BM != nil {
			return e.BM.Search(q, k)
		}
		return e.Search(TFIDF, q, k)
	case Dense:
		if e.Dense != nil && e.Dense.Ready() {
			return e.Dense.Search(q, k)
		}
		return e.Search(TFIDF, q, k)
	case Hybrid:
		return e.fuse(q, k)
	case HybridGraph:
		return e.expand(e.fuse(q, k*2), k)
	}
	if e.Inv != nil {
		return e.Inv.Search(q, k)
	}
	return nil
}

// fuse combines whichever rankers exist by reciprocal rank fusion. Fusing a
// single available ranker is just that ranker, so this degrades cleanly.
func (e *Engine) fuse(q string, k int) []index.Hit {
	depth := k * 5
	if depth < 100 {
		depth = 100
	}
	var lists [][]index.Hit
	if e.TF != nil {
		lists = append(lists, e.TF.Search(q, depth))
	}
	if e.BM != nil {
		lists = append(lists, e.BM.Search(q, depth))
	}
	if e.Dense != nil && e.Dense.Ready() {
		lists = append(lists, e.Dense.Search(q, depth))
	}
	if len(lists) == 0 {
		return e.Search(Overlap, q, k)
	}
	score := make(map[int]float64)
	for _, l := range lists {
		for rank, h := range l {
			score[h.Doc] += 1 / (rrfK + float64(rank+1))
		}
	}
	return rank(score, k)
}

// expand adds the backbone neighbours of the top seeds by spreading activation:
// a neighbour inherits a DISCOUNTED COPY of its best seed's fusion score, and a
// document's final score is the maximum of what it earned directly and what it
// inherited. Never a sum.
//
// Both properties are load-bearing, and each was measured here. Summing the
// bonuses lets a hub adjacent to many seeds outscore the document the retriever
// actually found (known-item P@1 fell 0.90 -> 0.33). Adding even a bounded bonus
// to a document's own score lets a weak seed that neighbours a strong one
// leapfrog the strong one (P@1 0.68). Taking the maximum with a discount below 1
// makes the top hit unbeatable by its own neighbourhood, while still letting a
// neighbour of the best hit enter the top ten ahead of a marginal direct match —
// which is the small, real gain the paper measures.
func (e *Engine) expand(seeds []index.Hit, k int) []index.Hit {
	if e.Graph == nil || len(seeds) == 0 {
		return trim(seeds, k)
	}
	// expansionDecay < 1 is what guarantees an inherited score always loses to
	// the seed it was inherited from.
	const expansionDecay = 0.9
	const seedDepth = 10

	score := make(map[int]float64, len(seeds)*4)
	for rank, h := range seeds {
		score[h.Doc] = 1 / (rrfK + float64(rank+1))
	}
	inherited := make(map[int]float64)
	for i, h := range seeds {
		if i >= seedDepth {
			break
		}
		w := expansionDecay * score[h.Doc]
		for _, nb := range e.Graph.Neighbors(h.Doc) {
			if w > inherited[nb] {
				inherited[nb] = w
			}
		}
	}
	for doc, w := range inherited {
		if w > score[doc] {
			score[doc] = w
		}
	}
	return rank(score, k)
}

func rank(score map[int]float64, k int) []index.Hit {
	hits := make([]index.Hit, 0, len(score))
	for d, s := range score {
		hits = append(hits, index.Hit{Doc: d, Score: s})
	}
	sort.Slice(hits, func(a, b int) bool {
		if hits[a].Score != hits[b].Score {
			return hits[a].Score > hits[b].Score
		}
		return hits[a].Doc < hits[b].Doc
	})
	return trim(hits, k)
}

func trim(h []index.Hit, k int) []index.Hit {
	if k > 0 && len(h) > k {
		return h[:k]
	}
	return h
}
