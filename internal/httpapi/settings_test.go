package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/yothgewalt/reharvester/internal/settings"
	"github.com/yothgewalt/reharvester/internal/store"
)

func newTestServer(t *testing.T, persist bool) *Server {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	s := settings.Default()
	s.DataDir = dir
	return New(st, Config{Version: "test", Settings: s, Persist: persist})
}

func decodeErrors(t *testing.T, body []byte) map[string]string {
	t.Helper()
	var v struct {
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("decode errors: %v\nbody: %s", err, body)
	}
	return v.Errors
}

func decodeSettingsView(t *testing.T, body []byte) settingsView {
	t.Helper()
	var v settingsView
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("decode settings view: %v\nbody: %s", err, body)
	}
	return v
}

// TestViewOfNeverLeaksKeys pins the contract: GET and PATCH describe whether a
// key is set, never its value.
func TestViewOfNeverLeaksKeys(t *testing.T) {
	live := liveConfig{settings: settings.Default()}
	live.settings.OpenAlexKey = "oa-secret"
	live.settings.SemanticScholarKey = "s2-secret"
	live.settings.OllamaKey = "ollama-secret"

	view := viewOf(live, "/data", true, 2026, nil)
	b, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"oa-secret", "s2-secret", "ollama-secret"} {
		if bytes.Contains(b, []byte(secret)) {
			t.Errorf("response leaked a key value: %s", b)
		}
	}
	if !view.Settings.HasOpenAlexKey || !view.Settings.HasSemanticScholarKey || !view.Settings.HasOllamaKey {
		t.Errorf("has* flags did not reflect the set keys: %+v", view.Settings)
	}
}

func TestGetSettingsShape(t *testing.T) {
	s := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	w := httptest.NewRecorder()
	s.getSettings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	view := decodeSettingsView(t, w.Body.Bytes())
	if view.Settings.Addr != ":8000" || view.Settings.Source != "arxiv" {
		t.Errorf("unexpected defaults: %+v", view.Settings)
	}
	if !view.Persisted {
		t.Error("persisted should be true")
	}
	if len(view.Sources) == 0 {
		t.Error("sources should be populated")
	}
	if view.Notices == nil {
		t.Error("notices must be [] not null")
	}
	want := []string{"addr", "project"}
	if len(view.RestartRequired) != 2 || view.RestartRequired[0] != want[0] || view.RestartRequired[1] != want[1] {
		t.Errorf("restartRequired = %v, want %v", view.RestartRequired, want)
	}
}

func doPatch(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.patchSettings(w, req)
	return w
}

func TestPatchSettingsInvalidJSON(t *testing.T) {
	s := newTestServer(t, true)
	w := doPatch(t, s, "{not json")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	errs := decodeErrors(t, w.Body.Bytes())
	if errs["_"] == "" {
		t.Errorf("errs = %v, want a message under \"_\"", errs)
	}
}

// TestPatchSettings400AppliesNothing: a bad key in the patch must reject the
// whole thing, and a later GET must show the original values.
func TestPatchSettings400AppliesNothing(t *testing.T) {
	s := newTestServer(t, true)
	w := doPatch(t, s, `{"addr":"127.0.0.1:9999","max":-1}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	errs := decodeErrors(t, w.Body.Bytes())
	if errs["max"] == "" {
		t.Fatalf("errs = %v, want a message for max", errs)
	}

	getW := httptest.NewRecorder()
	s.getSettings(getW, httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil))
	view := decodeSettingsView(t, getW.Body.Bytes())
	if view.Settings.Addr != ":8000" {
		t.Errorf("addr = %q, a rejected patch must not apply any field", view.Settings.Addr)
	}
}

func TestPatchSettingsUnknownKey(t *testing.T) {
	s := newTestServer(t, true)
	w := doPatch(t, s, `{"approx":true}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	errs := decodeErrors(t, w.Body.Bytes())
	if errs["approx"] != "not a web setting" {
		t.Errorf("errs[approx] = %q", errs["approx"])
	}
}

// TestPatchSettingsSuccessWritesOwnerOnlyFile: with Persist true, a valid
// patch writes tui-settings.json mode 0600 in the data directory.
func TestPatchSettingsSuccessWritesOwnerOnlyFile(t *testing.T) {
	s := newTestServer(t, true)
	w := doPatch(t, s, `{"max":500}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	view := decodeSettingsView(t, w.Body.Bytes())
	if view.Settings.Max != 500 {
		t.Errorf("max = %d, want 500", view.Settings.Max)
	}

	path := settings.Path(s.cfgNow().settings.DataDir)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("settings file was not written: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %o, want 600", perm)
	}
	onDisk := settings.Load(s.cfgNow().settings.DataDir)
	if onDisk.Max != 500 {
		t.Errorf("on-disk max = %d, want 500", onDisk.Max)
	}
}

// TestPatchSettingsWithoutPersistWritesNothing: harvester-server runs with
// Persist false and must never touch disk.
func TestPatchSettingsWithoutPersistWritesNothing(t *testing.T) {
	s := newTestServer(t, false)
	w := doPatch(t, s, `{"max":500}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	view := decodeSettingsView(t, w.Body.Bytes())
	if view.Settings.Max != 500 {
		t.Errorf("live max = %d, want 500 (still applied in memory)", view.Settings.Max)
	}
	if view.Persisted {
		t.Error("persisted should be false")
	}
	found := false
	for _, n := range view.Notices {
		if strings.Contains(n, "session only") {
			found = true
		}
	}
	if !found {
		t.Errorf("notices = %v, want a session-only notice", view.Notices)
	}
	if _, err := os.Stat(settings.Path(s.cfgNow().settings.DataDir)); !os.IsNotExist(err) {
		t.Errorf("settings file should not exist, stat err = %v", err)
	}
}

// TestPatchSettingsDiskMergeKeepsSiblingField: a field the TUI saved directly
// to disk must survive a PATCH that only touches a different field.
func TestPatchSettingsDiskMergeKeepsSiblingField(t *testing.T) {
	s := newTestServer(t, true)
	dataDir := s.cfgNow().settings.DataDir

	// The TUI writes through settings.Update, independently of the server.
	if _, err := settings.Update(dataDir, func(cur *settings.Settings) { cur.Project = "quantum-computing" }); err != nil {
		t.Fatalf("simulated TUI write: %v", err)
	}

	w := doPatch(t, s, `{"max":777}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	onDisk := settings.Load(dataDir)
	if onDisk.Project != "quantum-computing" {
		t.Errorf("project = %q, want the TUI's value preserved", onDisk.Project)
	}
	if onDisk.Max != 777 {
		t.Errorf("max = %d, want 777", onDisk.Max)
	}
}

func TestPatchSettingsEncoderChangeAddsNotice(t *testing.T) {
	s := newTestServer(t, true)
	w := doPatch(t, s, `{"embedModel":"nomic-embed-text"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	view := decodeSettingsView(t, w.Body.Bytes())
	found := false
	for _, n := range view.Notices {
		if strings.Contains(n, "rebuild") {
			found = true
		}
	}
	if !found {
		t.Errorf("notices = %v, want an encoder-changed notice", view.Notices)
	}
}

// TestPatchSettingsConcurrent exercises the liveMu path under -race: many
// goroutines patching distinct fields must all succeed and leave the store
// internally consistent.
func TestPatchSettingsConcurrent(t *testing.T) {
	s := newTestServer(t, true)
	var wg sync.WaitGroup
	for i := 1; i <= 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			body := `{"max":` + itoa(100+n) + `}`
			w := doPatch(t, s, body)
			if w.Code != http.StatusOK {
				t.Errorf("status = %d, body = %s", w.Code, w.Body.String())
			}
		}(i)
	}
	wg.Wait()

	view := decodeSettingsView(t, func() []byte {
		w := httptest.NewRecorder()
		s.getSettings(w, httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil))
		return w.Body.Bytes()
	}())
	if view.Settings.Max < 101 || view.Settings.Max > 120 {
		t.Errorf("max = %d, want one of the concurrently-applied values", view.Settings.Max)
	}
}
