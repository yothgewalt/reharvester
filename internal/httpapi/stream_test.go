package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func frameTypes(fs []json.RawMessage) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		var v struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(f, &v)
		out[i] = v.Type
	}
	return out
}

// A first attach gets the whole backlog: the client has seen nothing yet.
func TestFirstAttachReplaysEverything(t *testing.T) {
	h := NewHub()
	task := h.New("t1", "harvest")
	task.Log("info", "starting")
	task.Progress(10, "harvest")
	task.Delta([]GraphNode{{ID: "a"}}, nil)

	replay, live := task.attach()
	if live == nil {
		t.Fatal("an unfinished task must hand back a live channel")
	}
	got := frameTypes(replay)
	if len(got) != 3 || got[0] != "log" || got[1] != "graph_delta" || got[2] != "progress" {
		t.Errorf("first attach replay = %v, want log, graph_delta, progress", got)
	}
}

// The client dedupes nodes and edges by id but NOT log entries, so a reconnect
// must not replay the log backlog or the terminal prints the crawl twice.
func TestReattachReplaysDeltasNotLogs(t *testing.T) {
	h := NewHub()
	task := h.New("t2", "harvest")
	for range 5 {
		task.Log("info", "chatter")
	}
	task.Delta([]GraphNode{{ID: "a"}}, nil)
	task.Delta([]GraphNode{{ID: "b"}}, []GraphEdge{{ID: "a->b", Source: "a", Target: "b"}})
	task.Progress(50, "graph")

	first, live := task.attach()
	if n := count(frameTypes(first), "log"); n != 5 {
		t.Fatalf("first attach saw %d log frames, want 5", n)
	}
	task.detach(live)

	second, _ := task.attach()
	types := frameTypes(second)
	if n := count(types, "log"); n != 1 {
		t.Errorf("reattach replayed %d log frames, want exactly 1 (the reconnect notice)", n)
	}
	if n := count(types, "graph_delta"); n != 2 {
		t.Errorf("reattach replayed %d deltas, want both (they are idempotent)", n)
	}
	if n := count(types, "progress"); n != 1 {
		t.Errorf("reattach replayed %d progress frames, want the latest only", n)
	}
	if !strings.Contains(string(second[0]), "Reconnected") {
		t.Errorf("reconnect notice missing: %s", second[0])
	}
}

// Attaching to an already-finished task must deliver done and no live channel,
// so the client closes cleanly instead of treating it as a drop and retrying.
func TestAttachAfterDoneDeliversDoneAndNoChannel(t *testing.T) {
	h := NewHub()
	task := h.New("t3", "harvest")
	task.Log("info", "went by fast")
	task.Done(42, 1234*time.Millisecond)

	replay, live := task.attach()
	if live != nil {
		t.Error("a finished task must not hand back a live channel")
	}
	types := frameTypes(replay)
	if len(types) == 0 || types[len(types)-1] != "done" {
		t.Fatalf("replay must end with done, got %v", types)
	}
	var d doneFrame
	if err := json.Unmarshal(replay[len(replay)-1], &d); err != nil {
		t.Fatal(err)
	}
	if d.Summary.DocsIngested != 42 || d.Summary.DurationMs != 1234 {
		t.Errorf("done summary = %+v, want 42 docs / 1234 ms", d.Summary)
	}
}

// Frame shapes are what the client discriminates on; a wrong key is dropped
// silently, so they are pinned here.
func TestFrameShapes(t *testing.T) {
	h := NewHub()
	task := h.New("t4", "harvest")
	task.Log("warn", "rate limited")
	task.Progress(0, "harvest") // zero percent must still serialise
	replay, _ := task.attach()

	var lf struct {
		Type  string        `json:"type"`
		Entry CrawlLogEntry `json:"entry"`
	}
	if err := json.Unmarshal(replay[0], &lf); err != nil {
		t.Fatal(err)
	}
	if lf.Type != "log" || lf.Entry.Level != "warn" || lf.Entry.Message != "rate limited" ||
		lf.Entry.ID == "" || lf.Entry.Timestamp == "" {
		t.Errorf("log frame = %+v; every entry field must be populated", lf)
	}
	if !strings.Contains(string(replay[1]), `"percent":0`) {
		t.Errorf("zero percent was omitted from the progress frame: %s", replay[1])
	}
}

// An edge must never reach the client before its endpoints: the graph store
// drops any edge whose source or target it does not know.
func TestStreamSnapshotNeverEmitsEdgeBeforeNodes(t *testing.T) {
	var nodes []GraphNode
	for i := range 130 {
		nodes = append(nodes, GraphNode{ID: string(rune('a'+i%26)) + itoa(i)})
	}
	var edges []GraphEdge
	for i := 1; i < len(nodes); i++ {
		edges = append(edges, GraphEdge{
			ID: nodes[i-1].ID + "->" + nodes[i].ID, Source: nodes[i-1].ID, Target: nodes[i].ID,
		})
		// A backwards edge from an early node to a much later one: the ordering
		// bug this guards against.
		if i < 5 {
			edges = append(edges, GraphEdge{
				ID:     nodes[i].ID + "->" + nodes[len(nodes)-1].ID,
				Source: nodes[i].ID, Target: nodes[len(nodes)-1].ID,
			})
		}
	}
	h := NewHub()
	task := h.New("t5", "harvest")
	streamSnapshot(task, GraphSnapshot{Nodes: nodes, Edges: edges})
	replay, _ := task.attach()

	known := map[string]bool{}
	for _, f := range replay {
		var d deltaFrame
		if err := json.Unmarshal(f, &d); err != nil {
			t.Fatal(err)
		}
		for _, n := range d.AddedNodes {
			known[n.ID] = true
		}
		for _, e := range d.AddedEdges {
			if !known[e.Source] || !known[e.Target] {
				t.Fatalf("edge %s arrived before its endpoints", e.ID)
			}
		}
	}
}

func count(ss []string, want string) int {
	n := 0
	for _, s := range ss {
		if s == want {
			n++
		}
	}
	return n
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// The client calls .filter() on both delta arrays unconditionally. Go marshals a
// nil slice as null, which throws in the browser and loses the frame — invisible
// to a Go client, which decodes null into a nil slice happily.
func TestDeltaNeverSerialisesNullArrays(t *testing.T) {
	h := NewHub()
	task := h.New("t6", "harvest")
	task.Delta([]GraphNode{{ID: "a"}}, nil)                              // nodes only
	task.Delta(nil, []GraphEdge{{ID: "a->a", Source: "a", Target: "a"}}) // edges only
	replay, _ := task.attach()
	if len(replay) != 2 {
		t.Fatalf("want 2 delta frames, got %d", len(replay))
	}
	for i, f := range replay {
		s := string(f)
		if strings.Contains(s, `"addedNodes":null`) || strings.Contains(s, `"addedEdges":null`) {
			t.Errorf("frame %d serialised a null array: %s", i, s)
		}
		var d deltaFrame
		if err := json.Unmarshal(f, &d); err != nil {
			t.Fatal(err)
		}
		if d.AddedNodes == nil || d.AddedEdges == nil {
			t.Errorf("frame %d decoded to a nil slice: %s", i, s)
		}
	}
}
