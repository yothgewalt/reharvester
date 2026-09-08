package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// logSink captures everything written to the standard log package so the
// Console can show it. The TUI owns the terminal, so log output must never
// reach stdout directly — it would corrupt the rendered frame.
//
// Writes never block: when the TUI is slow or absent, lines are dropped from
// the channel rather than stalling the server that produced them. The ring
// buffer keeps the recent history a newly-opened Console needs.
type logSink struct {
	mu    sync.Mutex
	ring  []string
	max   int
	ch    chan string
	file  *os.File
	stamp bool
}

const logRing = 2000

// newLogSink tees into <dataDir>/logs/<name>.log when it can, so a user can
// attach a file to a bug report. A file that cannot be opened is not an error:
// the in-memory Console still works.
func newLogSink(dataDir, name string) *logSink {
	s := &logSink{max: logRing, ch: make(chan string, 256), stamp: true}
	dir := filepath.Join(dataDir, "logs")
	if os.MkdirAll(dir, 0o755) == nil {
		if f, err := os.OpenFile(filepath.Join(dir, name+".log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			s.file = f
		}
	}
	return s
}

// Write satisfies io.Writer for log.SetOutput. One call may carry several
// newline-separated lines; each becomes its own Console row.
func (s *logSink) Write(p []byte) (int, error) {
	if s.file != nil {
		_, _ = s.file.Write(p)
	}
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if line == "" {
			continue
		}
		s.push(line)
	}
	return len(p), nil
}

// Push adds a line the TUI generated itself, so command output and server
// output interleave in one place.
func (s *logSink) Push(line string) { s.push(line) }

func (s *logSink) push(line string) {
	if s.stamp && !strings.HasPrefix(line, "20") {
		line = time.Now().Format("15:04:05") + " " + line
	}
	s.mu.Lock()
	s.ring = append(s.ring, line)
	if len(s.ring) > s.max {
		s.ring = s.ring[len(s.ring)-s.max:]
	}
	s.mu.Unlock()

	select {
	case s.ch <- line:
	default: // Console is behind; the ring still holds it.
	}
}

// Lines returns the buffered history, newest last.
func (s *logSink) Lines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.ring))
	copy(out, s.ring)
	return out
}

// Path names the tee target, or "" when none was opened.
func (s *logSink) Path() string {
	if s.file == nil {
		return ""
	}
	return s.file.Name()
}

func (s *logSink) Close() {
	if s.file != nil {
		_ = s.file.Close()
	}
}
