// Command harvester-server runs the Reharvester pipeline and serves the local
// API the web UI talks to. Harvesting is the only networked operation.
//
// The operations live in internal/app; this binary is the flag interface to
// them, kept because README.md and RESULTS.md document these exact invocations
// as the reproduction path for the paper. For an interactive front end to the
// same operations, see cmd/reharvester.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yothgewalt/reharvester/internal/app"
	"github.com/yothgewalt/reharvester/internal/harvest"
	"github.com/yothgewalt/reharvester/internal/httpapi"
	"github.com/yothgewalt/reharvester/internal/rag"
	"github.com/yothgewalt/reharvester/internal/store"
)

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
		err = app.Harvest(ctx, st, *project, harvest.Query{
			Categories: app.SplitList(*categories),
			Keywords:   app.SplitList(*keywords),
			From:       *from, To: *to, Max: *maxRecords,
		}, *delay)
	case *doBuild:
		err = app.Build(ctx, st, *project, app.Embedder(ctx, *ollamaURL, *embedModel), app.BuildOptions{
			Approx:     *approx,
			EmbedModel: *embedModel,
		})
	case *doGraph:
		err = app.GraphCheck(st, *project)
	case *doAnalyze:
		err = app.Analyze(st, *project)
	default:
		err = app.Serve(ctx, st, app.ServeConfig{
			Addr:       *addr,
			Project:    *project,
			Categories: app.SplitList(*categories),
			MaxRecords: *maxRecords,
			Delay:      *delay,
			Snapshot:   *snapshotSize,
			OllamaURL:  *ollamaURL,
			EmbedModel: *embedModel,
			ChatModel:  *chatModel,
		})
	}
	if err != nil {
		log.Fatalf("%v", err)
	}
}
