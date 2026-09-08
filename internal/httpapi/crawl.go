package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/ledongthuc/pdf"

	"github.com/yothgewalt/reharvester/internal/harvest"
	"github.com/yothgewalt/reharvester/internal/pipeline"
	"github.com/yothgewalt/reharvester/internal/store"
)

// maxUpload matches the client's own ceiling: five PDFs, 20 MB each.
const maxUpload = 50 << 20

// deltaChunk is how many nodes travel per graph_delta frame, and
// maxEdgesPerFrame bounds the edges in one.
//
// Both caps exist because a frame's edge count grows with the chunk index: by
// the fifth chunk, every edge into any of the already-known nodes is eligible,
// and a single frame ran past the 32 KB WebSocket limit and dropped the
// connection before the done frame — which the UI reads as a failed crawl.
const (
	deltaChunk       = 25
	maxEdgesPerFrame = 120
)

// maxDeltaNodes caps what the stream pushes. The full snapshot is available
// from GET /graph/snapshot; the stream exists to show the map populating.
const maxDeltaNodes = 400

func (s *Server) harvestInit(w http.ResponseWriter, r *http.Request) {
	q := harvest.Query{
		Categories: s.cfg.Categories,
		From:       time.Now().Year() - 7,
		To:         time.Now().Year(),
		Max:        s.cfg.HarvestMax,
	}
	var label string

	ct := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "multipart/form-data"):
		if err := r.ParseMultipartForm(maxUpload); err != nil {
			http.Error(w, "upload too large", http.StatusBadRequest)
			return
		}
		var seed strings.Builder
		for _, fh := range r.MultipartForm.File["pdf"] {
			f, err := fh.Open()
			if err != nil {
				continue
			}
			text := extractPDF(f, fh.Size)
			f.Close()
			seed.WriteString(text)
			seed.WriteString("\n")
		}
		q.Keywords = slugKeywords(seed.String(), 6)
		label = fmt.Sprintf("%d PDF(s)", len(r.MultipartForm.File["pdf"]))
	default:
		var req struct {
			Keywords []string `json:"keywords"`
			Abstract string   `json:"abstract"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch {
		case len(req.Keywords) > 0:
			q.Keywords = req.Keywords
			label = strings.Join(req.Keywords, ", ")
		case strings.TrimSpace(req.Abstract) != "":
			q.Keywords = slugKeywords(req.Abstract, 6)
			label = firstWords(req.Abstract, 6)
		default:
			http.Error(w, "keywords, abstract or pdf is required", http.StatusBadRequest)
			return
		}
	}
	if len(q.Keywords) == 0 {
		http.Error(w, "could not derive a query from the input", http.StatusBadRequest)
		return
	}

	taskID := newTaskID()
	task := s.hub.New(taskID, "harvest")
	// The client uses taskId as its own project id, so the project directory is
	// named for it and the two stay in step.
	go s.runHarvest(context.WithoutCancel(r.Context()), task, taskID, label, q)
	writeJSON(w, TaskResponse{TaskID: taskID, StreamPath: "/api/v1/harvest/stream/" + taskID})
}

func (s *Server) runHarvest(ctx context.Context, task *Task, projectID, label string, q harvest.Query) {
	start := time.Now()
	// Whatever happens, the client must see a done frame: a close without one
	// marks the project failed and starts six reconnect attempts.
	docs := 0
	defer func() { task.Done(docs, time.Since(start)) }()

	sp, err := s.store.Project(projectID)
	if err != nil {
		task.Log("error", "Could not open project storage: "+err.Error())
		return
	}
	meta := store.Meta{
		ID: projectID, Name: label, Query: strings.Join(q.Keywords, ", "),
		CreatedAt: time.Now().UTC(), Status: "crawling",
	}
	_ = sp.SaveJSON("meta.json", &meta)

	task.Log("info", "Harvest task accepted: "+label)
	task.Log("info", fmt.Sprintf("Query: %s over %v, %d-%d",
		strings.Join(q.Keywords, " OR "), q.Categories, q.From, q.To))
	task.Progress(3, "harvest")

	c := harvest.NewClient()
	if s.cfg.Delay > 0 {
		c.Delay = s.cfg.Delay
	}
	papers, err := c.Harvest(ctx, q, func(fetched, total int, msg string) {
		pct := 3
		if q.Max > 0 {
			pct = 3 + fetched*10/q.Max
		}
		task.Progress(min(pct, 13), "harvest")
		// fetched is cumulative across the whole harvest while total is the
		// current year's match count, so they are reported separately rather
		// than as a misleading ratio.
		task.Log("info", fmt.Sprintf("Retained %d papers so far — %s (%d match this window)", fetched, msg, total))
	})
	if err != nil {
		task.Log("error", "Harvest failed: "+err.Error())
		meta.Status = "failed"
		_ = sp.SaveJSON("meta.json", &meta)
		return
	}
	if len(papers) == 0 {
		task.Log("warn", "No records matched the query")
		meta.Status = "failed"
		_ = sp.SaveJSON("meta.json", &meta)
		return
	}
	if err := sp.WritePapers(papers); err != nil {
		task.Log("error", "Could not write the corpus: "+err.Error())
		return
	}
	docs = len(papers)
	task.Log("success", fmt.Sprintf("Harvested %d papers; the network is no longer needed", docs))

	p, err := pipeline.Build(ctx, sp, papers, s.cfg.Embedder, false, task)
	if err != nil {
		task.Log("error", "Pipeline failed: "+err.Error())
		meta.Status = "failed"
		_ = sp.SaveJSON("meta.json", &meta)
		return
	}
	s.setActive(projectID, p)

	_, snaps := s.active()
	task.Progress(96, "graph-delta")
	streamSnapshot(task, snaps.Knowledge)

	meta.Status = "complete"
	meta.DocsIngested = docs
	_ = sp.SaveJSON("meta.json", &meta)
	task.Progress(100, "done")
	task.Log("success", fmt.Sprintf("Harvest complete — %d documents ingested at tier %s", docs, p.Tier()))
}

// streamSnapshot pushes nodes in chunks, then the edges those nodes made valid.
// An edge that arrives before its endpoints is silently discarded by the graph
// store, so nodes always go first and edges follow in bounded batches.
func streamSnapshot(task *Task, snap GraphSnapshot) {
	nodes := snap.Nodes
	if len(nodes) > maxDeltaNodes {
		nodes = nodes[:maxDeltaNodes]
	}
	byNode := make(map[string][]GraphEdge)
	for _, e := range snap.Edges {
		byNode[e.Target] = append(byNode[e.Target], e)
	}
	known := make(map[string]bool, len(nodes))
	sent := make(map[string]bool)
	for i := 0; i < len(nodes); i += deltaChunk {
		chunk := nodes[i:min(i+deltaChunk, len(nodes))]
		task.Delta(chunk, nil)
		for _, n := range chunk {
			known[n.ID] = true
		}
		var batch []GraphEdge
		for _, n := range chunk {
			for _, e := range byNode[n.ID] {
				if sent[e.ID] || !known[e.Source] || !known[e.Target] {
					continue
				}
				sent[e.ID] = true
				batch = append(batch, e)
				if len(batch) == maxEdgesPerFrame {
					// Endpoints are already on the client, so an edge-only
					// frame is safe and keeps every frame small.
					task.Delta(nil, batch)
					batch = nil
				}
			}
		}
		if len(batch) > 0 {
			task.Delta(nil, batch)
		}
	}
}

// stream serves the crawl WebSocket. It replays the task's buffered frames,
// then follows live, and always ends with a done frame before closing.
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskId")
	task := s.hub.Get(taskID)
	if task == nil {
		http.Error(w, "unknown task", http.StatusNotFound)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:3000", "127.0.0.1:3000", "localhost:*"},
	})
	if err != nil {
		log.Printf("api: websocket accept: %v", err)
		return
	}
	defer conn.CloseNow()

	ctx := r.Context()
	replay, live := task.attach()
	defer task.detach(live)

	for _, f := range replay {
		if err := conn.Write(ctx, websocket.MessageText, f); err != nil {
			return
		}
	}
	if live == nil {
		conn.Close(websocket.StatusNormalClosure, "task complete")
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case f := <-live:
			if err := conn.Write(ctx, websocket.MessageText, f); err != nil {
				return
			}
			if isDone(f) {
				conn.Close(websocket.StatusNormalClosure, "task complete")
				return
			}
		}
	}
}

func isDone(f []byte) bool { return bytes.Contains(f, []byte(`"type":"done"`)) }

// projectAction runs one of the four operations the UI's action menu offers,
// over the same task stream machinery as a harvest.
func (s *Server) projectAction(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	switch req.Action {
	case "add", "subtract", "extract", "summarize":
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	taskID := newTaskID()
	task := s.hub.New(taskID, req.Action)
	go s.runAction(context.WithoutCancel(r.Context()), task, projectID, req.Action)
	writeJSON(w, TaskResponse{TaskID: taskID, StreamPath: "/api/v1/harvest/stream/" + taskID})
}

func (s *Server) runAction(ctx context.Context, task *Task, projectID, action string) {
	start := time.Now()
	count := 0
	defer func() { task.Done(count, time.Since(start)) }()

	sp, err := s.store.Project(projectID)
	if err != nil {
		task.Log("error", err.Error())
		return
	}
	papers, err := sp.ReadPapers()
	if err != nil {
		task.Log("error", "This project has no corpus on disk: "+err.Error())
		return
	}
	task.Progress(15, action+"-start")

	switch action {
	case "add":
		task.Log("info", "Expanding the corpus with another page of matching records")
		var meta store.Meta
		_ = sp.LoadJSON("meta.json", &meta)
		q := harvest.Query{
			Categories: s.cfg.Categories, Keywords: splitComma(meta.Query),
			From: time.Now().Year() - 7, To: time.Now().Year(), Max: s.cfg.HarvestMax / 2,
		}
		c := harvest.NewClient()
		if s.cfg.Delay > 0 {
			c.Delay = s.cfg.Delay
		}
		got, err := c.Harvest(ctx, q, func(f, t int, msg string) { task.Log("info", msg) })
		if err != nil {
			task.Log("warn", "Expansion stopped early: "+err.Error())
		}
		have := make(map[string]bool, len(papers))
		for _, p := range papers {
			have[p.ID] = true
		}
		added := 0
		for _, p := range got {
			if !have[p.ID] {
				papers = append(papers, p)
				added++
			}
		}
		task.Log("info", fmt.Sprintf("Added %d new papers", added))

	case "subtract":
		// Drop the least structurally connected tenth: the documents the
		// backbone never wired into a neighbourhood.
		p, err := pipeline.Load(ctx, sp, s.cfg.Embedder)
		if err != nil {
			task.Log("error", err.Error())
			return
		}
		cut := len(papers) / 10
		task.Log("info", fmt.Sprintf("Removing the %d least connected documents", cut))
		order := make([]int, len(papers))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(a, b int) bool {
			return p.Graph.Degree(order[a]) < p.Graph.Degree(order[b])
		})
		drop := make(map[int]bool, cut)
		for i := range cut {
			drop[order[i]] = true
		}
		kept := papers[:0:0]
		for i, pp := range papers {
			if !drop[i] {
				kept = append(kept, pp)
			}
		}
		papers = kept

	case "extract":
		task.Log("info", "Re-running phrase extraction and gap scoring over the existing corpus")

	case "summarize":
		task.Log("info", "Synthesising wiki pages for the most central documents")
		if s.cfg.Ollama == nil {
			task.Log("warn", "No local model is reachable — pages will use template synthesis")
		}
	}

	if action != "extract" && action != "summarize" {
		if err := sp.WritePapers(papers); err != nil {
			task.Log("error", err.Error())
			return
		}
	}
	task.Progress(55, action+"-rebuild")
	p, err := pipeline.Build(ctx, sp, papers, s.cfg.Embedder, false, task)
	if err != nil {
		task.Log("error", err.Error())
		return
	}
	s.setActive(projectID, p)
	_, snaps := s.active()
	task.Progress(96, "graph-delta")
	streamSnapshot(task, snaps.Knowledge)

	count = len(papers)
	var meta store.Meta
	_ = sp.LoadJSON("meta.json", &meta)
	meta.DocsIngested = count
	meta.Status = "complete"
	_ = sp.SaveJSON("meta.json", &meta)
	task.Progress(100, "done")
	task.Log("success", fmt.Sprintf("Action %q complete — %d documents at tier %s", action, count, p.Tier()))
}

func splitComma(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func firstWords(s string, n int) string {
	f := strings.Fields(s)
	if len(f) > n {
		return strings.Join(f[:n], " ") + "…"
	}
	return strings.Join(f, " ")
}

// extractPDF pulls what text it can from an upload. A PDF that resists parsing
// yields an empty seed rather than an error: the harvest query then falls back
// to whatever the other uploads produced.
func extractPDF(f io.ReaderAt, size int64) string {
	rd, err := pdf.NewReader(f, size)
	if err != nil {
		return ""
	}
	var b strings.Builder
	pages := rd.NumPage()
	for i := 1; i <= pages && i <= 3; i++ {
		p := rd.Page(i)
		if p.V.IsNull() {
			continue
		}
		txt, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		b.WriteString(txt)
	}
	return firstWords(b.String(), 400)
}
