package rag

import (
	"strings"
	"testing"
)

func TestClassifyRequest(t *testing.T) {
	tests := []struct {
		name string
		q    string
		want Guard
	}{
		{"run a harvest", "Run a harvest on quantum computing", GuardExecute},
		{"how question mentioning run", "How do I run a harvest?", GuardNone},
		{"what should I harvest", "What should I harvest next?", GuardNone},
		{"which papers", "Which papers discuss execution time?", GuardNone},
		{"please delete", "please delete this project", GuardExecute},
		{"can you download", "can you download the PDFs", GuardExecute},
		{"rm -rf", "rm -rf /", GuardExecute},
		{"fenced code block", "```bash\nls\n```", GuardExecute},
		{"why question", "Why is this gap under-connected?", GuardNone},
		{"is question", "Is this topic emerging?", GuardNone},
		{"are question", "Are there papers on this?", GuardNone},
		{"does question", "Does this corpus cover LLMs?", GuardNone},
		{"where question", "Where are the research gaps?", GuardNone},
		{"when question", "When did this trend start?", GuardNone},
		{"who question", "Who wrote the most-cited paper?", GuardNone},
		{"empty", "", GuardNone},
		{"crawl command", "crawl arxiv for new papers", GuardExecute},
		{"could you modal", "could you fetch the latest papers", GuardExecute},
		{"ordinary question", "What are the main research areas here?", GuardNone},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyRequest(tc.q); got != tc.want {
				t.Errorf("ClassifyRequest(%q) = %v, want %v", tc.q, got, tc.want)
			}
		})
	}
}

func TestExecuteRefusalIsFixed(t *testing.T) {
	a := ExecuteRefusal("run a harvest")
	b := ExecuteRefusal("delete everything")
	if a != b {
		t.Errorf("ExecuteRefusal must not vary with the request: %q vs %q", a, b)
	}
	if !strings.Contains(a, "Harvest") || !strings.Contains(a, "Settings") {
		t.Errorf("refusal should name the pages that do it: %q", a)
	}
}

func TestBuildChatMessagesCapsHistoryAndOrdersTurns(t *testing.T) {
	var history []Message
	for i := range 10 {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		history = append(history, Message{Role: role, Content: "turn"})
	}
	docs := []ContextDoc{
		{Title: "First paper", Abstract: "alpha"},
		{Title: "Second paper", Abstract: "beta"},
	}
	msgs := BuildChatMessages("overview text", docs, history, "what is alpha?")

	if msgs[0].Role != "system" || msgs[0].Content != chatSystem {
		t.Fatalf("first message must be the fixed system prompt")
	}
	// system + at most 6 prior turns + the final user turn.
	if len(msgs) != 1+6+1 {
		t.Fatalf("len(msgs) = %d, want %d", len(msgs), 8)
	}
	if !equalMessages(msgs[1:7], history[len(history)-6:]) {
		t.Errorf("history was not trimmed to the last 6 turns: %+v", msgs[1:7])
	}

	last := msgs[len(msgs)-1]
	if last.Role != "user" {
		t.Fatalf("the final turn must be from the user, got %q", last.Role)
	}
	for _, want := range []string{"Corpus overview:", "overview text", "[1] First paper", "alpha", "[2] Second paper", "beta", "Question: what is alpha?"} {
		if !strings.Contains(last.Content, want) {
			t.Errorf("final turn missing %q:\n%s", want, last.Content)
		}
	}
	if strings.Index(last.Content, "[1]") > strings.Index(last.Content, "[2]") {
		t.Error("sources must stay numbered in order")
	}
	if strings.Index(last.Content, "Sources:") > strings.Index(last.Content, "Question:") {
		t.Error("the question must come after the sources, not before")
	}
}

func TestBuildChatMessagesKeepsShortHistoryWhole(t *testing.T) {
	history := []Message{{Role: "user", Content: "hi"}, {Role: "assistant", Content: "hello"}}
	msgs := BuildChatMessages("", nil, history, "anything else?")
	if len(msgs) != 1+2+1 {
		t.Fatalf("len(msgs) = %d, want 4", len(msgs))
	}
	if !equalMessages(msgs[1:3], history) {
		t.Errorf("short history should not be trimmed: %+v", msgs[1:3])
	}
}

func equalMessages(a, b []Message) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
