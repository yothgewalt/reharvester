package app

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/yothgewalt/reharvester/internal/store"
)

func TestSplitList(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"cs.IR,cs.DL", []string{"cs.IR", "cs.DL"}},
		{" cs.IR , cs.DL ", []string{"cs.IR", "cs.DL"}},
		{"cs.IR,,cs.DL", []string{"cs.IR", "cs.DL"}},
		{"", nil},
		{"   ", nil},
		{",", nil},
	}
	for _, tc := range cases {
		if got := SplitList(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("SplitList(%q) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
}

// TestPickProjectPrefersNamedWithCorpus: the named project wins only when it
// actually holds papers, otherwise serving would load an empty corpus and the
// UI would show nothing with no explanation.
func TestPickProjectPrefersNamedWithCorpus(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeCorpus(t, st, "alpha")
	writeCorpus(t, st, "beta")

	if got := PickProject(st, "beta"); got != "beta" {
		t.Errorf("PickProject(beta) = %q, want beta", got)
	}
	// A named project with no papers must fall through to one that has them.
	if _, err := st.Project("empty"); err != nil {
		t.Fatal(err)
	}
	got := PickProject(st, "empty")
	if got != "alpha" && got != "beta" {
		t.Errorf("PickProject(empty) = %q, want a project holding papers", got)
	}
}

func TestPickProjectEmptyWhenNothingHarvested(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := PickProject(st, "default"); got != "" {
		t.Errorf("PickProject = %q, want empty when no corpus exists", got)
	}
}

// TestEmbedRecordRoundTrip covers the guard against a silent failure: a corpus
// built with one encoder and queried with another returns no dense results and
// no error, so the recorded model is the only way to detect it.
func TestEmbedRecordRoundTrip(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	proj, err := st.Project("dev")
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := ReadEmbedRecord(st, "dev"); ok {
		t.Fatal("a project with no record must report ok=false, not a zero record")
	}

	want := EmbedRecord{Model: "nomic-embed-text", Dim: 768}
	if err := proj.SaveJSON(embedRecordFile, &want); err != nil {
		t.Fatal(err)
	}
	got, ok := ReadEmbedRecord(st, "dev")
	if !ok {
		t.Fatal("record written but not read back")
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestEmbedRecordIgnoresJunk: a truncated or hand-edited record must read as
// "unknown" rather than as a model named "".
func TestEmbedRecordIgnoresJunk(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	proj, err := st.Project("dev")
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"{not json", `{}`, `{"model":""}`} {
		if err := os.WriteFile(proj.Path(embedRecordFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, ok := ReadEmbedRecord(st, "dev"); ok {
			t.Errorf("body %q should not produce a usable record", body)
		}
	}
}

func TestPortSuffix(t *testing.T) {
	cases := map[string]string{
		":8000":          ":8000",
		"0.0.0.0:8000":   ":8000",
		"127.0.0.1:9999": ":9999",
		"8000":           ":8000",
	}
	for in, want := range cases {
		if got := portSuffix(in); got != want {
			t.Errorf("portSuffix(%q) = %q, want %q", in, got, want)
		}
	}
}

// writeCorpus creates the state a completed harvest leaves behind: the papers
// plus the meta record. Store.Projects only lists directories carrying
// meta.json, so a fixture without it is invisible to PickProject's fallback.
func writeCorpus(t *testing.T, st *store.Store, id string) {
	t.Helper()
	proj, err := st.Project(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proj.Path("papers.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := store.Meta{
		ID: id, Name: id, Status: "complete", DocsIngested: 1,
		CreatedAt: time.Now().UTC(),
	}
	if err := proj.SaveJSON("meta.json", &meta); err != nil {
		t.Fatal(err)
	}
}
