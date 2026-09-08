package index

// Inverted is tier T0: a plain postings map with boolean candidate generation
// and token-overlap ranking, needing nothing beyond the standard library. It is
// the bottom rung of the capability ladder and a complete system on its own —
// the paper measures it answering 70.5 % of known-item queries in the top ten,
// 77 % of what the full neural tier reaches.
type Inverted struct {
	Postings map[string][]int32
	DocLen   []int32
	N        int
}

func BuildInverted(docs []string) *Inverted {
	ix := &Inverted{Postings: make(map[string][]int32), DocLen: make([]int32, len(docs)), N: len(docs)}
	for i, d := range docs {
		toks := Unigrams(d)
		ix.DocLen[i] = int32(len(toks))
		seen := make(map[string]struct{}, len(toks))
		for _, t := range toks {
			if _, dup := seen[t]; dup {
				continue
			}
			seen[t] = struct{}{}
			ix.Postings[t] = append(ix.Postings[t], int32(i))
		}
	}
	return ix
}

// Search ranks by how many distinct query tokens a document contains, breaking
// ties toward the shorter document since the same overlap in fewer words is the
// more concentrated match.
func (ix *Inverted) Search(text string, k int) []Hit {
	overlap := make([]int32, ix.N)
	touched := make([]int32, 0, 1024)
	seenQ := make(map[string]struct{})
	for _, t := range Unigrams(text) {
		if _, dup := seenQ[t]; dup {
			continue
		}
		seenQ[t] = struct{}{}
		for _, d := range ix.Postings[t] {
			if overlap[d] == 0 {
				touched = append(touched, d)
			}
			overlap[d]++
		}
	}
	scores := make([]float64, ix.N)
	for _, d := range touched {
		l := float64(ix.DocLen[d])
		if l < 1 {
			l = 1
		}
		scores[d] = float64(overlap[d]) + 1/(1+l/1000)
	}
	return topK(scores, touched, k)
}
