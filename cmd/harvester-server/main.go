// Command harvester-server runs the Reharvester pipeline and serves the local
// API the web UI talks to. Harvesting is the only networked operation.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/yothgewalt/reharvester/internal/analyze"
	"github.com/yothgewalt/reharvester/internal/graph"
	"github.com/yothgewalt/reharvester/internal/harvest"
	"github.com/yothgewalt/reharvester/internal/httpapi"
	"github.com/yothgewalt/reharvester/internal/index"
	"github.com/yothgewalt/reharvester/internal/pipeline"
	"github.com/yothgewalt/reharvester/internal/rag"
	"github.com/yothgewalt/reharvester/internal/store"
)

const version = "0.1.0"

func main() {
	var (
		data         = flag.String("data", ".reharvester", "directory holding projects and artefacts")
		addr         = flag.String("addr", ":8000", "listen address for the local API")
		project      = flag.String("project", "default", "project id to operate on")
		doHarvest    = flag.Bool("harvest", false, "run a harvest and exit instead of serving")
		doBuild      = flag.Bool("build", false, "build indexes, backbone and analytics, then exit")
		doGraph      = flag.Bool("graphcheck", false, "build the backbone both ways and report the pruning cost, then exit")
		doAnalyze    = flag.Bool("analyze", false, "report trends and gap candidates, then exit")
		categories   = flag.String("categories", "cs.IR,cs.DL,cs.CL,cs.SI,cs.DB", "comma-separated arXiv categories")
		keywords     = flag.String("keywords", "", "comma-separated keywords to AND with the categories")
		from         = flag.Int("from", 2013, "earliest submission year")
		to           = flag.Int("to", time.Now().Year(), "latest submission year")
		maxRecords   = flag.Int("max", 2000, "maximum records to retain")
		delay        = flag.Duration("delay", harvest.DefaultDelay, "politeness delay between API requests")
		approx       = flag.Bool("approx", false, "use the inverted-index pruned backbone instead of the exact one")
		ollamaURL    = flag.String("ollama", rag.DefaultBaseURL, "local model server; the dense tier is skipped when unreachable")
		embedModel   = flag.String("embed-model", rag.DefaultEmbedding, "sentence encoder model for tier T3")
		chatModel    = flag.String("chat-model", rag.DefaultChatModel, "generation model for wiki synthesis and answers")
		snapshotSize = flag.Int("snapshot-nodes", httpapi.DefaultSnapshotNodes, "papers per graph snapshot served to the UI")
	)
	flag.Parse()

	st, err := store.Open(*data)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch {
	case *doHarvest:
		err = runHarvest(ctx, st, *project, harvest.Query{
			Categories: splitList(*categories),
			Keywords:   splitList(*keywords),
			From:       *from, To: *to, Max: *maxRecords,
		}, *delay)
	case *doBuild:
		err = runBuild(ctx, st, *project, embedder(ctx, *ollamaURL, *embedModel), *approx)
	case *doGraph:
		err = runGraphCheck(st, *project)
	case *doAnalyze:
		err = runAnalyze(st, *project)
	default:
		err = serve(ctx, st, *addr, *project, harvestConfig{
			categories: splitList(*categories),
			maxRecords: *maxRecords,
			delay:      *delay,
			snapshot:   *snapshotSize,
			ollamaURL:  *ollamaURL,
			embedModel: *embedModel,
			chatModel:  *chatModel,
		})
	}
	if err != nil {
		log.Fatalf("%v", err)
	}
}

type harvestConfig struct {
	categories []string
	maxRecords int
	delay      time.Duration
	snapshot   int
	ollamaURL  string
	embedModel string
	chatModel  string
}

// serve brings up the local API. It loads the most recent project so the UI has
// a corpus on first paint, but starts fine with none: every read endpoint
// answers empty rather than erroring, and the first harvest fills them.
func serve(ctx context.Context, st *store.Store, addr, preferred string, hc harvestConfig) error {
	o := rag.NewOllama(hc.ollamaURL, hc.embedModel, hc.chatModel)
	var llm *rag.Ollama
	if o.CanGenerate(ctx) {
		llm = o
		log.Printf("api: generation model %q", o.ChatModel)
	} else if o.Available(ctx) {
		log.Printf("api: model server is up but has no suitable generation model — wiki pages use template synthesis (try: ollama pull %s)", rag.SuggestChatModel())
	} else {
		log.Printf("api: no model server at %s — wiki pages use template synthesis and /health reports llm unreachable", hc.ollamaURL)
	}
	srv := httpapi.New(st, httpapi.Config{
		Version:      version,
		Ollama:       llm,
		Embedder:     embedder(ctx, hc.ollamaURL, hc.embedModel),
		SnapshotSize: hc.snapshot,
		HarvestMax:   hc.maxRecords,
		Categories:   hc.categories,
		Delay:        hc.delay,
	})

	if id := pickProject(st, preferred); id != "" {
		if err := srv.LoadActive(ctx, id); err != nil {
			log.Printf("api: could not load project %q: %v", id, err)
		}
	} else {
		log.Printf("api: no project on disk yet — run a harvest from the UI or with --harvest")
	}
	go srv.RunScheduler(ctx)

	hs := &http.Server{
		Addr:    addr,
		Handler: srv.Handler(),
		// Generous: a crawl socket is long-lived, and harvesting is bounded by
		// the source API's politeness delay rather than by us.
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		sh, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hs.Shutdown(sh)
	}()
	log.Printf("api: listening on %s", addr)
	if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// pickProject prefers the named project when it holds a corpus, otherwise the
// most recently created one.
func pickProject(st *store.Store, preferred string) string {
	if sp, err := st.Project(preferred); err == nil && sp.Exists("papers.jsonl") {
		return preferred
	}
	metas, err := st.Projects()
	if err != nil {
		return ""
	}
	for _, m := range metas {
		if sp, err := st.Project(m.ID); err == nil && sp.Exists("papers.jsonl") {
			return m.ID
		}
	}
	return ""
}

func runHarvest(ctx context.Context, st *store.Store, projectID string, q harvest.Query, delay time.Duration) error {
	proj, err := st.Project(projectID)
	if err != nil {
		return err
	}
	c := harvest.NewClient()
	c.Delay = delay

	start := time.Now()
	papers, err := c.Harvest(ctx, q, func(fetched, total int, msg string) {
		log.Printf("harvest: %d retained (%d match this window) — %s", fetched, total, msg)
	})
	// Harvest returns what it collected alongside any error, and a full harvest
	// is minutes of politeness-limited network. Keep the partial corpus rather
	// than throwing it away on a cancellation or an upstream hiccup.
	if len(papers) == 0 {
		if err != nil {
			return err
		}
		return fmt.Errorf("no records matched")
	}
	if err != nil {
		log.Printf("harvest: interrupted after %d papers (%v) — keeping what was collected", len(papers), err)
	}
	if err := proj.WritePapers(papers); err != nil {
		return err
	}
	meta := store.Meta{
		ID: projectID, Name: projectID,
		Query:     strings.Join(q.Categories, ", "),
		CreatedAt: time.Now().UTC(), Status: "complete", DocsIngested: len(papers),
	}
	if err := proj.SaveJSON("meta.json", &meta); err != nil {
		return err
	}
	log.Printf("harvest: %d papers in %s -> %s", len(papers),
		time.Since(start).Round(time.Millisecond), proj.Path("papers.jsonl"))
	return nil
}

func runBuild(ctx context.Context, st *store.Store, projectID string, emb index.Embedder, approx bool) error {
	proj, err := st.Project(projectID)
	if err != nil {
		return err
	}
	papers, err := proj.ReadPapers()
	if err != nil {
		return err
	}
	start := time.Now()
	p, err := pipeline.Build(ctx, proj, papers, emb, approx, consoleReporter{})
	if err != nil {
		return err
	}
	log.Printf("build: tier %s, total %s", p.Tier(), time.Since(start).Round(time.Millisecond))
	return nil
}

// runGraphCheck builds the backbone both ways and reports what the pruning
// costs, which is the paper's scaling result in miniature.
func runGraphCheck(st *store.Store, projectID string) error {
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

func runAnalyze(st *store.Store, projectID string) error {
	proj, err := st.Project(projectID)
	if err != nil {
		return err
	}
	papers, err := proj.ReadPapers()
	if err != nil {
		return err
	}
	c := pipeline.BuildLexical(papers, pipeline.DefaultBuildOptions())
	g := graph.Build(c.TF, papers, graph.DefaultKNNOptions())

	t := time.Now()
	a := analyze.BuildAnalytics(papers, g.Community, len(g.Communities), 2)
	log.Printf("analyze: %d tracked terms over %v in %s", len(a.Terms), a.Years, time.Since(t).Round(time.Millisecond))

	ys := a.Years
	mid := len(ys) / 2
	early := analyze.Window{From: ys[0], To: ys[mid-1]}
	late := analyze.Window{From: ys[mid], To: ys[len(ys)-1]}
	t = time.Now()
	tr := a.Trends(early, late)
	log.Printf("analyze: early %d-%d vs late %d-%d, %d terms survive specificity, recomputed in %s",
		early.From, early.To, late.From, late.To, len(tr), time.Since(t).Round(time.Microsecond))

	log.Printf("analyze: fastest-rising")
	for i, x := range tr {
		if i >= 7 {
			break
		}
		log.Printf("  %+6.2f log2 lift  %4d late docs  S=%.2f  %s", x.Log2Lift, x.LateDocs, x.Specificity, x.Term)
	}
	log.Printf("analyze: steepest-declining")
	for i := len(tr) - 1; i >= 0 && i > len(tr)-8; i-- {
		x := tr[i]
		log.Printf("  %+6.2f log2 lift  %4d->%4d docs  %s", x.Log2Lift, x.EarlyDocs, x.LateDocs, x.Term)
	}
	byBurst := append([]analyze.Trend(nil), tr...)
	sort.Slice(byBurst, func(i, j int) bool { return byBurst[i].Burst > byBurst[j].Burst })
	log.Printf("analyze: top burst terms (after specificity filter)")
	for i, x := range byBurst {
		if i >= 5 {
			break
		}
		log.Printf("  burst %6.1f  S=%.2f  %s", x.Burst, x.Specificity, x.Term)
	}

	t = time.Now()
	rep := analyze.Gaps(g, analyze.DefaultPermutations)
	log.Printf("analyze: %d/%d community pairs reach z < %.0f in %s",
		rep.BelowThreshold, rep.Total, analyze.GapZMax, time.Since(t).Round(time.Millisecond))
	for i, p := range rep.Pairs {
		if i >= 5 {
			break
		}
		log.Printf("  sim %.3f  %d cross vs null %.0f (z=%.1f)  [%s] <-> [%s]",
			p.Similarity, p.Observed, p.NullMean, p.Z, p.LabelA, p.LabelB)
	}
	return nil
}

// embedder returns the T3 encoder, or nil when no model server answers. A nil
// encoder is the T2 rung of the capability ladder, not an error.
func embedder(ctx context.Context, baseURL, model string) index.Embedder {
	o := rag.NewOllama(baseURL, model, "")
	if !o.Available(ctx) {
		log.Printf("encoder: no model server at %s — running at tier T2", baseURL)
		return nil
	}
	if !o.HasModel(ctx, model) {
		log.Printf("encoder: model %q not pulled (try: ollama pull %s) — running at tier T2", model, model)
		return nil
	}
	return o
}

// consoleReporter prints pipeline stage progress to the log.
type consoleReporter struct{}

func (consoleReporter) Log(level, msg string)          { log.Printf("  [%s] %s", level, msg) }
func (consoleReporter) Progress(pct int, stage string) { log.Printf("  %3d%% %s", pct, stage) }

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
