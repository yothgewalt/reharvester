package httpapi

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// maxBufferedLogs matches the client's own log cap.
const maxBufferedLogs = 2000

// Task is one crawl or project action, and the buffer of frames its stream has
// produced. The client reconnects to the same task id up to six times on any
// close it did not see a "done" frame before, so a task must survive its socket
// and be re-attachable.
type Task struct {
	ID     string
	Action string

	mu       sync.Mutex
	logs     []json.RawMessage
	deltas   []json.RawMessage
	progress json.RawMessage
	done     json.RawMessage
	subs     map[chan json.RawMessage]struct{}
	attached bool

	seq atomic.Int64
}

type Hub struct {
	mu    sync.Mutex
	tasks map[string]*Task
}

func NewHub() *Hub { return &Hub{tasks: map[string]*Task{}} }

func (h *Hub) New(id, action string) *Task {
	h.mu.Lock()
	defer h.mu.Unlock()
	t := &Task{ID: id, Action: action, subs: map[chan json.RawMessage]struct{}{}}
	h.tasks[id] = t
	return t
}

func (h *Hub) Get(id string) *Task {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.tasks[id]
}

func (t *Task) publish(frame any, keep func(json.RawMessage)) {
	b, err := json.Marshal(frame)
	if err != nil {
		return
	}
	t.mu.Lock()
	if keep != nil {
		keep(b)
	}
	subs := make([]chan json.RawMessage, 0, len(t.subs))
	for c := range t.subs {
		subs = append(subs, c)
	}
	t.mu.Unlock()
	for _, c := range subs {
		// Never block the pipeline on a slow reader; a dropped frame is
		// recovered by the replay on reconnect.
		select {
		case c <- b:
		default:
		}
	}
}

func (t *Task) Log(level, msg string) {
	entry := CrawlLogEntry{
		ID:        fmt.Sprintf("%s-%d", t.ID, t.seq.Add(1)),
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level,
		Message:   msg,
	}
	t.publish(logFrame{Type: "log", Entry: entry}, func(b json.RawMessage) {
		t.logs = append(t.logs, b)
		if len(t.logs) > maxBufferedLogs {
			t.logs = t.logs[len(t.logs)-maxBufferedLogs:]
		}
	})
}

func (t *Task) Progress(percent int, stage string) {
	t.publish(progressFrame{Type: "progress", Percent: percent, Stage: stage},
		func(b json.RawMessage) { t.progress = b })
}

// Delta must never reference a node the client has not seen: the client drops
// any edge whose endpoints are unknown, so nodes always precede the edges that
// need them.
//
// Both slices are coerced to empty rather than left nil. Go marshals a nil slice
// as JSON null, and the client calls .filter() on both fields unconditionally,
// so a null throws inside its stream handler and the whole frame is lost. A Go
// client decodes null into a nil slice without complaint, which is why this is
// only visible from a browser.
func (t *Task) Delta(nodes []GraphNode, edges []GraphEdge) {
	if len(nodes) == 0 && len(edges) == 0 {
		return
	}
	if nodes == nil {
		nodes = []GraphNode{}
	}
	if edges == nil {
		edges = []GraphEdge{}
	}
	t.publish(deltaFrame{Type: "graph_delta", AddedNodes: nodes, AddedEdges: edges},
		func(b json.RawMessage) { t.deltas = append(t.deltas, b) })
}

// Done must be sent before the socket closes. A close the client did not see a
// done frame before marks the project failed and triggers a reconnect.
func (t *Task) Done(docs int, dur time.Duration) {
	t.publish(doneFrame{Type: "done", Summary: doneSummary{
		DocsIngested: docs, DurationMs: dur.Milliseconds(),
	}}, func(b json.RawMessage) { t.done = b })
}

func (t *Task) Finished() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.done != nil
}

// attach returns the frames a newly connected client should receive before live
// streaming, plus its subscription channel.
//
// Replay policy is asymmetric on purpose. The client dedupes nodes and edges by
// id, so replaying every delta is free and guarantees the graph is complete.
// It does NOT dedupe log entries, so replaying the log backlog to a reconnecting
// client would print the whole crawl twice; a reconnect gets one line saying so
// instead.
func (t *Task) attach() ([]json.RawMessage, chan json.RawMessage) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var replay []json.RawMessage
	if !t.attached {
		replay = append(replay, t.logs...)
		t.attached = true
	} else {
		if b, err := json.Marshal(logFrame{Type: "log", Entry: CrawlLogEntry{
			ID:        fmt.Sprintf("%s-reattach-%d", t.ID, t.seq.Add(1)),
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Level:     "info",
			Message:   "Reconnected to the running harvest — replaying graph state",
		}}); err == nil {
			replay = append(replay, b)
		}
	}
	replay = append(replay, t.deltas...)
	if t.progress != nil {
		replay = append(replay, t.progress)
	}
	if t.done != nil {
		replay = append(replay, t.done)
		return replay, nil // finished: nothing left to stream
	}
	c := make(chan json.RawMessage, 256)
	t.subs[c] = struct{}{}
	return replay, c
}

func (t *Task) detach(c chan json.RawMessage) {
	if c == nil {
		return
	}
	t.mu.Lock()
	delete(t.subs, c)
	t.mu.Unlock()
}
