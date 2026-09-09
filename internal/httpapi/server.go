package httpapi

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yothgewalt/reharvester/internal/analyze"
	"github.com/yothgewalt/reharvester/internal/index"
	"github.com/yothgewalt/reharvester/internal/paper"
	"github.com/yothgewalt/reharvester/internal/pipeline"
	"github.com/yothgewalt/reharvester/internal/rag"
	"github.com/yothgewalt/reharvester/internal/store"
	"github.com/yothgewalt/reharvester/internal/webui"
)

// Config carries what the server needs that is not derivable from the store.
type Config struct {
	Version      string
	Ollama       *rag.Ollama
	Embedder     index.Embedder
	SnapshotSize int
	HarvestMax   int
	Delay        time.Duration
	AllowOrigins []string
}

// Server holds one active project at a time. The UI's graph, trends, corpus and
// wiki endpoints carry no project id, so "the corpus" is whichever project was
// most recently built; a completed crawl makes its own project active.
type Server struct {
	store *store.Store
	hub   *Hub
	cfg   Config

	mu       sync.RWMutex
	project  *pipeline.Project
	snaps    *Snapshots
	activeID string

	// wikiSynth caches model-written orientation sections by docId. Generation
	// takes far longer than the ten seconds the client waits, so a page is
	// never blocked on it: the template render goes out immediately and the
	// synthesis lands in this cache for the next view.
	wikiSynth sync.Map
	synthing  sync.Map
}

func New(st *store.Store, cfg Config) *Server {
	if cfg.SnapshotSize <= 0 {
		cfg.SnapshotSize = DefaultSnapshotNodes
	}
	if cfg.HarvestMax <= 0 {
		cfg.HarvestMax = 2000
	}
	if len(cfg.AllowOrigins) == 0 {
		cfg.AllowOrigins = []string{"http://localhost:3000", "http://127.0.0.1:3000"}
	}
	return &Server{store: st, hub: NewHub(), cfg: cfg}
}

// LoadActive brings a project into memory and renders its snapshots. Every
// request-time endpoint reads from what this produced; nothing heavy runs
// inside a handler, because the client abandons any request over ten seconds.
func (s *Server) LoadActive(ctx context.Context, id string) error {
	sp, err := s.store.Project(id)
	if err != nil {
		return err
	}
	p, err := pipeline.Load(ctx, sp, s.cfg.Embedder)
	if err != nil {
		return err
	}
	s.setActive(id, p)
	log.Printf("api: active project %q — %d papers, tier %s", id, len(p.Papers), p.Tier())
	return nil
}

func (s *Server) setActive(id string, p *pipeline.Project) {
	snaps := BuildSnapshots(p, s.cfg.SnapshotSize)
	s.mu.Lock()
	s.project, s.snaps, s.activeID = p, snaps, id
	s.mu.Unlock()
	// The corpus changed, so every cached synthesis is about a stale
	// neighbourhood.
	s.wikiSynth.Clear()
	s.synthing.Clear()
}

func (s *Server) active() (*pipeline.Project, *Snapshots) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.project, s.snaps
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	// Health sits outside /api/v1 and is polled under a 3 s deadline.
	mux.HandleFunc("GET /health", s.health)

	mux.HandleFunc("POST /api/v1/harvest/init", s.harvestInit)
	mux.HandleFunc("/api/v1/harvest/stream/{taskId}", s.stream)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/action", s.projectAction)
	mux.HandleFunc("GET /api/v1/projects", s.projects)
	mux.HandleFunc("POST /api/v1/jobs/register", s.registerJob)
	mux.HandleFunc("GET /api/v1/jobs", s.listJobs)
	mux.HandleFunc("GET /api/v1/graph/snapshot", s.snapshot)
	mux.HandleFunc("GET /api/v1/wiki/raw/{docId}", s.wiki)
	mux.HandleFunc("GET /api/v1/trends/recalculate", s.trends)
	mux.HandleFunc("GET /api/v1/gaps/positions", s.gaps)
	mux.HandleFunc("GET /api/v1/gaps/pairs", s.gapPairs)
	mux.HandleFunc("GET /api/v1/communities", s.communities)
	mux.HandleFunc("GET /api/v1/communities/links", s.communityLinks)
	mux.HandleFunc("POST /api/v1/ask", s.ask)
	mux.HandleFunc("POST /api/v1/ask/stream", s.askStream)

	// The UI, when it was built into the binary. Everything above is /health or
	// /api/v1/*, so this catch-all cannot shadow an endpoint. Absent a build it
	// stays unmounted and the API serves itself.
	if ui := webui.Handler(); ui != nil {
		mux.Handle("/", ui)
	}
	return s.cors(s.accessLog(mux))
}

// accessLog records one line per request. It exists because a packaged install
// serves the UI from this process: without it, a missing asset fails in the
// browser and leaves no trace anywhere the user can see. Static assets are
// logged only when they fail, so a page load stays one line rather than fifty.
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if isAPIPath(r.URL.Path) || rec.status >= http.StatusBadRequest {
			log.Printf("http: %d %s %s (%s)", rec.status, r.Method, r.URL.Path,
				time.Since(start).Round(time.Millisecond))
		}
	})
}

// statusRecorder captures the status code for the access log. Hijack is
// forwarded because the crawl endpoint upgrades to a WebSocket, and Flush
// because job progress streams.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.written {
		r.status, r.written = code, true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.written = true
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("httpapi: response writer does not support hijack")
	}
	return h.Hijack()
}

func isAPIPath(p string) bool {
	return p == "/health" || strings.HasPrefix(p, "/api/")
}

// cors admits the Next dev server. The API binds to localhost and holds no
// credentials, so the allowance is deliberately narrow rather than "*".
func (s *Server) cors(next http.Handler) http.Handler {
	allowed := make(map[string]bool, len(s.cfg.AllowOrigins))
	for _, o := range s.cfg.AllowOrigins {
		allowed[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" && allowed[o] {
			w.Header().Set("Access-Control-Allow-Origin", o)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api: encode: %v", err)
	}
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	// The client aborts this at three seconds, so the model probe gets a much
	// shorter deadline than that and its absence is an answer, not an error.
	ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
	defer cancel()
	// A reachable server whose only model is the encoder still cannot generate,
	// so the badge the UI shows must reflect generation, not reachability.
	llm := "unreachable"
	if s.cfg.Ollama != nil && s.cfg.Ollama.CanGenerate(ctx) {
		llm = "ok"
	}
	p, _ := s.active()
	tier := "T0"
	if p != nil {
		tier = string(p.Tier())
	}
	writeJSON(w, HealthStatus{Status: "ok", LLM: llm, Version: s.cfg.Version, Tier: tier})
}

func (s *Server) snapshot(w http.ResponseWriter, r *http.Request) {
	_, snaps := s.active()
	if snaps == nil {
		writeJSON(w, GraphSnapshot{Nodes: []GraphNode{}, Edges: []GraphEdge{},
			GeneratedAt: time.Now().UTC().Format(time.RFC3339)})
		return
	}
	if r.URL.Query().Get("kind") == "cooccurrence" {
		writeJSON(w, snaps.Cooccurring)
		return
	}
	writeJSON(w, snaps.Knowledge)
}

func (s *Server) gaps(w http.ResponseWriter, r *http.Request) {
	_, snaps := s.active()
	ids := []string{}
	if snaps != nil {
		ids = snaps.GapNodeIDs
	}
	writeJSON(w, GapPositions{GapNodeIDs: ids})
}

// gapPairs serves the ranked community pairs behind the paper's Fig. 2d.
//
// It returns the report built at analysis time rather than recomputing it:
// Community.Centroid is json:"-" and does not survive a reload, so a recomputed
// report would score every pair's similarity as zero. BelowThreshold and Total
// ship with the pairs because the ranking is carried by centroid similarity
// rather than by the weak z-filter — the caller needs both to say so honestly.
func (s *Server) gapPairs(w http.ResponseWriter, r *http.Request) {
	p, _ := s.active()
	if p == nil {
		writeJSON(w, analyze.GapReport{Pairs: []analyze.GapPair{}})
		return
	}
	rep := p.Gaps
	if rep.Pairs == nil {
		rep.Pairs = []analyze.GapPair{}
	}
	writeJSON(w, rep)
}

// communityLinks counts backbone edges crossing each pair of communities —
// the edges of the paper's coarse-grained field map (Fig. 2c). Every pair is
// returned with its raw count; thresholding is the caller's decision, and with
// at most a few dozen communities the whole set is small.
func (s *Server) communityLinks(w http.ResponseWriter, r *http.Request) {
	p, _ := s.active()
	out := []CommunityLink{}
	if p == nil || p.Graph == nil {
		writeJSON(w, out)
		return
	}
	g := p.Graph
	counts := make(map[[2]int]int)
	for _, e := range g.Edges {
		if int(e.A) >= len(g.Community) || int(e.B) >= len(g.Community) {
			continue
		}
		a, b := int(g.Community[e.A]), int(g.Community[e.B])
		if a < 0 || b < 0 || a == b {
			continue
		}
		if a > b {
			a, b = b, a
		}
		counts[[2]int{a, b}]++
	}
	for pair, n := range counts {
		out = append(out, CommunityLink{Source: pair[0], Target: pair[1], Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	writeJSON(w, out)
}

// communities lists the Louvain partitions for the reader's left rail. Members
// outside the snapshot have no node id, so MemberDocIDs is a subset of Size.
func (s *Server) communities(w http.ResponseWriter, r *http.Request) {
	p, snaps := s.active()
	out := []Community{}
	if p == nil || p.Graph == nil || snaps == nil {
		writeJSON(w, out)
		return
	}
	for _, c := range p.Graph.Communities {
		ids := []string{}
		for _, m := range c.Members {
			if id, ok := snaps.DocIDByPos[int(m)]; ok {
				ids = append(ids, id)
			}
		}
		out = append(out, Community{
			ID:    c.ID,
			Label: c.Label,
			Slug:  paper.Slug(c.Label),
			Size:  c.Size,
			// Terms feeds the rail's "a / b" label, so ship them all.
			Terms:        c.Terms,
			MemberDocIDs: ids,
		})
	}
	writeJSON(w, out)
}

// trends answers a window recomputation. The UI supplies the late window; the
// early window is the equal-length span immediately before it, clamped to the
// corpus. growthRate carries the paper's smoothed log2 prevalence lift rather
// than a regression slope.
func (s *Server) trends(w http.ResponseWriter, r *http.Request) {
	p, _ := s.active()
	if p == nil || p.Analytics == nil || len(p.Analytics.Years) == 0 {
		writeJSON(w, []TrendKeyword{})
		return
	}
	years := p.Analytics.Years
	start := atoiDefault(r.URL.Query().Get("start"), years[0])
	end := atoiDefault(r.URL.Query().Get("end"), years[len(years)-1])
	if end < start {
		start, end = end, start
	}
	span := end - start + 1
	late := analyze.Window{From: start, To: end}
	early := analyze.Window{From: start - span, To: start - 1}
	if early.From < years[0] {
		early.From = years[0]
	}
	if early.To < early.From {
		early.To = early.From
	}

	// Trends comes back sorted by lift descending, so the steepest declines are
	// the tail. Both ends ship: a decline is a finding, not a leftover, and a
	// term that fell to zero late docs is the sharpest one there is.
	trends := p.Analytics.Trends(early, late)
	out := make([]TrendKeyword, 0, 2*TrendsPerDirection)
	emit := func(t analyze.Trend, dir string, rank int) {
		out = append(out, TrendKeyword{
			Keyword:     t.Term,
			Count:       t.LateDocs,
			EarlyDocs:   t.EarlyDocs,
			GrowthRate:  round2(t.Log2Lift),
			Direction:   dir,
			Rank:        rank,
			Specificity: round2(t.Specificity),
			Burst:       round2(t.Burst),
			Trajectory:  trajectory(p.Analytics, t.Term),
		})
	}
	for i := 0; i < len(trends) && i < TrendsPerDirection; i++ {
		emit(trends[i], "rising", i+1)
	}
	for i, rank := len(trends)-1, 1; i >= 0 && rank <= TrendsPerDirection; i, rank = i-1, rank+1 {
		if trends[i].Log2Lift >= 0 {
			break // never report a riser as a decline on a small corpus
		}
		emit(trends[i], "declining", rank)
	}
	writeJSON(w, out) // a bare array: the client expects no envelope
}

// trajectory turns a term's per-year prevalence map into an ordered series.
func trajectory(a *analyze.Analytics, term string) []TrendPoint {
	byYear := a.Prevalence(term)
	if len(byYear) == 0 {
		return nil
	}
	out := make([]TrendPoint, 0, len(a.Years))
	for _, y := range a.Years {
		out = append(out, TrendPoint{Year: y, Prevalence: byYear[y]})
	}
	return out
}

func (s *Server) wiki(w http.ResponseWriter, r *http.Request) {
	docID := r.PathValue("docId")
	p, snaps := s.active()
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	if p == nil || snaps == nil {
		w.Write([]byte("# " + docID + "\n\n_No corpus is loaded yet._\n"))
		return
	}
	d, ok := s.wikiDoc(docID, p, snaps)
	if !ok {
		// A 404 surfaces as an inline error in the reader; a placeholder page
		// is the friendlier answer and matches the mock's own behaviour.
		w.Write([]byte("# " + docID + "\n\n_No wiki entry for this node yet._\n"))
		return
	}
	if cached, ok := s.wikiSynth.Load(docID); ok {
		w.Write([]byte(cached.(string)))
		return
	}
	// Answer now with the template render — a complete page that needs no model
	// — and synthesise in the background for the next view.
	w.Write([]byte(rag.Render(d)))
	if s.cfg.Ollama != nil {
		s.synthesizeInBackground(docID, d)
	}
}

// synthesizeInBackground generates at most one orientation section per docId at
// a time, detached from the request that triggered it.
func (s *Server) synthesizeInBackground(docID string, d rag.WikiDoc) {
	if _, busy := s.synthing.LoadOrStore(docID, true); busy {
		return
	}
	go func() {
		defer s.synthing.Delete(docID)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		out := rag.Synthesize(ctx, s.cfg.Ollama, d)
		// Synthesize returns the plain render on any failure; caching that would
		// pin the page to a version that never improves.
		if out != rag.Render(d) {
			s.wikiSynth.Store(docID, out)
		}
	}()
}

func (s *Server) wikiDoc(docID string, p *pipeline.Project, snaps *Snapshots) (rag.WikiDoc, bool) {
	if members, ok := snaps.ConceptDocs[docID]; ok {
		d := rag.WikiDoc{ID: docID, Title: snaps.Labels[docID], Kind: "concept", Cluster: snaps.ConceptOf[docID]}
		for _, m := range members {
			if id, ok := snaps.DocIDByPos[m]; ok {
				d.Members = append(d.Members, rag.WikiLink{ID: id, Label: p.Papers[m].Title})
			}
		}
		return d, true
	}
	pos, ok := snaps.PosByDocID[docID]
	if !ok {
		return rag.WikiDoc{}, false
	}
	pp := p.Papers[pos]
	d := rag.WikiDoc{
		ID: docID, Title: pp.Title, Kind: "paper", Year: pp.Year(),
		Abstract: pp.Abstract, Authors: pp.Authors, Cluster: "unclustered",
	}
	if g := p.Graph; g != nil {
		if pos < len(g.PageRank) {
			d.PageRank = g.PageRank[pos]
		}
		if pos < len(g.Bridge) {
			d.Bridge = g.Bridge[pos]
		}
		for _, c := range g.Communities {
			if int32(c.ID) == g.Community[pos] {
				d.Cluster = c.Label
				break
			}
		}
		// Rank by cosine before truncating: the adjacency is in edge-emission
		// order, so the first 8 are otherwise an arbitrary slice of the fan-out.
		nbs, ws := g.NeighborsWeighted(pos)
		for i, nb := range nbs {
			if id, ok := snaps.DocIDByPos[nb]; ok {
				d.Neighbors = append(d.Neighbors, rag.WikiLink{
					ID: id, Label: p.Papers[nb].Title, Weight: float64(ws[i]),
				})
			}
		}
		sort.Slice(d.Neighbors, func(i, j int) bool {
			return d.Neighbors[i].Weight > d.Neighbors[j].Weight
		})
		if len(d.Neighbors) > 8 {
			d.Neighbors = d.Neighbors[:8]
		}
	}
	return d, true
}

func (s *Server) projects(w http.ResponseWriter, r *http.Request) {
	metas, err := s.store.Projects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]ProjectSummary, 0, len(metas))
	for _, m := range metas {
		out = append(out, ProjectSummary{
			ID: m.ID, Name: m.Name, Query: m.Query,
			CreatedAt: m.CreatedAt.Format(time.RFC3339), Status: m.Status,
			DocsIngested: m.DocsIngested,
		})
	}
	writeJSON(w, out)
}

func (s *Server) registerJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string   `json:"name"`
		Keywords []string `json:"keywords"`
		Cron     string   `json:"cron"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" || len(req.Keywords) == 0 {
		http.Error(w, "name and keywords are required", http.StatusBadRequest)
		return
	}
	j, err := s.store.AddJob(store.Job{Name: req.Name, Keywords: req.Keywords, Cron: req.Cron})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, jobToProfile(j))
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.store.Jobs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]SchedulerProfile, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, jobToProfile(j))
	}
	writeJSON(w, out)
}

func jobToProfile(j store.Job) SchedulerProfile {
	return SchedulerProfile{
		ID: j.ID, Name: j.Name, Keywords: j.Keywords, Cron: j.Cron,
		CreatedAt: j.CreatedAt.Format(time.RFC3339), Status: j.Status,
	}
}

// askSource is one document admitted to the context set, as the UI sees it.
type askSource struct {
	DocID    string `json:"docId"` // "" when the paper is outside the snapshot
	Title    string `json:"title"`
	ViaGraph bool   `json:"viaGraph"`
	Words    int    `json:"words"`
}

type askPrep struct {
	Question string
	Docs     []rag.ContextDoc
	Sources  []askSource
	Tier     string
	Budget   int
	Hops     int
}

// noModelAnswer stands in when generation is unavailable. The context set is
// still the useful half of the answer.
const noModelAnswer = "No local model is available, so no prose was generated. The retrieved sources below are the assembled context."

// prepareAsk does everything up to generation. It is fast — retrieval and
// context assembly are milliseconds against seconds of decoding — which is why
// the streaming endpoint can show sources almost immediately.
//
// A false second return means a reply has already been written.
func (s *Server) prepareAsk(w http.ResponseWriter, r *http.Request) (askPrep, bool) {
	var req struct {
		Question string `json:"question"`
		Hops     *int   `json:"hops"` // nil means "unspecified"; 0 disables expansion
		Budget   int    `json:"budget"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Question) == "" {
		http.Error(w, "question is required", http.StatusBadRequest)
		return askPrep{}, false
	}
	p, snaps := s.active()
	if p == nil {
		http.Error(w, "no corpus loaded", http.StatusServiceUnavailable)
		return askPrep{}, false
	}
	titles := make([]string, len(p.Papers))
	for i := range p.Papers {
		titles[i] = p.Papers[i].Title
	}
	// One hop by default: hops=0 disables expansion entirely, which would leave
	// the UI's expansion chips permanently empty.
	hops := 1
	if req.Hops != nil {
		hops = *req.Hops
	}
	budget := req.Budget
	if budget <= 0 {
		budget = rag.DefaultBudgetWords
	}
	docs := rag.AssembleContext(p.Engine, titles, p.Corpus.Abstracts, req.Question, budget, hops)

	prep := askPrep{
		Question: req.Question, Docs: docs, Sources: []askSource{},
		Tier: string(p.Tier()), Budget: budget, Hops: hops,
	}
	for _, d := range docs {
		prep.Sources = append(prep.Sources, askSource{
			DocID: snaps.DocIDByPos[d.Pos], Title: d.Title,
			ViaGraph: d.ViaGraph, Words: d.Words,
		})
	}
	return prep, true
}

func (s *Server) ask(w http.ResponseWriter, r *http.Request) {
	prep, ok := s.prepareAsk(w, r)
	if !ok {
		return
	}
	resp := struct {
		Answer  string      `json:"answer"`
		Sources []askSource `json:"sources"`
		Tier    string      `json:"tier"`
		Budget  int         `json:"budget"`
		Hops    int         `json:"hops"`
	}{Sources: prep.Sources, Tier: prep.Tier, Budget: prep.Budget, Hops: prep.Hops}

	if s.cfg.Ollama != nil {
		ctx, cancel := context.WithTimeout(r.Context(), askGenerateTimeout)
		defer cancel()
		if a, err := rag.Answer(ctx, s.cfg.Ollama, prep.Question, prep.Docs); err == nil {
			resp.Answer = a
		}
	}
	if resp.Answer == "" {
		resp.Answer = noModelAnswer
	}
	writeJSON(w, resp)
}

// askGenerateTimeout bounds a single answer. Decoding runs at tens of
// milliseconds per token on a laptop CPU, so a long answer legitimately takes
// most of a minute; the non-streaming endpoint gets the tighter bound because a
// caller there is blocked with nothing to show.
const (
	askGenerateTimeout = 60 * time.Second
	askStreamTimeout   = 10 * time.Minute
)

// askStream answers over Server-Sent Events: the sources first, then the prose
// token by token.
//
// Retrieval costs milliseconds and generation costs seconds, so withholding the
// whole reply until the last token exists is the difference between a page that
// responds immediately and one that appears hung. Events are "sources", "token",
// "done" and "error".
func (s *Server) askStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	prep, ok := s.prepareAsk(w, r)
	if !ok {
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // defeat proxy buffering, which would defeat the point
	w.WriteHeader(http.StatusOK)

	send := func(event string, payload any) bool {
		b, err := json.Marshal(payload)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	send("sources", map[string]any{
		"sources": prep.Sources, "tier": prep.Tier,
		"budget": prep.Budget, "hops": prep.Hops,
	})

	if s.cfg.Ollama == nil {
		send("done", map[string]any{"answer": noModelAnswer, "generated": false})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), askStreamTimeout)
	defer cancel()

	answer, err := rag.AnswerStream(ctx, s.cfg.Ollama, prep.Question, prep.Docs,
		func(chunk string) { send("token", map[string]string{"text": chunk}) })

	// A partial answer plus an error still beats discarding what arrived: the
	// reader has already seen those tokens on screen.
	if err != nil && answer == "" {
		log.Printf("api: ask stream: %v", err)
		send("error", map[string]string{"message": "generation failed — the sources above are the assembled context"})
		return
	}
	if err != nil {
		log.Printf("api: ask stream ended early: %v", err)
	}
	send("done", map[string]any{"answer": answer, "generated": true, "truncated": err != nil})
}

func atoiDefault(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

func newTaskID() string {
	var b [12]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// slugKeywords turns a pasted abstract into harvest keywords by taking its most
// frequent multi-word phrases, preferring bigrams as more discriminating.
func slugKeywords(text string, n int) []string {
	counts := map[string]int{}
	for _, t := range index.Terms(text, 2) {
		counts[t]++
	}
	type kv struct {
		t string
		c int
	}
	all := make([]kv, 0, len(counts))
	for t, c := range counts {
		weight := c
		if strings.Contains(t, " ") {
			weight *= 3
		}
		all = append(all, kv{t, weight})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].c != all[j].c {
			return all[i].c > all[j].c
		}
		return all[i].t < all[j].t
	})
	out := make([]string, 0, n)
	for _, k := range all {
		if len(out) == n {
			break
		}
		out = append(out, k.t)
	}
	return out
}
