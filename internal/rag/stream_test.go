package rag

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ndjson serves an Ollama-shaped streaming chat response: one JSON object per
// line rather than a single document.
func ndjson(t *testing.T, lines ...string) *Ollama {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		f, _ := w.(http.Flusher)
		for _, l := range lines {
			fmt.Fprintln(w, l)
			if f != nil {
				f.Flush()
			}
		}
	}))
	t.Cleanup(srv.Close)
	o := NewOllama(srv.URL, "", "")
	return o
}

func TestGenerateStreamAssemblesChunks(t *testing.T) {
	o := ndjson(t,
		`{"message":{"content":"Hello"},"done":false}`,
		`{"message":{"content":", "},"done":false}`,
		`{"message":{"content":"world"},"done":false}`,
		`{"message":{"content":""},"done":true}`,
	)
	var got []string
	answer, err := o.GenerateStream(context.Background(), "sys", "prompt", func(c string) {
		got = append(got, c)
	})
	if err != nil {
		t.Fatalf("GenerateStream: %v", err)
	}
	if answer != "Hello, world" {
		t.Errorf("answer = %q, want %q", answer, "Hello, world")
	}
	// Chunks must arrive separately and in order — that is the whole point.
	if len(got) != 3 {
		t.Fatalf("onToken called %d times, want 3: %q", len(got), got)
	}
	if strings.Join(got, "") != answer {
		t.Errorf("chunks %q do not reconstruct %q", got, answer)
	}
}

// TestGenerateStreamStopsAtDone: a server that keeps the connection open after
// reporting done must not leave the caller hanging.
func TestGenerateStreamStopsAtDone(t *testing.T) {
	o := ndjson(t,
		`{"message":{"content":"first"},"done":false}`,
		`{"message":{"content":""},"done":true}`,
		`{"message":{"content":"after done"},"done":false}`,
	)
	answer, err := o.GenerateStream(context.Background(), "s", "p", nil)
	if err != nil {
		t.Fatalf("GenerateStream: %v", err)
	}
	if answer != "first" {
		t.Errorf("answer = %q, want %q — content after done must be ignored", answer, "first")
	}
}

// TestGenerateStreamKeepsPartialOnError: tokens already delivered are on the
// reader's screen, so the caller gets them back alongside the error rather than
// an empty string.
func TestGenerateStreamKeepsPartialOnError(t *testing.T) {
	o := ndjson(t,
		`{"message":{"content":"partial answer"},"done":false}`,
		`{"error":"model unloaded"}`,
	)
	answer, err := o.GenerateStream(context.Background(), "s", "p", nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if answer != "partial answer" {
		t.Errorf("answer = %q, want the partial text preserved", answer)
	}
	if !strings.Contains(err.Error(), "model unloaded") {
		t.Errorf("error %v should carry the server's message", err)
	}
}

func TestGenerateStreamNilCallback(t *testing.T) {
	o := ndjson(t, `{"message":{"content":"ok"},"done":true}`)
	if _, err := o.GenerateStream(context.Background(), "s", "p", nil); err != nil {
		t.Fatalf("a nil onToken must be allowed: %v", err)
	}
}

func TestGenerateStreamHonoursCancellation(t *testing.T) {
	o := ndjson(t, `{"message":{"content":"x"},"done":false}`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := o.GenerateStream(ctx, "s", "p", nil); err == nil {
		t.Error("a cancelled context must abort the request")
	}
}

// TestAnswerPromptNumbersSources pins the citation contract: the system prompt
// asks for [1], [2] and so on, which only works if the sources are numbered.
func TestAnswerPromptNumbersSources(t *testing.T) {
	docs := []ContextDoc{
		{Title: "First paper", Abstract: "alpha"},
		{Title: "Second paper", Abstract: "beta"},
	}
	got := answerPrompt("what is alpha?", docs)
	for _, want := range []string{"[1] First paper", "alpha", "[2] Second paper", "beta", "Question: what is alpha?"} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "[1]") > strings.Index(got, "[2]") {
		t.Error("sources must be numbered in order")
	}
}
