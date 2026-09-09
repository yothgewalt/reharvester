package pipeline

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yothgewalt/reharvester/internal/graph"
	"github.com/yothgewalt/reharvester/internal/paper"
	"github.com/yothgewalt/reharvester/internal/store"
)

// corpus builds n papers on one of two topics, with ids drawn from the given
// prefix so two corpora of identical size are still distinguishable.
func corpus(prefix, topic string, n int) []paper.Paper {
	vocab := map[string][]string{
		"aero": {"aerodynamic", "airfoil", "turbulence", "wing", "lift", "supersonic", "flow", "thrust"},
		"nlp":  {"language", "transformer", "retrieval", "embedding", "corpus", "token", "attention", "decoder"},
	}[topic]
	out := make([]paper.Paper, n)
	for i := range n {
		var b strings.Builder
		for j, w := range vocab {
			fmt.Fprintf(&b, "%s %s%d ", w, w, (i+j)%4)
		}
		out[i] = paper.Paper{
			ID:         fmt.Sprintf("%s.%04d", prefix, i),
			Title:      fmt.Sprintf("%s study %d", topic, i),
			Abstract:   strings.Repeat(b.String(), 4),
			Authors:    []string{fmt.Sprintf("Author %d", i%7)},
			Categories: []string{"eess.SY"},
			Published:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}
	}
	return out
}

func newProject(t *testing.T) *store.Project {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sp, err := st.Project("p")
	if err != nil {
		t.Fatal(err)
	}
	return sp
}

func TestFingerprintDistinguishesCorporaOfEqualSize(t *testing.T) {
	a := corpus("aero", "aero", 40)
	b := corpus("nlp", "nlp", 40)
	if graph.Fingerprint(a) == graph.Fingerprint(b) {
		t.Error("two different 40-paper corpora share a fingerprint")
	}
	if graph.Fingerprint(a) != graph.Fingerprint(a) {
		t.Error("fingerprint is not stable across calls")
	}
	shuffled := append([]paper.Paper{}, a[20:]...)
	shuffled = append(shuffled, a[:20]...)
	if graph.Fingerprint(a) != graph.Fingerprint(shuffled) {
		t.Error("fingerprint changed with paper order; it must not")
	}
}

func TestLoadRebuildsWhenTheCorpusChangesButTheCountDoesNot(t *testing.T) {
	// This is the live hazard: the harvest cap makes every corpus exactly the
	// same size, so the old paper-count check happily served the previous
	// corpus's communities after a re-harvest into the same project.
	ctx := context.Background()
	sp := newProject(t)

	first := corpus("aero", "aero", 40)
	if err := sp.WritePapers(first); err != nil {
		t.Fatal(err)
	}
	built, err := Build(ctx, sp, first, nil, false, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if built.Graph.Fingerprint != graph.Fingerprint(first) {
		t.Fatal("Build did not stamp the corpus fingerprint onto the graph")
	}
	before := built.Graph.Communities

	second := corpus("nlp", "nlp", len(first))
	if err := sp.WritePapers(second); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(ctx, sp, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Graph.Fingerprint != graph.Fingerprint(second) {
		t.Error("Load served a graph built from the previous corpus")
	}
	if len(before) > 0 && len(loaded.Graph.Communities) > 0 &&
		before[0].Label == loaded.Graph.Communities[0].Label {
		t.Errorf("communities did not change with the corpus; still labelled %q", before[0].Label)
	}
}

func TestLoadReusesTheCacheForAnUnchangedCorpus(t *testing.T) {
	ctx := context.Background()
	sp := newProject(t)

	papers := corpus("aero", "aero", 40)
	if err := sp.WritePapers(papers); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(ctx, sp, papers, nil, false, nil); err != nil {
		t.Fatalf("Build: %v", err)
	}
	loaded, err := Load(ctx, sp, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Graph.Fingerprint != graph.Fingerprint(papers) {
		t.Error("Load rebuilt a cache that was still valid")
	}
}

func TestLoadRebuildsAGraphWrittenBeforeFingerprintsExisted(t *testing.T) {
	ctx := context.Background()
	sp := newProject(t)

	papers := corpus("aero", "aero", 40)
	if err := sp.WritePapers(papers); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(ctx, sp, papers, nil, false, nil); err != nil {
		t.Fatalf("Build: %v", err)
	}
	var g graph.Graph
	if err := sp.LoadJSON("graph.json", &g); err != nil {
		t.Fatal(err)
	}
	g.Fingerprint = ""
	if err := sp.SaveJSON("graph.json", &g); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(ctx, sp, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Graph.Fingerprint == "" {
		t.Error("a graph with no fingerprint should be rebuilt, not reused")
	}
}
