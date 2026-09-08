package index

import (
	"context"
	"math"
	"sort"
)

// Embedder produces unit-length sentence embeddings. It is an interface so the
// dense tier can be absent: a nil Embedder is exactly the T2 rung of the
// capability ladder, not an error path.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dim() int
}

// Dense is tier T3: one 384-dimensional vector per abstract. It is the most
// expensive stage to build by a wide margin and the cheapest to query — a dense
// scan beats a sparse cosine over a six-figure vocabulary — so its cost is paid
// once at index time.
type Dense struct {
	Vecs [][]float32
	Emb  Embedder
}

func (d *Dense) Ready() bool { return d != nil && len(d.Vecs) > 0 && d.Emb != nil }

func (d *Dense) Search(text string, k int) []Hit {
	if !d.Ready() {
		return nil
	}
	qs, err := d.Emb.Embed(context.Background(), []string{text})
	if err != nil || len(qs) == 0 {
		return nil
	}
	return d.SearchVec(qs[0], k)
}

// SearchVec ranks by cosine against an already-encoded query. Stored vectors are
// unit length, so this is a dot product.
func (d *Dense) SearchVec(q []float32, k int) []Hit {
	hits := make([]Hit, 0, len(d.Vecs))
	for i, v := range d.Vecs {
		if len(v) != len(q) {
			continue
		}
		var s float32
		for j, x := range v {
			s += x * q[j]
		}
		hits = append(hits, Hit{Doc: i, Score: float64(s)})
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

// Normalize scales v to unit length in place, so every later comparison is a
// dot product.
func Normalize(v []float32) {
	var n float64
	for _, x := range v {
		n += float64(x) * float64(x)
	}
	if n == 0 {
		return
	}
	inv := float32(1 / math.Sqrt(n))
	for i := range v {
		v[i] *= inv
	}
}
