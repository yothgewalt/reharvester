// Package app holds the operations behind every entry point: the flag-driven
// server and the TUI both call these, so behaviour cannot drift between them.
//
// Narrative output goes through the standard log package rather than a
// reporter, which lets a caller capture everything — including the logging
// inside internal/httpapi — with a single log.SetOutput. Stage progress from a
// build still flows through pipeline.Reporter, because it carries percentages
// a log line cannot.
package app

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yothgewalt/reharvester/internal/httpapi"
	"github.com/yothgewalt/reharvester/internal/index"
	"github.com/yothgewalt/reharvester/internal/rag"
	"github.com/yothgewalt/reharvester/internal/store"
	"github.com/yothgewalt/reharvester/internal/webui"
)

// ServeConfig carries what the API needs beyond the store. Zero values are not
// useful defaults — construct it from flags or from the TUI settings screen.
type ServeConfig struct {
	Addr       string
	Project    string
	Categories []string
	MaxRecords int
	Delay      time.Duration
	Snapshot   int
	OllamaURL  string
	EmbedModel string
	ChatModel  string
}

// Serve brings up the local API and blocks until ctx is cancelled. It loads the
// most recent project so the UI has a corpus on first paint, but starts fine
// with none: every read endpoint answers empty rather than erroring, and the
// first harvest fills them.
func Serve(ctx context.Context, st *store.Store, cfg ServeConfig) error {
	o := rag.NewOllama(cfg.OllamaURL, cfg.EmbedModel, cfg.ChatModel)
	var llm *rag.Ollama
	if o.CanGenerate(ctx) {
		llm = o
		log.Printf("api: generation model %q", o.ChatModel)
	} else if o.Available(ctx) {
		log.Printf("api: model server is up but has no suitable generation model — wiki pages use template synthesis (try: ollama pull %s)", rag.SuggestChatModel())
	} else {
		log.Printf("api: no model server at %s — wiki pages use template synthesis and /health reports llm unreachable", cfg.OllamaURL)
	}
	srv := httpapi.New(st, httpapi.Config{
		Version:      Version,
		Ollama:       llm,
		Embedder:     Embedder(ctx, cfg.OllamaURL, cfg.EmbedModel),
		SnapshotSize: cfg.Snapshot,
		HarvestMax:   cfg.MaxRecords,
		Categories:   cfg.Categories,
		Delay:        cfg.Delay,
	})

	if id := PickProject(st, cfg.Project); id != "" {
		if err := srv.LoadActive(ctx, id); err != nil {
			log.Printf("api: could not load project %q: %v", id, err)
		}
	} else {
		log.Printf("api: no project on disk yet — run a harvest from the UI or with --harvest")
	}
	go srv.RunScheduler(ctx)

	hs := &http.Server{
		Addr:    cfg.Addr,
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
	if webui.Embedded() {
		log.Printf("api: listening on %s — open http://localhost%s", cfg.Addr, portSuffix(cfg.Addr))
	} else {
		log.Printf("api: listening on %s (no UI embedded — API only)", cfg.Addr)
	}
	if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// portSuffix turns a listen address into something a browser can follow, so
// ":8000" prints as a usable URL rather than as a bare bind string.
func portSuffix(addr string) string {
	if i := strings.LastIndexByte(addr, ':'); i >= 0 {
		return addr[i:]
	}
	return ":" + addr
}

// PickProject prefers the named project when it holds a corpus, otherwise the
// most recently created one. An empty return means no corpus exists anywhere.
func PickProject(st *store.Store, preferred string) string {
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

// Embedder returns the T3 encoder, or nil when no model server answers. A nil
// encoder is the T2 rung of the capability ladder, not an error.
func Embedder(ctx context.Context, baseURL, model string) index.Embedder {
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

// ConsoleReporter prints pipeline stage progress to the log.
type ConsoleReporter struct{}

func (ConsoleReporter) Log(level, msg string)          { log.Printf("  [%s] %s", level, msg) }
func (ConsoleReporter) Progress(pct int, stage string) { log.Printf("  %3d%% %s", pct, stage) }

// SplitList parses a comma-separated flag value, dropping blanks.
func SplitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
