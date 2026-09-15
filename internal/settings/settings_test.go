package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := Default()
	s.DataDir = dir
	s.Addr = ":9123"
	s.EmbedModel = "nomic-embed-text"
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := Load(dir)
	if got.Addr != ":9123" || got.EmbedModel != "nomic-embed-text" {
		t.Fatalf("round trip lost values: %+v", got)
	}
	if got.Delay != s.Delay {
		t.Errorf("delay = %v, want %v", got.Delay, s.Delay)
	}
}

// TestLoadSurvivesCorruption: a bad settings file must not stop a launch.
func TestLoadSurvivesCorruption(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(Path(dir), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Load(dir)
	if got.Addr != Default().Addr {
		t.Fatalf("expected defaults, got %+v", got)
	}
}

func TestSaveWritesOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	s := Default()
	s.DataDir = dir
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %o, want 600", perm)
	}
}

func TestSaveFixesAnOlderWorldReadableFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(dir), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Default()
	s.DataDir = dir
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %o, want 600", perm)
	}
}

// TestUpdateKeepsOtherFields: a writer that only sets its own field must not
// clobber a value saved earlier by a different writer.
func TestUpdateKeepsOtherFields(t *testing.T) {
	dir := t.TempDir()
	if _, err := Update(dir, func(s *Settings) { s.Addr = ":9000" }); err != nil {
		t.Fatalf("update addr: %v", err)
	}
	got, err := Update(dir, func(s *Settings) { s.Project = "quantum" })
	if err != nil {
		t.Fatalf("update project: %v", err)
	}
	if got.Addr != ":9000" || got.Project != "quantum" {
		t.Fatalf("update lost a sibling field: %+v", got)
	}

	onDisk := Load(dir)
	if onDisk.Addr != ":9000" || onDisk.Project != "quantum" {
		t.Fatalf("disk copy lost a field: %+v", onDisk)
	}
}

func raw(t *testing.T, v any) map[string]json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestApply(t *testing.T) {
	cur := Default()
	cases := []struct {
		name    string
		patch   map[string]any
		wantErr string // key that must be present in errs; "" means no error
		check   func(t *testing.T, got Settings)
	}{
		{"addr ok", map[string]any{"addr": "127.0.0.1:9000"}, "",
			func(t *testing.T, got Settings) {
				if got.Addr != "127.0.0.1:9000" {
					t.Errorf("addr = %q", got.Addr)
				}
			}},
		{"addr missing port", map[string]any{"addr": "127.0.0.1"}, "addr", nil},
		{"addr bare port ok", map[string]any{"addr": ":8080"}, "", nil},
		{"addr non numeric port", map[string]any{"addr": "host:abc"}, "addr", nil},
		{"project ok", map[string]any{"project": "myproj"}, "", nil},
		{"project has slash", map[string]any{"project": "a/b"}, "project", nil},
		{"project has dot", map[string]any{"project": "a.b"}, "project", nil},
		{"project has backslash", map[string]any{"project": `a\b`}, "project", nil},
		{"source ok", map[string]any{"source": "OpenAlex"}, "",
			func(t *testing.T, got Settings) {
				if got.Source != "openalex" {
					t.Errorf("source = %q, want normalised", got.Source)
				}
			}},
		{"source unknown", map[string]any{"source": "scopus"}, "source", nil},
		{"max ok", map[string]any{"max": 500}, "", nil},
		{"max zero", map[string]any{"max": 0}, "max", nil},
		{"max too big", map[string]any{"max": 50001}, "max", nil},
		{"max not a number", map[string]any{"max": "500"}, "max", nil},
		{"delay ok", map[string]any{"delay": "5s"}, "", nil},
		{"delay too long", map[string]any{"delay": "2m"}, "delay", nil},
		{"delay negative", map[string]any{"delay": "-1s"}, "delay", nil},
		{"delay malformed", map[string]any{"delay": "soon"}, "delay", nil},
		{"arxivSnapshot ok", map[string]any{"arxivSnapshot": "  /data/x.json  "}, "",
			func(t *testing.T, got Settings) {
				if got.SnapshotPath != "/data/x.json" {
					t.Errorf("snapshotPath = %q", got.SnapshotPath)
				}
			}},
		{"ollamaUrl ok", map[string]any{"ollamaUrl": "http://localhost:11434"}, "", nil},
		{"ollamaUrl https ok", map[string]any{"ollamaUrl": "https://ollama.example.com"}, "", nil},
		{"ollamaUrl no scheme", map[string]any{"ollamaUrl": "localhost:11434"}, "ollamaUrl", nil},
		{"ollamaUrl bad scheme", map[string]any{"ollamaUrl": "ftp://x"}, "ollamaUrl", nil},
		{"chatModel ok", map[string]any{"chatModel": "llama3.2"}, "", nil},
		{"chatModel empty", map[string]any{"chatModel": "  "}, "chatModel", nil},
		{"chatModel spaces", map[string]any{"chatModel": "llama 3"}, "chatModel", nil},
		{"embedModel ok", map[string]any{"embedModel": "all-minilm"}, "", nil},
		{"embedModel empty", map[string]any{"embedModel": ""}, "embedModel", nil},
		{"snapshotNodes ok", map[string]any{"snapshotNodes": 100}, "", nil},
		{"snapshotNodes zero", map[string]any{"snapshotNodes": 0}, "snapshotNodes", nil},
		{"openalexKey sets", map[string]any{"openalexKey": "abc"}, "",
			func(t *testing.T, got Settings) {
				if got.OpenAlexKey != "abc" {
					t.Errorf("openalexKey = %q", got.OpenAlexKey)
				}
			}},
		{"openalexKey clears", map[string]any{"openalexKey": nil}, "",
			func(t *testing.T, got Settings) {
				if got.OpenAlexKey != "" {
					t.Errorf("openalexKey = %q, want cleared", got.OpenAlexKey)
				}
			}},
		{"openalexKey empty errors", map[string]any{"openalexKey": ""}, "openalexKey", nil},
		{"semanticScholarKey sets", map[string]any{"semanticScholarKey": "s2"}, "", nil},
		{"ollamaKey sets", map[string]any{"ollamaKey": "ok"}, "", nil},
		{"unknown key", map[string]any{"approx": true}, "approx", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, errs := Apply(cur, raw(t, tc.patch))
			if tc.wantErr != "" {
				if errs[tc.wantErr] == "" {
					t.Fatalf("errs = %v, want a message for %q", errs, tc.wantErr)
				}
				if got != cur {
					t.Errorf("settings changed despite a validation error: %+v", got)
				}
				return
			}
			if len(errs) != 0 {
				t.Fatalf("unexpected errs: %v", errs)
			}
			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}
}

// TestApplyRejectsWholePatchOnAnyError: one bad key must leave every field
// from the same patch untouched, including the ones that would have been
// valid on their own.
func TestApplyRejectsWholePatchOnAnyError(t *testing.T) {
	cur := Default()
	got, errs := Apply(cur, raw(t, map[string]any{
		"addr": "127.0.0.1:9999",
		"max":  -1,
	}))
	if len(errs) == 0 {
		t.Fatal("expected an error for max")
	}
	if got != cur {
		t.Errorf("valid key was applied despite a sibling error: %+v", got)
	}
}

func TestValidYearValidMaxValidSource(t *testing.T) {
	if !ValidYear(2020) || ValidYear(1989) || ValidYear(2101) {
		t.Error("ValidYear bounds wrong")
	}
	if !ValidMax(1) || !ValidMax(50000) || ValidMax(0) || ValidMax(50001) {
		t.Error("ValidMax bounds wrong")
	}
	if !ValidSource("arxiv") || !ValidSource(" ArXiv ") || ValidSource("bogus") {
		t.Error("ValidSource wrong")
	}
}

func TestPathIsUnderDataDir(t *testing.T) {
	if got := Path("foo"); got != filepath.Join("foo", "tui-settings.json") {
		t.Errorf("Path = %q", got)
	}
}
