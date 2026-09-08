package eval

import (
	"math/rand/v2"
	"sort"

	"github.com/yothgewalt/reharvester/internal/index"
	"github.com/yothgewalt/reharvester/internal/paper"
)

// Query is one evaluation query with its judgments.
type Query struct {
	Seed  int // the corpus position the query was drawn from
	Text  string
	Judge Judgments
}

// TaskA builds the known-item set: the query is a paper's title and the single
// relevant document is its own abstract. Because titles are excluded from every
// index, this measures whether a retriever can bridge a short, differently
// worded query to the right document.
func TaskA(papers []paper.Paper, n int, seed uint64) []Query {
	positions := sample(len(papers), n, seed)
	out := make([]Query, 0, len(positions))
	for _, i := range positions {
		out = append(out, Query{Seed: i, Text: papers[i].Title, Judge: Judgments{i: 1}})
	}
	return out
}

// GradeTaskB implements the paper's relevance criterion over arXiv's own
// multi-label category assignment, applied by authors and moderators
// independently of this system:
//
//	3  same primary category and at least 2 shared labels
//	2  same primary category
//	1  at least 2 shared secondary labels
//	0  otherwise
//
// Sharing a single broad label is not relevant, which is what keeps the task
// discriminative.
func GradeTaskB(a, b paper.Paper) int {
	if len(a.Categories) == 0 || len(b.Categories) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(b.Categories))
	for _, c := range b.Categories {
		set[c] = struct{}{}
	}
	shared := 0
	for _, c := range a.Categories {
		if _, ok := set[c]; ok {
			shared++
		}
	}
	samePrimary := a.Categories[0] == b.Categories[0]
	switch {
	case samePrimary && shared >= 2:
		return 3
	case samePrimary:
		return 2
	case shared >= 2:
		return 1
	default:
		return 0
	}
}

// TaskB builds the topical set: the query is a paper's abstract, graded against
// arXiv categories.
//
// Judgments are POOLED. The caller supplies each system's top-poolDepth results
// and only those documents are judged, so the ideal nDCG ranking comes from the
// union of all systems' output rather than from any one system's own.
func TaskB(papers []paper.Paper, n int, seed uint64) []Query {
	positions := sample(len(papers), n, seed)
	out := make([]Query, 0, len(positions))
	for _, i := range positions {
		out = append(out, Query{Seed: i, Text: papers[i].Abstract, Judge: Judgments{}})
	}
	return out
}

// Pool judges the union of every system's top-depth results for one query. The
// seed document itself is excluded: retrieving the query's own source is
// trivially correct and tells us nothing.
func Pool(q *Query, papers []paper.Paper, runs []Ranked, depth int) {
	for _, r := range runs {
		for i, d := range r {
			if i >= depth {
				break
			}
			if d == q.Seed {
				continue
			}
			if _, seen := q.Judge[d]; seen {
				continue
			}
			q.Judge[d] = GradeTaskB(papers[q.Seed], papers[d])
		}
	}
}

// GoldSet defines Task C's target documents: the nearest neighbours of a seed in
// a character-4-gram TF-IDF space, further constrained to grade >= 2 under the
// Task B criterion.
//
// The judge space is the point. It is used by no retriever and played no part in
// building the graph, which is what removes the circularity: an earlier version
// of this experiment defined gold sets from graph neighbourhoods and duly
// reported that graph expansion won.
func GoldSet(charTF *index.TFIDF, papers []paper.Paper, seed, size int) []int {
	hits := charTF.Search(papers[seed].Abstract, size*8)
	out := make([]int, 0, size)
	for _, h := range hits {
		if h.Doc == seed {
			continue
		}
		if GradeTaskB(papers[seed], papers[h.Doc]) < 2 {
			continue
		}
		out = append(out, h.Doc)
		if len(out) == size {
			break
		}
	}
	return out
}

// sample draws n distinct positions without replacement, deterministically.
func sample(total, n int, seed uint64) []int {
	if n <= 0 || n > total {
		n = total
	}
	all := make([]int, total)
	for i := range all {
		all[i] = i
	}
	rng := rand.New(rand.NewPCG(seed, seed*2+1))
	rng.Shuffle(total, func(i, j int) { all[i], all[j] = all[j], all[i] })
	out := all[:n]
	sort.Ints(out)
	return out
}
