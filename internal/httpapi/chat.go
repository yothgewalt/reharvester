package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/yothgewalt/reharvester/internal/pipeline"
	"github.com/yothgewalt/reharvester/internal/rag"
	"github.com/yothgewalt/reharvester/internal/retrieve"
	"github.com/yothgewalt/reharvester/internal/store"
)

// Limits on a chat turn: generous enough for a real question, small enough
// that the body can be read whole before any validation runs.
const (
	chatMaxBodyBytes  = 64 * 1024
	chatMaxMessages   = 40
	chatMaxContentLen = 4000
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Messages    []chatMessage `json:"messages"`
	PinnedDocID string        `json:"pinnedDocId"`
}

// chatStream answers POST /api/v1/chat/stream over Server-Sent Events: a
// read-only, multi-turn assistant grounded in the active project. Framing
// matches askStream — "sources", "token", "done", "error" — but chat adds two
// guardrails a one-shot question never needed: ClassifyRequest refuses a
// command outright, and a no-match check says so when nothing in the corpus
// is relevant, rather than letting the model guess.
func (s *Server) chatStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, chatMaxBodyBytes)
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrors(w, map[string]string{"_": "request body must be valid JSON no larger than 64KB"})
		return
	}
	if errs := validateChatRequest(req); errs != nil {
		writeErrors(w, errs)
		return
	}

	p, snaps := s.active()
	if p == nil || snaps == nil {
		writeErrorsStatus(w, http.StatusConflict, map[string]string{"_": "No project is loaded — open one from Projects"})
		return
	}

	s.mu.RLock()
	activeID := s.activeID
	s.mu.RUnlock()
	meta := store.Meta{ID: activeID, Name: activeID}
	if sp, ok := s.store.ExistingProject(activeID); ok {
		var m store.Meta
		if sp.LoadJSON("meta.json", &m) == nil {
			meta = m
		}
	}
	projectInfo := map[string]string{"id": activeID, "name": meta.Name}

	writeSSEHeaders(w)
	send := sseSender(w, flusher)
	last := req.Messages[len(req.Messages)-1].Content

	if rag.ClassifyRequest(last) == rag.GuardExecute {
		send("sources", map[string]any{
			"sources": []askSource{}, "tier": string(p.Tier()),
			"budget": rag.DefaultBudgetWords, "hops": 1, "project": projectInfo,
		})
		send("done", map[string]any{
			"answer": rag.ExecuteRefusal(last), "generated": false, "guard": "execute",
		})
		return
	}

	titles := paperTitles(p)
	query := chatRetrievalQuery(req.Messages)
	docs := rag.AssembleContext(p.Engine, titles, p.Corpus.Abstracts, query, rag.DefaultBudgetWords, 1)

	pinned := false
	if req.PinnedDocID != "" {
		if pos, ok := snaps.PosByDocID[req.PinnedDocID]; ok {
			pinned = true
			docs = pinDoc(docs, pos, titles[pos], p.Corpus.Abstracts[pos], rag.DefaultBudgetWords)
		}
	}

	if !pinned && !hasOverviewIntent(last) && len(p.Engine.Search(retrieve.BM25, query, 1)) == 0 {
		send("sources", map[string]any{
			"sources": []askSource{}, "tier": string(p.Tier()),
			"budget": rag.DefaultBudgetWords, "hops": 1, "project": projectInfo,
		})
		send("done", map[string]any{
			"answer":    fmt.Sprintf("Nothing in %s matches this question. Try different wording, or harvest papers on this topic from the Harvest page.", meta.Name),
			"generated": false, "guard": "no-match",
		})
		return
	}

	sources := make([]askSource, 0, len(docs))
	for _, d := range docs {
		sources = append(sources, askSource{
			DocID: snaps.DocIDByPos[d.Pos], Title: d.Title, ViaGraph: d.ViaGraph, Words: d.Words,
		})
	}
	send("sources", map[string]any{
		"sources": sources, "tier": string(p.Tier()),
		"budget": rag.DefaultBudgetWords, "hops": 1, "project": projectInfo,
	})

	llm := s.cfgNow().llm
	if llm == nil {
		send("done", map[string]any{"answer": noModelAnswer, "generated": false})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), askStreamTimeout)
	defer cancel()

	msgs := rag.BuildChatMessages(overviewOf(p, meta), docs, chatHistory(req.Messages), last)
	answer, err := llm.ChatStream(ctx, msgs, func(chunk string) { send("token", map[string]string{"text": chunk}) })

	// A partial answer plus an error still beats discarding what arrived: the
	// reader has already seen those tokens on screen.
	if err != nil && answer == "" {
		log.Printf("api: chat stream: %v", err)
		send("error", map[string]string{"message": "generation failed — the sources above are the assembled context"})
		return
	}
	if err != nil {
		log.Printf("api: chat stream ended early: %v", err)
	}
	send("done", map[string]any{"answer": answer, "generated": true, "truncated": err != nil})
}

// validateChatRequest checks the shape ClassifyRequest and retrieval both
// assume: at least one message and no more than chatMaxMessages, every role
// either "user" or "assistant", every content non-empty and within
// chatMaxContentLen runes, and the conversation ending on a user turn.
func validateChatRequest(req chatRequest) map[string]string {
	if n := len(req.Messages); n == 0 || n > chatMaxMessages {
		return map[string]string{"messages": fmt.Sprintf("must contain between 1 and %d messages", chatMaxMessages)}
	}
	for i, m := range req.Messages {
		if m.Role != "user" && m.Role != "assistant" {
			return map[string]string{"messages": fmt.Sprintf("message %d: role must be \"user\" or \"assistant\"", i)}
		}
		n := utf8.RuneCountInString(m.Content)
		if strings.TrimSpace(m.Content) == "" || n > chatMaxContentLen {
			return map[string]string{"messages": fmt.Sprintf("message %d: content must be 1-%d characters", i, chatMaxContentLen)}
		}
	}
	if req.Messages[len(req.Messages)-1].Role != "user" {
		return map[string]string{"messages": "the last message must be from the user"}
	}
	return nil
}

// chatRetrievalQuery folds the last user turn and the previous one into one
// retrieval query, so a follow-up like "tell me more about the first one"
// still carries the noun phrases that made the previous turn findable. The
// previous turn counts only when an assistant answer came between the two: an
// unanswered one (refused, dropped) must not make an off-corpus question match.
func chatRetrievalQuery(msgs []chatMessage) string {
	last := msgs[len(msgs)-1].Content
	answered := false
	for i := len(msgs) - 2; i >= 0; i-- {
		switch msgs[i].Role {
		case "assistant":
			answered = true
		case "user":
			if answered {
				return msgs[i].Content + "\n" + last
			}
			return last
		}
	}
	return last
}

// chatHistory converts every turn but the last into rag.Message, for
// BuildChatMessages to cap at its own last-6 window.
func chatHistory(msgs []chatMessage) []rag.Message {
	if len(msgs) <= 1 {
		return nil
	}
	out := make([]rag.Message, len(msgs)-1)
	for i, m := range msgs[:len(msgs)-1] {
		out[i] = rag.Message{Role: m.Role, Content: m.Content}
	}
	return out
}

// pinDoc gives a pinned document first place in the context set, dropping any
// duplicate the retrieval pass already picked, then re-admits the rest in
// their existing order until the budget is spent — so the pin always counts
// toward it rather than riding for free on top.
func pinDoc(docs []rag.ContextDoc, pos int, title, abstract string, budget int) []rag.ContextDoc {
	pinned := rag.ContextDoc{Pos: pos, Title: title, Abstract: abstract, Words: len(strings.Fields(abstract))}
	out := make([]rag.ContextDoc, 0, len(docs)+1)
	out = append(out, pinned)
	spent := pinned.Words
	for _, d := range docs {
		if d.Pos == pos {
			continue
		}
		if spent+d.Words > budget {
			continue // a shorter document later may still fit
		}
		spent += d.Words
		out = append(out, d)
	}
	return out
}

// overviewIntentWords are terms that mark a question as being about the
// corpus as a whole rather than a specific paper, so the no-match guard does
// not fire on "what are the emerging areas here?" just because no single
// document matches those words well.
var overviewIntentWords = []string{
	"gap", "trend", "emerging", "communit", "topic", "area",
	"overview", "summary", "corpus", "this project",
}

func hasOverviewIntent(q string) bool {
	lower := strings.ToLower(q)
	for _, w := range overviewIntentWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// overviewOf renders the corpus-level context a chat turn needs alongside its
// retrieved sources: project identity and size, the largest communities, the
// sharpest gap pairs, and recent term movement. It stays near 400 words —
// this text rides in every model call, on top of the sources.
func overviewOf(p *pipeline.Project, meta store.Meta) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Project %q", meta.Name)
	if meta.Query != "" {
		fmt.Fprintf(&b, " (harvested for query %q)", meta.Query)
	}
	fmt.Fprintf(&b, ": %d papers", len(p.Papers))
	if p.Analytics != nil && len(p.Analytics.Years) > 0 {
		years := p.Analytics.Years
		fmt.Fprintf(&b, ", %d-%d", years[0], years[len(years)-1])
	}
	b.WriteString(".\n\n")

	if p.Graph != nil && len(p.Graph.Communities) > 0 {
		idx := make([]int, len(p.Graph.Communities))
		for i := range idx {
			idx[i] = i
		}
		sort.Slice(idx, func(i, j int) bool {
			return p.Graph.Communities[idx[i]].Size > p.Graph.Communities[idx[j]].Size
		})
		if len(idx) > 12 {
			idx = idx[:12]
		}
		b.WriteString("Top research communities:\n")
		for _, i := range idx {
			c := p.Graph.Communities[i]
			fmt.Fprintf(&b, "- %s (%d papers)\n", c.Label, c.Size)
		}
		b.WriteString("\n")
	}

	if len(p.Gaps.Pairs) > 0 {
		b.WriteString("Top research gaps (under-connected community pairs). How to read the figures: " +
			"similarity is how alike the two communities' papers read (0-1, higher = closer topics); " +
			"z compares their cross-links with chance, so a negative z means FEWER links than expected " +
			"and the more negative, the wider and more promising the gap:\n")
		for i, g := range p.Gaps.Pairs {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "- %s <-> %s (similarity %.2f, z %.2f)\n", g.LabelA, g.LabelB, g.Similarity, g.Z)
		}
		b.WriteString("\n")
	}

	if p.Analytics != nil && len(p.Analytics.Years) > 0 {
		years := p.Analytics.Years
		end := years[len(years)-1]
		start := end - 2
		if start < years[0] {
			start = years[0]
		}
		var rising, declining []string
		for _, t := range trendKeywords(p, start, end) {
			switch {
			case t.Direction == "rising" && len(rising) < 7:
				rising = append(rising, t.Keyword)
			case t.Direction == "declining" && len(declining) < 5:
				declining = append(declining, t.Keyword)
			}
		}
		if len(rising) > 0 {
			b.WriteString("Rising terms (last 3 years vs the 3 before): " + strings.Join(rising, ", ") + "\n")
		}
		if len(declining) > 0 {
			b.WriteString("Declining terms: " + strings.Join(declining, ", ") + "\n")
		}
	}

	return b.String()
}
