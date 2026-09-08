package rag

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/yothgewalt/reharvester/internal/index"
	"github.com/yothgewalt/reharvester/internal/retrieve"
)

// DefaultBudgetWords is the context budget question answering assembles under.
const DefaultBudgetWords = 1500

// ContextDoc is one document admitted to a context set.
type ContextDoc struct {
	Pos      int
	Title    string
	Abstract string
	Words    int
	ViaGraph bool
}

// AssembleContext seeds with hybrid retrieval and optionally expands along the
// backbone, admitting documents by score until the word budget is spent.
//
// Expanded documents must COMPETE with the seeds, not queue behind them. At a
// 1500-word budget only about eight abstracts fit, so appending neighbours after
// all fifty seeds makes expansion unreachable — it then measures as exactly zero
// effect, which looks like a finding and is actually a bug. Neighbours therefore
// inherit a decayed copy of their best seed's score, exactly as retrieval-time
// graph expansion does, and the whole candidate pool is ranked together.
//
// The paper's negative result stands on top of this: even when expansion can
// displace a seed, it does not improve gold-document coverage at equal budget,
// because it retrieves documents resembling those already retrieved and so
// trades topical diversity for local density.
func AssembleContext(eng *retrieve.Engine, titles, abstracts []string, query string, budget int, hops int) []ContextDoc {
	if budget <= 0 {
		budget = DefaultBudgetWords
	}
	seeds := eng.Search(retrieve.Hybrid, query, 50)
	if len(seeds) == 0 {
		return nil
	}
	const hopDecay = 0.9

	score := make(map[int]float64, len(seeds)*4)
	for rank, h := range seeds {
		score[h.Doc] = 1 / (60 + float64(rank+1))
	}
	viaGraph := make(map[int]bool)
	if hops > 0 && eng.Graph != nil {
		frontier := make([]int, 0, len(seeds))
		for _, h := range seeds {
			frontier = append(frontier, h.Doc)
		}
		for hop := 1; hop <= hops; hop++ {
			inherited := make(map[int]float64)
			for _, d := range frontier {
				w := score[d] * hopDecay
				for _, nb := range eng.Graph.Neighbors(d) {
					if w > inherited[nb] {
						inherited[nb] = w
					}
				}
			}
			var next []int
			for doc, w := range inherited {
				if w > score[doc] {
					if _, existing := score[doc]; !existing {
						viaGraph[doc] = true
					}
					score[doc] = w
					next = append(next, doc)
				}
			}
			frontier = next
			if len(frontier) == 0 {
				break
			}
		}
	}

	order := make([]index.Hit, 0, len(score))
	for doc, sc := range score {
		order = append(order, index.Hit{Doc: doc, Score: sc})
	}
	sort.Slice(order, func(a, b int) bool {
		if order[a].Score != order[b].Score {
			return order[a].Score > order[b].Score
		}
		return order[a].Doc < order[b].Doc
	})

	var out []ContextDoc
	spent := 0
	for _, h := range order {
		if h.Doc >= len(abstracts) {
			continue
		}
		w := len(strings.Fields(abstracts[h.Doc]))
		if spent+w > budget {
			if spent >= budget*9/10 {
				break
			}
			continue // a shorter document may still fit
		}
		spent += w
		out = append(out, ContextDoc{
			Pos: h.Doc, Title: titles[h.Doc], Abstract: abstracts[h.Doc],
			Words: w, ViaGraph: viaGraph[h.Doc],
		})
	}
	return out
}

const answerSystem = `You answer questions using only the numbered sources provided.
Cite sources inline as [1], [2] and so on. If the sources do not contain the
answer, say so plainly rather than guessing. Be concise.`

// AnswerStream grounds a question in an assembled context set and streams the
// prose back as it is written. Prefer it wherever a person is waiting: the
// context set is ready in milliseconds while generation takes seconds, so
// there is no reason to withhold the first sentence until the last one exists.
func AnswerStream(ctx context.Context, o *Ollama, question string, docs []ContextDoc, onToken func(string)) (string, error) {
	return o.GenerateStream(ctx, answerSystem, answerPrompt(question, docs), onToken)
}

// Answer grounds a question in an assembled context set.
func Answer(ctx context.Context, o *Ollama, question string, docs []ContextDoc) (string, error) {
	return o.Generate(ctx, answerSystem, answerPrompt(question, docs))
}

// answerPrompt lays the numbered sources out for citation.
func answerPrompt(question string, docs []ContextDoc) string {
	var b strings.Builder
	for i, d := range docs {
		b.WriteString("[")
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString("] ")
		b.WriteString(d.Title)
		b.WriteString("\n")
		b.WriteString(d.Abstract)
		b.WriteString("\n\n")
	}
	return "Sources:\n\n" + b.String() + "Question: " + question
}
