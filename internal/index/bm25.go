package index

import "math"

// BM25 is the Okapi ranker at the reference parameters. It shares the corpus
// tokenisation with the other tiers but is unigram-only: the paper finds it
// clearly better than TF-IDF on short known-item queries and no better on a
// task whose query is a 200-word abstract, and that contrast is the point.
type BM25 struct {
	Postings map[string][]Posting // W carries the raw term frequency
	DocLen   []float64
	AvgDL    float64
	IDF      map[string]float64
	N        int
}

const (
	bm25K1 = 1.5
	bm25B  = 0.75
)

func BuildBM25(docs []string) *BM25 {
	n := len(docs)
	ix := &BM25{
		Postings: make(map[string][]Posting),
		DocLen:   make([]float64, n),
		IDF:      make(map[string]float64),
		N:        n,
	}
	var total float64
	for i, d := range docs {
		toks := Unigrams(d)
		ix.DocLen[i] = float64(len(toks))
		total += float64(len(toks))
		tf := make(map[string]int, len(toks))
		for _, t := range toks {
			tf[t]++
		}
		for t, c := range tf {
			ix.Postings[t] = append(ix.Postings[t], Posting{Doc: int32(i), W: float32(c)})
		}
	}
	if n > 0 {
		ix.AvgDL = total / float64(n)
	}
	for t, ps := range ix.Postings {
		df := float64(len(ps))
		// Robertson/Sparck Jones IDF with the standard +0.5 smoothing, floored
		// at zero so a term in nearly every document cannot score negatively.
		v := math.Log((float64(n)-df+0.5)/(df+0.5) + 1)
		ix.IDF[t] = v
	}
	return ix
}

func (ix *BM25) Search(text string, k int) []Hit {
	scores := make([]float64, ix.N)
	touched := make([]int32, 0, 1024)
	seen := make([]bool, ix.N)
	for _, t := range Unigrams(text) {
		ps, ok := ix.Postings[t]
		if !ok {
			continue
		}
		idf := ix.IDF[t]
		for _, p := range ps {
			tf := float64(p.W)
			norm := tf + bm25K1*(1-bm25B+bm25B*ix.DocLen[p.Doc]/ix.AvgDL)
			scores[p.Doc] += idf * tf * (bm25K1 + 1) / norm
			if !seen[p.Doc] {
				seen[p.Doc] = true
				touched = append(touched, p.Doc)
			}
		}
	}
	return topK(scores, touched, k)
}
