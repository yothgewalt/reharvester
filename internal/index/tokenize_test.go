package index

import (
	"slices"
	"strings"
	"testing"
)

// The regression the paper documents: an extractor that drops stopwords and
// then bigrams the survivors reports "models llms" as the fastest-rising bigram
// in the corpus, an artefact of joining "...models." to "LLMs have...".
func TestNoCrossSentenceBigrams(t *testing.T) {
	got := Terms("We evaluate large language models. LLMs have improved retrieval.", 2)
	for _, bad := range []string{"models llms", "models llm"} {
		if slices.Contains(got, bad) {
			t.Errorf("formed cross-sentence bigram %q; got %v", bad, got)
		}
	}
	// The legitimate in-clause bigrams must survive.
	for _, want := range []string{"large language", "language models"} {
		if !slices.Contains(got, want) {
			t.Errorf("missing in-clause bigram %q; got %v", want, got)
		}
	}
}

// A stopword ends the run it sits in, so tokens either side of it were not
// adjacent and must not form a gram.
func TestStopwordBreaksRun(t *testing.T) {
	got := Terms("retrieval of documents", 2)
	if slices.Contains(got, "retrieval documents") {
		t.Errorf("bigram spans a removed stopword: %v", got)
	}
}

func TestHyphenStaysInsideToken(t *testing.T) {
	got := Terms("retrieval-augmented generation improves grounding", 2)
	for _, want := range []string{"retrieval-augmented", "retrieval-augmented generation"} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %q; got %v", want, got)
		}
	}
}

func TestLaTeXRejected(t *testing.T) {
	for _, in := range []string{
		`the \textbf{transformer} encoder`,
		`the textbf transformer encoder`,
		`energy $\alpha$ scaling of transformer encoder`,
	} {
		for _, tok := range Terms(in, 1) {
			if tok == "textbf" || tok == "alpha" {
				t.Errorf("LaTeX control word %q survived from %q", tok, in)
			}
		}
	}
}

func TestDropsBareNumbersAndShortTokens(t *testing.T) {
	for _, tok := range Terms("in 2023 we scaled to 8 GPUs", 1) {
		if tok == "2023" || tok == "8" {
			t.Errorf("bare number %q survived", tok)
		}
	}
}

func TestClauseBreakOnPunctuation(t *testing.T) {
	got := strings.Join(Terms("graph neural, language models", 2), "|")
	if strings.Contains(got, "neural language") {
		t.Errorf("comma did not break the run: %s", got)
	}
}
