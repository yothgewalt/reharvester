package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yothgewalt/reharvester/internal/analyze"
	"github.com/yothgewalt/reharvester/internal/graph"
	"github.com/yothgewalt/reharvester/internal/paper"
	"github.com/yothgewalt/reharvester/internal/pipeline"
	"github.com/yothgewalt/reharvester/internal/store"
)

// chatFixtureProject builds a small but real project: a lexical index and a
// BM25-capable retrieve.Engine over two clearly separated topics, one early
// (gravitational lensing) and one recent (dataset distillation), so a
// same-topic query retrieves, an off-topic query retrieves nothing, and the
// trend window has a genuine riser and decliner to describe.
func chatFixtureProject(t *testing.T) *pipeline.Project {
	t.Helper()
	var papers []paper.Paper
	for i := range 10 {
		papers = append(papers, paper.Paper{
			ID:        fmt.Sprintf("lens.%d", i),
			Title:     fmt.Sprintf("Gravitational Lensing Study %d", i),
			Abstract:  "Gravitational lensing surveys measure dark matter halo mass through shear estimation across the sky.",
			Published: time.Date(2019+i%3, 1, 1, 0, 0, 0, 0, time.UTC),
		})
	}
	for i := range 10 {
		papers = append(papers, paper.Paper{
			ID:        fmt.Sprintf("dd.%d", i),
			Title:     fmt.Sprintf("Dataset Distillation Method %d", i),
			Abstract:  "Dataset distillation compresses training data into a small synthetic set for vision models.",
			Published: time.Date(2022+i%3, 1, 1, 0, 0, 0, 0, time.UTC),
		})
	}
	community := make([]int32, len(papers))
	for i := 10; i < len(papers); i++ {
		community[i] = 1
	}

	corpus := pipeline.BuildLexical(papers, pipeline.DefaultBuildOptions())
	g := &graph.Graph{
		N: len(papers), Community: community,
		PageRank: make([]float64, len(papers)), Bridge: make([]float64, len(papers)),
		Communities: []graph.Community{
			{ID: 0, Label: "Gravitational Lensing", Size: 10},
			{ID: 1, Label: "Dataset Distillation", Size: 10},
		},
	}
	g.Index()

	p := &pipeline.Project{Papers: papers, Corpus: corpus, Graph: g}
	p.Engine = corpus.Engine(g)
	p.Analytics = analyze.BuildAnalytics(papers, community, 2, 2)
	p.Gaps = analyze.GapReport{Pairs: []analyze.GapPair{
		{A: 0, B: 1, LabelA: "Gravitational Lensing", LabelB: "Dataset Distillation", Similarity: 0.12, Z: -3.4},
	}}
	return p
}

// sseEvent is one parsed "event: ...\ndata: ...\n\n" frame.
type sseEvent struct {
	name string
	data map[string]any
}

func parseSSE(t *testing.T, body string) []sseEvent {
	t.Helper()
	var out []sseEvent
	for _, chunk := range strings.Split(strings.TrimSpace(body), "\n\n") {
		if chunk == "" {
			continue
		}
		var ev sseEvent
		for _, line := range strings.Split(chunk, "\n") {
			switch {
			case strings.HasPrefix(line, "event: "):
				ev.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev.data); err != nil {
					t.Fatalf("bad SSE data line %q: %v", line, err)
				}
			}
		}
		out = append(out, ev)
	}
	return out
}

func findEvent(events []sseEvent, name string) (sseEvent, bool) {
	for _, e := range events {
		if e.name == name {
			return e, true
		}
	}
	return sseEvent{}, false
}

func postChatStream(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/stream", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.chatStream(w, req)
	return w
}

func TestChatStreamValidation(t *testing.T) {
	longContent := strings.Repeat("a", 4001)
	manyMessages := `{"messages":[`
	for i := range 41 {
		if i > 0 {
			manyMessages += ","
		}
		manyMessages += `{"role":"user","content":"hi"}`
	}
	manyMessages += `]}`

	tests := []struct {
		name string
		body string
	}{
		{"invalid JSON", "{not json"},
		{"zero messages", `{"messages":[]}`},
		{"too many messages", manyMessages},
		{"bad role", `{"messages":[{"role":"system","content":"hi"}]}`},
		{"last message not user", `{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello"}]}`},
		{"empty content", `{"messages":[{"role":"user","content":"   "}]}`},
		{"content too long", `{"messages":[{"role":"user","content":"` + longContent + `"}]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(t, false)
			w := postChatStream(t, s, tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400, body: %s", w.Code, w.Body.String())
			}
			errs := decodeErrors(t, w.Body.Bytes())
			if errs["messages"] == "" && errs["_"] == "" {
				t.Errorf("errs = %v, want a message under \"messages\" or \"_\"", errs)
			}
		})
	}
}

func TestChatStreamNoActiveProjectIs409(t *testing.T) {
	s := newTestServer(t, false)
	w := postChatStream(t, s, `{"messages":[{"role":"user","content":"What are the gaps?"}]}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body: %s", w.Code, w.Body.String())
	}
	errs := decodeErrors(t, w.Body.Bytes())
	if errs["_"] == "" {
		t.Errorf("errs = %v, want a message under \"_\"", errs)
	}
}

// TestChatStreamExecuteGuardSkipsRetrievalAndModel pins the guardrail: an
// execute command gets an empty source list and the fixed refusal, and never
// reaches retrieval or generation.
func TestChatStreamExecuteGuardSkipsRetrievalAndModel(t *testing.T) {
	s := newTestServer(t, false)
	s.setActive("proj1", chatFixtureProject(t))

	w := postChatStream(t, s, `{"messages":[{"role":"user","content":"Run a harvest on quantum computing"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	events := parseSSE(t, w.Body.String())
	if len(events) != 2 {
		t.Fatalf("events = %+v, want exactly sources then done (no token, no retrieval)", events)
	}

	sources, ok := findEvent(events, "sources")
	if !ok {
		t.Fatal("missing sources event")
	}
	if list, _ := sources.data["sources"].([]any); len(list) != 0 {
		t.Errorf("sources = %v, want an empty list", sources.data["sources"])
	}
	if sources.data["project"] == nil || sources.data["tier"] == nil {
		t.Errorf("sources event must still carry tier/project: %+v", sources.data)
	}

	done, ok := findEvent(events, "done")
	if !ok {
		t.Fatal("missing done event")
	}
	if done.data["guard"] != "execute" {
		t.Errorf("guard = %v, want \"execute\"", done.data["guard"])
	}
	if done.data["generated"] != false {
		t.Errorf("generated = %v, want false", done.data["generated"])
	}
	answer, _ := done.data["answer"].(string)
	if !strings.Contains(answer, "can't run harvests") {
		t.Errorf("answer = %q, want the fixed refusal", answer)
	}
}

// TestChatStreamNoMatchGuard pins the second guardrail: a question with no
// lexical overlap and no overview intent gets a not-in-corpus notice instead
// of an empty-context model call.
func TestChatStreamNoMatchGuard(t *testing.T) {
	s := newTestServer(t, false)
	s.setActive("proj1", chatFixtureProject(t))

	w := postChatStream(t, s, `{"messages":[{"role":"user","content":"What is the capital of France?"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	events := parseSSE(t, w.Body.String())

	sources, ok := findEvent(events, "sources")
	if !ok {
		t.Fatal("missing sources event")
	}
	if list, _ := sources.data["sources"].([]any); len(list) != 0 {
		t.Errorf("sources = %v, want an empty list", sources.data["sources"])
	}

	done, ok := findEvent(events, "done")
	if !ok {
		t.Fatal("missing done event")
	}
	if done.data["guard"] != "no-match" {
		t.Errorf("guard = %v, want \"no-match\"", done.data["guard"])
	}
	answer, _ := done.data["answer"].(string)
	if !strings.Contains(answer, "Nothing in") || !strings.Contains(answer, "Harvest page") {
		t.Errorf("answer = %q, want the not-in-corpus notice", answer)
	}
}

// TestChatStreamRetrievesOnTopicSources is the positive control for the
// no-match guard: a question the corpus actually covers must retrieve real
// sources rather than tripping the guard.
func TestChatStreamRetrievesOnTopicSources(t *testing.T) {
	s := newTestServer(t, false)
	s.setActive("proj1", chatFixtureProject(t))

	w := postChatStream(t, s, `{"messages":[{"role":"user","content":"What papers discuss gravitational lensing shear estimation?"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	events := parseSSE(t, w.Body.String())

	sources, ok := findEvent(events, "sources")
	if !ok {
		t.Fatal("missing sources event")
	}
	list, _ := sources.data["sources"].([]any)
	if len(list) == 0 {
		t.Fatalf("sources = %v, want retrieved documents for an on-topic question", sources.data["sources"])
	}

	done, ok := findEvent(events, "done")
	if !ok {
		t.Fatal("missing done event")
	}
	if _, guarded := done.data["guard"]; guarded {
		t.Errorf("done = %+v, an on-topic question must not carry a guard", done.data)
	}
}

func TestOverviewOfContainsGapLabelsAndRisingTerms(t *testing.T) {
	p := chatFixtureProject(t)
	got := overviewOf(p, store.Meta{Name: "Dark Matter Survey", Query: "dark matter"})

	for _, want := range []string{
		"Dark Matter Survey", "dark matter", "20 papers",
		"Gravitational Lensing", "Dataset Distillation",
		"distillation",
	} {
		if !strings.Contains(strings.ToLower(got), strings.ToLower(want)) {
			t.Errorf("overview missing %q:\n%s", want, got)
		}
	}
}

// A previous question only feeds retrieval when an answer followed it: a
// refused or dropped turn must not lend its words to the next question, or the
// no-match guard stops firing.
func TestChatRetrievalQueryUsesOnlyAnsweredFollowUps(t *testing.T) {
	u := func(c string) chatMessage { return chatMessage{Role: "user", Content: c} }
	a := func(c string) chatMessage { return chatMessage{Role: "assistant", Content: c} }
	for _, tc := range []struct {
		name string
		msgs []chatMessage
		want string
	}{
		{"single", []chatMessage{u("q1")}, "q1"},
		{"answered follow-up", []chatMessage{u("q1"), a("a1"), u("more?")}, "q1\nmore?"},
		{"unanswered previous", []chatMessage{u("run a harvest"), u("capital of france")}, "capital of france"},
		{"only latest answered pair", []chatMessage{u("q1"), a("a1"), u("q2"), a("a2"), u("more?")}, "q2\nmore?"},
	} {
		if got := chatRetrievalQuery(tc.msgs); got != tc.want {
			t.Errorf("%s: chatRetrievalQuery = %q, want %q", tc.name, got, tc.want)
		}
	}
}
