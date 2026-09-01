// Package graph builds the semantic backbone that turns a ranked list into a
// neighbourhood the user can walk, and the community structure the analytic
// layer is defined over.
package graph

import (
	"runtime"
	"sort"
	"sync"

	"github.com/yothgewalt/reharvester/internal/index"
)

// Edge is one undirected backbone edge, stored once with A < B.
type Edge struct {
	A, B int32
	W    float32
}

// KNNOptions configures backbone construction. K is the neighbour count before
// the mutual filter; the paper uses 8.
type KNNOptions struct {
	K int
	// Approximate switches to inverted-index candidate pruning. The exact
	// backbone scales as n^2.00 and dominates total build cost past about
	// 10,000 papers; the pruned variant scales near-linearly at a real fidelity
	// cost, so it is right for navigation and retrieval seeding and wrong for
	// community-level analytics.
	Approximate bool
	// ProbeTerms is how many of a document's highest-weight terms are probed,
	// and MaxCandidates caps the pool before exact cosine scoring. The paper
	// uses 24 and 400.
	ProbeTerms    int
	MaxCandidates int
}

func DefaultKNNOptions() KNNOptions {
	return KNNOptions{K: 8, ProbeTerms: 24, MaxCandidates: 400}
}

type neighbour struct {
	doc int32
	sim float32
}

// BuildKNN returns the mutual k-nearest-neighbour graph over the TF-IDF space,
// edges weighted by cosine similarity. Mutual means an edge survives only when
// each endpoint is in the other's top-k, which is what keeps hub documents from
// wiring themselves to everything.
func BuildKNN(tf *index.TFIDF, opt KNNOptions) []Edge {
	if opt.K <= 0 {
		opt = DefaultKNNOptions()
	}
	n := tf.N
	if n == 0 {
		return nil
	}
	top := make([][]neighbour, n)

	var order []int32
	if opt.Approximate {
		order = termsByRarity(tf)
	}

	workers := runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	rows := make(chan int, workers*4)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]float32, n)
			for i := range rows {
				if opt.Approximate {
					top[i] = approxTopK(tf, i, opt, order, buf)
				} else {
					tf.CosineRow(i, buf)
					top[i] = exactTopK(buf, opt.K)
				}
			}
		}()
	}
	for i := range n {
		rows <- i
	}
	close(rows)
	wg.Wait()

	return mutualEdges(top, opt.K)
}

func exactTopK(sims []float32, k int) []neighbour {
	out := make([]neighbour, 0, k)
	var worst float32
	for j, s := range sims {
		if s <= 0 {
			continue
		}
		if len(out) < k {
			out = append(out, neighbour{int32(j), s})
			if len(out) == k {
				sort.Slice(out, func(a, b int) bool { return out[a].sim < out[b].sim })
				worst = out[0].sim
			}
			continue
		}
		if s <= worst {
			continue
		}
		out[0] = neighbour{int32(j), s}
		sort.Slice(out, func(a, b int) bool { return out[a].sim < out[b].sim })
		worst = out[0].sim
	}
	sort.Slice(out, func(a, b int) bool { return out[a].sim > out[b].sim })
	return out
}

// termsByRarity orders term ids by ascending document frequency, so probing
// stops early on the terms that discriminate most.
func termsByRarity(tf *index.TFIDF) []int32 {
	rank := make([]int32, len(tf.DF))
	for i := range rank {
		rank[i] = int32(i)
	}
	sort.Slice(rank, func(a, b int) bool { return tf.DF[rank[a]] < tf.DF[rank[b]] })
	pos := make([]int32, len(tf.DF))
	for r, id := range rank {
		pos[id] = int32(r)
	}
	return pos
}

// approxTopK probes only the document's heaviest terms, rarest postings first,
// and caps the candidate pool before scoring it exactly. It needs no external
// ANN library, which is what preserves the local-first property.
func approxTopK(tf *index.TFIDF, i int, opt KNNOptions, rarity []int32, _ []float32) []neighbour {
	ids, ws := tf.Row(i)
	if len(ids) == 0 {
		return nil
	}
	type tw struct {
		id int32
		w  float32
	}
	terms := make([]tw, len(ids))
	for j := range ids {
		terms[j] = tw{ids[j], ws[j]}
	}
	sort.Slice(terms, func(a, b int) bool { return terms[a].w > terms[b].w })
	if len(terms) > opt.ProbeTerms {
		terms = terms[:opt.ProbeTerms]
	}
	sort.Slice(terms, func(a, b int) bool { return rarity[terms[a].id] < rarity[terms[b].id] })

	cands := make([]int32, 0, opt.MaxCandidates)
	seen := make(map[int32]struct{}, opt.MaxCandidates)
	for _, t := range terms {
		for _, p := range tf.Postings[t.id] {
			if p.Doc == int32(i) {
				continue
			}
			if _, dup := seen[p.Doc]; dup {
				continue
			}
			seen[p.Doc] = struct{}{}
			cands = append(cands, p.Doc)
			if len(cands) >= opt.MaxCandidates {
				break
			}
		}
		if len(cands) >= opt.MaxCandidates {
			break
		}
	}
	if len(cands) == 0 {
		return nil
	}
	// Score the pool exactly, walking each candidate's own row against this
	// document's terms. Accumulating over the probed terms' full postings
	// instead would touch every document they occur in, which is the cost the
	// pruning exists to avoid.
	q := make(map[int32]float32, len(ids))
	for j, id := range ids {
		q[id] = ws[j]
	}
	out := make([]neighbour, 0, len(cands))
	for _, c := range cands {
		cids, cws := tf.Row(int(c))
		var s float32
		for j, id := range cids {
			if qw, ok := q[id]; ok {
				s += qw * cws[j]
			}
		}
		if s > 0 {
			out = append(out, neighbour{c, s})
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].sim > out[b].sim })
	if len(out) > opt.K {
		out = out[:opt.K]
	}
	return out
}

// mutualEdges keeps a pair only when each endpoint is in the other's top-k.
func mutualEdges(top [][]neighbour, k int) []Edge {
	inTop := make([]map[int32]float32, len(top))
	for i, ns := range top {
		m := make(map[int32]float32, len(ns))
		for _, nb := range ns {
			m[nb.doc] = nb.sim
		}
		inTop[i] = m
	}
	var edges []Edge
	for i, ns := range top {
		for _, nb := range ns {
			j := int(nb.doc)
			if j <= i {
				continue // emit each pair once, from the lower endpoint
			}
			if _, mutual := inTop[j][int32(i)]; !mutual {
				continue
			}
			edges = append(edges, Edge{A: int32(i), B: nb.doc, W: nb.sim})
		}
	}
	sort.Slice(edges, func(a, b int) bool {
		if edges[a].A != edges[b].A {
			return edges[a].A < edges[b].A
		}
		return edges[a].B < edges[b].B
	})
	return edges
}
