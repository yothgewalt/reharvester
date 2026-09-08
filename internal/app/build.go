package app

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/yothgewalt/reharvester/internal/graph"
	"github.com/yothgewalt/reharvester/internal/index"
	"github.com/yothgewalt/reharvester/internal/pipeline"
	"github.com/yothgewalt/reharvester/internal/store"
)

// embedRecordFile records which encoder produced embeddings.bin. Stored beside
// the vectors rather than in meta.json so the UI's project record keeps its
// current shape.
const embedRecordFile = "embedding.json"

// EmbedRecord names the encoder a corpus was built with. Query vectors must
// come from the same model: index.Dense skips vectors whose length differs from
// the query, so a mismatched encoder yields an empty result set rather than an
// error, and /health still reports tier T3. Comparing this record at serve time
// is what makes that failure visible.
type EmbedRecord struct {
	Model string `json:"model"`
	Dim   int    `json:"dim"`
}

// BuildOptions configures a corpus build. A nil Reporter is valid and silent.
type BuildOptions struct {
	Approx     bool
	EmbedModel string
	Reporter   pipeline.Reporter
}

// Build turns a harvested corpus into indexes, the k-NN backbone and analytics.
// A nil embedder is the T2 rung of the ladder, not an error.
func Build(ctx context.Context, st *store.Store, projectID string, emb index.Embedder, opt BuildOptions) error {
	proj, err := st.Project(projectID)
	if err != nil {
		return err
	}
	papers, err := proj.ReadPapers()
	if err != nil {
		return err
	}
	rep := opt.Reporter
	if rep == nil {
		rep = ConsoleReporter{}
	}
	start := time.Now()
	p, err := pipeline.Build(ctx, proj, papers, emb, opt.Approx, rep)
	if err != nil {
		return err
	}
	if emb != nil && opt.EmbedModel != "" {
		rec := EmbedRecord{Model: opt.EmbedModel, Dim: emb.Dim()}
		if err := proj.SaveJSON(embedRecordFile, &rec); err != nil {
			log.Printf("build: could not record encoder %q: %v", opt.EmbedModel, err)
		}
	}
	log.Printf("build: tier %s, total %s", p.Tier(), time.Since(start).Round(time.Millisecond))
	return nil
}

// ReadEmbedRecord reports the encoder a project was built with. ok is false
// when the project predates this record or was built at tier T2, in which case
// no mismatch can be diagnosed and callers should stay quiet.
func ReadEmbedRecord(st *store.Store, projectID string) (rec EmbedRecord, ok bool) {
	proj, err := st.Project(projectID)
	if err != nil {
		return EmbedRecord{}, false
	}
	if !proj.Exists(embedRecordFile) {
		return EmbedRecord{}, false
	}
	if err := proj.LoadJSON(embedRecordFile, &rec); err != nil || rec.Model == "" {
		return EmbedRecord{}, false
	}
	return rec, true
}

// GraphCheck builds the backbone both ways and reports what the pruning costs,
// which is the paper's scaling result in miniature.
func GraphCheck(st *store.Store, projectID string) error {
	proj, err := st.Project(projectID)
	if err != nil {
		return err
	}
	papers, err := proj.ReadPapers()
	if err != nil {
		return err
	}
	c := pipeline.BuildLexical(papers, pipeline.DefaultBuildOptions())

	t := time.Now()
	g := graph.Build(c.TF, papers, graph.DefaultKNNOptions())
	exactBuild := time.Since(t)

	opt := graph.DefaultKNNOptions()
	opt.Approximate = true
	t = time.Now()
	approx := graph.BuildKNN(c.TF, opt)
	approxBuild := time.Since(t)

	exactSet := make(map[[2]int32]struct{}, len(g.Edges))
	for _, e := range g.Edges {
		exactSet[[2]int32{e.A, e.B}] = struct{}{}
	}
	hit := 0
	for _, e := range approx {
		if _, ok := exactSet[[2]int32{e.A, e.B}]; ok {
			hit++
		}
	}
	recall := 0.0
	if len(g.Edges) > 0 {
		recall = float64(hit) / float64(len(g.Edges))
	}
	isolated := 0
	for i := range g.N {
		if g.Degree(i) == 0 {
			isolated++
		}
	}
	log.Printf("graph: %d nodes, %d backbone edges, %d co-author edges, %d isolated",
		g.N, len(g.Edges), len(g.Coauthor), isolated)
	log.Printf("graph: %d communities, largest %d, exact build %s",
		len(g.Communities), g.Communities[0].Size, exactBuild.Round(time.Millisecond))
	log.Printf("graph: pruned backbone %d edges in %s (%.2fx), edge recall %.3f",
		len(approx), approxBuild.Round(time.Millisecond),
		float64(exactBuild)/float64(approxBuild), recall)
	for i, cm := range g.Communities {
		if i >= 8 {
			break
		}
		log.Printf("  community %2d  n=%-4d  %s", cm.ID, cm.Size, strings.Join(cm.Terms, ", "))
	}
	return nil
}
