package index

import (
	"math"
	"sort"
	"strings"
)

// Hit is one scored document, by corpus position.
type Hit struct {
	Doc   int
	Score float64
}

// Posting is one document's weight for a term.
type Posting struct {
	Doc int32
	W   float32
}

// TFIDF is the tier-T1 index: unigrams and bigrams, sublinear term-frequency
// scaling, smoothed IDF, L2-normalised rows. It is also the vector space the
// k-NN backbone is built in, which is deliberate — building the graph from the
// TF-IDF space rather than the dense space keeps the backbone, the communities
// and every analytic result depending on them available at tier T2 with no
// neural model present.
//
// Rows are held CSR-style (Indptr/Indices/Data) for the row-wise access the
// k-NN needs, and again as per-term Postings for query scoring and for the
// candidate pruning the approximate backbone does.
type TFIDF struct {
	Vocab    map[string]int32
	IDF      []float64
	DF       []int32
	Indptr   []int32
	Indices  []int32
	Data     []float32
	Postings [][]Posting
	N        int
	MaxN     int

	// tokenize is how documents were split; queries must be split the same way.
	tokenize func(string) []string
}

// BuildTFIDF indexes docs with word n-grams up to maxN, discarding terms
// appearing in fewer than minDF documents.
func BuildTFIDF(docs []string, maxN, minDF int) *TFIDF {
	return BuildTFIDFWith(docs, minDF, func(d string) []string { return Terms(d, maxN) }, maxN)
}

// BuildCharTFIDF indexes docs over character n-grams instead of words.
//
// The evaluation needs this: Task C defines its gold sets by proximity in a
// character-4-gram space that no retriever uses and that played no part in
// building the graph. Without an independent judge space the measurement is
// circular — defining gold sets from graph neighbourhoods is what made an
// earlier version of this experiment report that graph expansion won.
func BuildCharTFIDF(docs []string, n, minDF int) *TFIDF {
	return BuildTFIDFWith(docs, minDF, func(d string) []string { return CharGrams(d, n) }, 1)
}

// CharGrams returns the character n-grams of a lowercased, whitespace-collapsed
// string.
func CharGrams(s string, n int) []string {
	r := []rune(strings.ToLower(strings.Join(strings.Fields(s), " ")))
	if len(r) < n {
		return nil
	}
	out := make([]string, 0, len(r)-n+1)
	for i := 0; i+n <= len(r); i++ {
		out = append(out, string(r[i:i+n]))
	}
	return out
}

// BuildTFIDFWith is the shared construction: any tokenizer, sublinear term
// frequency, smoothed IDF, L2-normalised rows.
func BuildTFIDFWith(docs []string, minDF int, tokenize func(string) []string, maxN int) *TFIDF {
	if minDF < 1 {
		minDF = 1
	}
	n := len(docs)
	counts := make([]map[string]int, n)
	df := make(map[string]int32)
	for i, d := range docs {
		c := make(map[string]int)
		for _, t := range tokenize(d) {
			c[t]++
		}
		counts[i] = c
		for t := range c {
			df[t]++
		}
	}
	vocab := make(map[string]int32, len(df))
	var idf []float64
	var dfs []int32
	for t, d := range df {
		if int(d) < minDF {
			continue
		}
		vocab[t] = int32(len(idf))
		// sklearn's smooth_idf: ln((1+n)/(1+df)) + 1.
		idf = append(idf, math.Log(float64(1+n)/float64(1+d))+1)
		dfs = append(dfs, d)
	}

	ix := &TFIDF{
		tokenize: tokenize,
		Vocab:    vocab,
		IDF:      idf,
		DF:       dfs,
		Indptr:   make([]int32, n+1),
		Postings: make([][]Posting, len(idf)),
		N:        n,
		MaxN:     maxN,
	}
	for i := range n {
		row := make([]Posting, 0, len(counts[i]))
		var norm float64
		for t, tf := range counts[i] {
			id, ok := vocab[t]
			if !ok {
				continue
			}
			w := (1 + math.Log(float64(tf))) * idf[id] // sublinear tf
			row = append(row, Posting{Doc: id, W: float32(w)})
			norm += w * w
		}
		if norm > 0 {
			inv := float32(1 / math.Sqrt(norm))
			for j := range row {
				row[j].W *= inv
			}
		}
		sort.Slice(row, func(a, b int) bool { return row[a].Doc < row[b].Doc })
		for _, p := range row {
			ix.Indices = append(ix.Indices, p.Doc)
			ix.Data = append(ix.Data, p.W)
			ix.Postings[p.Doc] = append(ix.Postings[p.Doc], Posting{Doc: int32(i), W: p.W})
		}
		ix.Indptr[i+1] = int32(len(ix.Indices))
	}
	return ix
}

// terms splits a query the same way the corpus was split. A TFIDF decoded from
// JSON has no tokenizer, so it falls back to the word n-grams of its MaxN.
func (ix *TFIDF) terms(text string) []string {
	if ix.tokenize != nil {
		return ix.tokenize(text)
	}
	return Terms(text, ix.MaxN)
}

// Row returns document i's term ids and L2-normalised weights. The slices alias
// the index; do not mutate them.
func (ix *TFIDF) Row(i int) (ids []int32, w []float32) {
	lo, hi := ix.Indptr[i], ix.Indptr[i+1]
	return ix.Indices[lo:hi], ix.Data[lo:hi]
}

// Vector builds an L2-normalised query vector in the index's term space.
func (ix *TFIDF) Vector(text string) map[int32]float64 {
	tf := make(map[int32]int)
	for _, t := range ix.terms(text) {
		if id, ok := ix.Vocab[t]; ok {
			tf[id]++
		}
	}
	v := make(map[int32]float64, len(tf))
	var norm float64
	for id, c := range tf {
		w := (1 + math.Log(float64(c))) * ix.IDF[id]
		v[id] = w
		norm += w * w
	}
	if norm > 0 {
		inv := 1 / math.Sqrt(norm)
		for id := range v {
			v[id] *= inv
		}
	}
	return v
}

// Search ranks documents by cosine against the query. Rows are already
// normalised, so the cosine is a dot product accumulated over the query's
// postings.
func (ix *TFIDF) Search(text string, k int) []Hit {
	q := ix.Vector(text)
	if len(q) == 0 {
		return nil
	}
	scores := make([]float64, ix.N)
	touched := make([]int32, 0, 1024)
	for id, qw := range q {
		for _, p := range ix.Postings[id] {
			if scores[p.Doc] == 0 {
				touched = append(touched, p.Doc)
			}
			scores[p.Doc] += qw * float64(p.W)
		}
	}
	return topK(scores, touched, k)
}

// CosineRow scores every other document against document i. Used by the exact
// k-NN backbone.
func (ix *TFIDF) CosineRow(i int, out []float32) {
	for j := range out {
		out[j] = 0
	}
	ids, ws := ix.Row(i)
	for n, id := range ids {
		qw := ws[n]
		for _, p := range ix.Postings[id] {
			out[p.Doc] += qw * p.W
		}
	}
	out[i] = 0
}

func topK(scores []float64, touched []int32, k int) []Hit {
	hits := make([]Hit, 0, len(touched))
	for _, d := range touched {
		if scores[d] > 0 {
			hits = append(hits, Hit{Doc: int(d), Score: scores[d]})
		}
	}
	sort.Slice(hits, func(a, b int) bool {
		if hits[a].Score != hits[b].Score {
			return hits[a].Score > hits[b].Score
		}
		return hits[a].Doc < hits[b].Doc
	})
	if k > 0 && len(hits) > k {
		hits = hits[:k]
	}
	return hits
}
