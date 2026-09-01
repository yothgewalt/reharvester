package pipeline

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yothgewalt/reharvester/internal/analyze"
	"github.com/yothgewalt/reharvester/internal/graph"
	"github.com/yothgewalt/reharvester/internal/index"
	"github.com/yothgewalt/reharvester/internal/paper"
	"github.com/yothgewalt/reharvester/internal/retrieve"
	"github.com/yothgewalt/reharvester/internal/store"
)

// Reporter receives stage progress. The HTTP layer turns these into the log and
// progress frames the crawl stream carries; a nil reporter is fine.
type Reporter interface {
	Log(level, msg string)
	Progress(percent int, stage string)
}

type nopReporter struct{}

func (nopReporter) Log(string, string)   {}
func (nopReporter) Progress(int, string) {}

// Project is one corpus fully loaded: indexes, backbone, analytics, and the
// retrieval engine over them.
type Project struct {
	Store     *store.Project
	Papers    []paper.Paper
	Corpus    *Corpus
	Graph     *graph.Graph
	Analytics *analyze.Analytics
	Gaps      analyze.GapReport
	Engine    *retrieve.Engine
	Timings   map[string]time.Duration
}

func (p *Project) Tier() retrieve.Tier {
	if p == nil || p.Engine == nil {
		return retrieve.T0
	}
	return p.Engine.Tier()
}

// Build runs stages two through four over an already-harvested corpus and
// persists what is expensive to recompute. emb may be nil, which is exactly
// tier T2: the backbone, the communities and every analytic result are built
// from the TF-IDF space, so none of them need a neural model present.
func Build(ctx context.Context, sp *store.Project, papers []paper.Paper, emb index.Embedder, approx bool, rep Reporter) (*Project, error) {
	if rep == nil {
		rep = nopReporter{}
	}
	if len(papers) == 0 {
		return nil, fmt.Errorf("empty corpus")
	}
	p := &Project{Store: sp, Papers: papers, Timings: map[string]time.Duration{}}

	rep.Progress(15, "index")
	rep.Log("info", fmt.Sprintf("Building lexical indexes over %d abstracts", len(papers)))
	t := time.Now()
	p.Corpus = BuildLexical(papers, DefaultBuildOptions())
	p.Timings["index"] = time.Since(t)
	rep.Log("info", fmt.Sprintf("Indexed in %s — %d terms in the TF-IDF vocabulary",
		p.Timings["index"].Round(time.Millisecond), len(p.Corpus.TF.Vocab)))

	rep.Progress(45, "graph")
	rep.Log("info", "Building the mutual k-NN backbone")
	opt := graph.DefaultKNNOptions()
	opt.Approximate = approx
	t = time.Now()
	p.Graph = graph.Build(p.Corpus.TF, papers, opt)
	p.Timings["graph"] = time.Since(t)
	rep.Log("info", fmt.Sprintf("Backbone: %d edges, %d communities, %d co-author links in %s",
		len(p.Graph.Edges), len(p.Graph.Communities), len(p.Graph.Coauthor),
		p.Timings["graph"].Round(time.Millisecond)))

	rep.Progress(65, "analyse")
	rep.Log("info", "Extracting phrase trends and scoring research gaps")
	t = time.Now()
	p.Analytics = analyze.BuildAnalytics(papers, p.Graph.Community, len(p.Graph.Communities), 2)
	p.Gaps = analyze.Gaps(p.Graph, analyze.DefaultPermutations)
	p.Timings["analyse"] = time.Since(t)
	rep.Log("info", fmt.Sprintf("Tracking %d phrases; %d of %d community pairs fall below the permutation null",
		len(p.Analytics.Terms), p.Gaps.BelowThreshold, p.Gaps.Total))

	if emb != nil {
		rep.Progress(80, "encode")
		rep.Log("info", "Encoding abstracts with the local sentence encoder")
		t = time.Now()
		vecs, err := emb.Embed(ctx, p.Corpus.Abstracts)
		if err != nil {
			// The neural tier is optional by design. Losing it drops the system
			// to T2, which is a complete system, not a failure.
			rep.Log("warn", fmt.Sprintf("Encoder unavailable, staying at tier T2: %v", err))
		} else {
			p.Corpus.Dense = &index.Dense{Vecs: vecs, Emb: emb}
			p.Timings["encode"] = time.Since(t)
			rep.Log("info", fmt.Sprintf("Encoded %d abstracts in %s", len(vecs),
				p.Timings["encode"].Round(time.Millisecond)))
		}
	}

	rep.Progress(92, "persist")
	if err := p.save(); err != nil {
		return nil, err
	}
	p.Engine = p.Corpus.Engine(p.Graph)
	rep.Log("success", fmt.Sprintf("Pipeline complete at tier %s", p.Tier()))
	return p, nil
}

func (p *Project) save() error {
	if err := p.Store.SaveJSON("graph.json", p.Graph); err != nil {
		return err
	}
	if err := p.Store.SaveJSON("analytics.json", p.Analytics); err != nil {
		return err
	}
	if err := p.Store.SaveJSON("gaps.json", p.Gaps); err != nil {
		return err
	}
	if p.Corpus.Dense.Ready() {
		if err := p.Store.SaveVectors("embeddings.bin", p.Corpus.Dense.Vecs); err != nil {
			return err
		}
	}
	return nil
}

// Load reconstructs a project from disk, rebuilding the lexical indexes and
// reusing the persisted backbone, analytics and embeddings. If the artefacts
// are missing or stale relative to the corpus, it rebuilds them.
func Load(ctx context.Context, sp *store.Project, emb index.Embedder) (*Project, error) {
	papers, err := sp.ReadPapers()
	if err != nil {
		return nil, err
	}
	if len(papers) == 0 {
		return nil, fmt.Errorf("no papers in %s", sp.ID)
	}
	p := &Project{Store: sp, Papers: papers, Timings: map[string]time.Duration{}}

	t := time.Now()
	p.Corpus = BuildLexical(papers, DefaultBuildOptions())
	p.Timings["index"] = time.Since(t)

	var g graph.Graph
	if err := sp.LoadJSON("graph.json", &g); err != nil || g.N != len(papers) {
		log.Printf("pipeline: rebuilding artefacts for %s", sp.ID)
		return Build(ctx, sp, papers, emb, false, nil)
	}
	g.Index()
	p.Graph = &g
	if err := sp.LoadJSON("analytics.json", &p.Analytics); err != nil {
		return Build(ctx, sp, papers, emb, false, nil)
	}
	_ = sp.LoadJSON("gaps.json", &p.Gaps)

	if emb != nil && sp.Exists("embeddings.bin") {
		if vecs, err := sp.LoadVectors("embeddings.bin"); err == nil && len(vecs) == len(papers) {
			p.Corpus.Dense = &index.Dense{Vecs: vecs, Emb: emb}
		}
	}
	p.Engine = p.Corpus.Engine(p.Graph)
	return p, nil
}
