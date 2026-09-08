// Package pipeline wires the five stages together and owns the in-memory view
// of a project the server answers from.
package pipeline

import (
	"time"

	"github.com/yothgewalt/reharvester/internal/index"
	"github.com/yothgewalt/reharvester/internal/paper"
	"github.com/yothgewalt/reharvester/internal/retrieve"
)

// Corpus is one project loaded and indexed. The lexical indexes are rebuilt
// from papers.jsonl rather than persisted: at the paper's own measurements that
// is a few seconds, cheaper than keeping a serialised copy correct.
type Corpus struct {
	Papers    []paper.Paper
	Abstracts []string
	Inv       *index.Inverted
	TF        *index.TFIDF
	BM        *index.BM25
	Dense     *index.Dense
	BuildTime map[string]time.Duration
}

// BuildOptions controls index construction. MaxN and MinDF apply to TF-IDF;
// the paper uses unigrams and bigrams.
type BuildOptions struct {
	MaxN  int
	MinDF int
}

func DefaultBuildOptions() BuildOptions { return BuildOptions{MaxN: 2, MinDF: 2} }

// BuildLexical constructs the three tiers that need no model. Every index is
// built over abstracts only, which is what makes the known-item task a genuine
// test: the query is a title, and no title is in any index.
func BuildLexical(papers []paper.Paper, opt BuildOptions) *Corpus {
	if opt.MaxN < 1 {
		opt = DefaultBuildOptions()
	}
	c := &Corpus{
		Papers:    papers,
		Abstracts: make([]string, len(papers)),
		BuildTime: map[string]time.Duration{},
	}
	for i, p := range papers {
		c.Abstracts[i] = p.Abstract
	}
	t := time.Now()
	c.Inv = index.BuildInverted(c.Abstracts)
	c.BuildTime["inverted"] = time.Since(t)

	t = time.Now()
	c.TF = index.BuildTFIDF(c.Abstracts, opt.MaxN, opt.MinDF)
	c.BuildTime["tfidf"] = time.Since(t)

	t = time.Now()
	c.BM = index.BuildBM25(c.Abstracts)
	c.BuildTime["bm25"] = time.Since(t)
	return c
}

// Engine exposes the corpus as a retrieval engine. nbrs may be nil below T2.
func (c *Corpus) Engine(nbrs retrieve.Neighbours) *retrieve.Engine {
	return &retrieve.Engine{Inv: c.Inv, TF: c.TF, BM: c.BM, Dense: c.Dense, Graph: nbrs}
}
