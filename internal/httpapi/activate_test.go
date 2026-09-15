package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yothgewalt/reharvester/internal/graph"
	"github.com/yothgewalt/reharvester/internal/index"
	"github.com/yothgewalt/reharvester/internal/paper"
	"github.com/yothgewalt/reharvester/internal/pipeline"
	"github.com/yothgewalt/reharvester/internal/store"
)

func tinyProject(n int) *pipeline.Project {
	papers := make([]paper.Paper, n)
	for i := range papers {
		papers[i] = paper.Paper{ID: fmt.Sprintf("x.%d", i), Title: fmt.Sprintf("p%d", i)}
	}
	g := &graph.Graph{N: n, Community: make([]int32, n), PageRank: make([]float64, n), Bridge: make([]float64, n)}
	g.Index()
	return &pipeline.Project{Papers: papers, Graph: g}
}

// makeProject creates a project directory under the server's store, with a
// corpus file when withCorpus is set.
func makeProject(t *testing.T, s *Server, id string, withCorpus bool) {
	t.Helper()
	sp, err := s.store.Project(id)
	if err != nil {
		t.Fatal(err)
	}
	if withCorpus {
		if err := os.WriteFile(sp.Path("papers.jsonl"), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func doActivation(t *testing.T, s *Server, method, id string) (int, activationStatus, map[string]string) {
	t.Helper()
	path := "/api/v1/projects/" + id + "/activation"
	if method == http.MethodPost {
		path = "/api/v1/projects/" + id + "/activate"
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	if rec.Code >= 400 {
		return rec.Code, activationStatus{}, decodeErrors(t, rec.Body.Bytes())
	}
	var st activationStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, rec.Body.Bytes())
	}
	return rec.Code, st, nil
}

func waitStatus(t *testing.T, s *Server, id, want string) activationStatus {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, st, _ := doActivation(t, s, http.MethodGet, id)
		if st.Status == want {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("activation of %q stayed %q, want %q", id, st.Status, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestActivateRejectsBadMissingAndEmptyProjects(t *testing.T) {
	s := newTestServer(t, false)
	makeProject(t, s, "empty", false)

	for _, tc := range []struct {
		name, id string
		code     int
	}{
		{"dot in id", "a.b", http.StatusBadRequest},
		{"unknown", "ghost", http.StatusNotFound},
		{"no corpus", "empty", http.StatusConflict},
	} {
		code, _, errs := doActivation(t, s, http.MethodPost, tc.id)
		if code != tc.code || errs["_"] == "" {
			t.Errorf("%s: POST = %d %v, want %d with a message", tc.name, code, errs, tc.code)
		}
	}
	if _, err := os.Stat(filepath.Join(s.store.Root(), "projects", "ghost")); !os.IsNotExist(err) {
		t.Errorf("activating an unknown project created its directory (stat err %v)", err)
	}
}

func TestActivateLoadsInBackgroundThenServesTheProject(t *testing.T) {
	s := newTestServer(t, false)
	makeProject(t, s, "alpha", true)
	release := make(chan struct{})
	s.loadProject = func(ctx context.Context, sp *store.Project, _ index.Embedder) (*pipeline.Project, error) {
		<-release
		return tinyProject(3), nil
	}

	code, st, _ := doActivation(t, s, http.MethodPost, "alpha")
	if code != http.StatusAccepted || st.Status != activationLoading {
		t.Fatalf("POST = %d %+v, want 202 loading", code, st)
	}
	if code, st, _ := doActivation(t, s, http.MethodPost, "alpha"); code != http.StatusAccepted || st.Status != activationLoading {
		t.Fatalf("second POST while loading = %d %+v, want 202 loading", code, st)
	}
	close(release)
	waitStatus(t, s, "alpha", activationActive)

	if _, snaps := s.active(); snaps == nil || len(snaps.Knowledge.Nodes) != 3 {
		t.Fatalf("active snapshot not swapped in: %+v", snaps)
	}
	if code, st, _ := doActivation(t, s, http.MethodPost, "alpha"); code != http.StatusOK || st.Status != activationActive {
		t.Errorf("POST on the active project = %d %+v, want 200 active", code, st)
	}
}

func TestNewerActivationWins(t *testing.T) {
	s := newTestServer(t, false)
	makeProject(t, s, "old", true)
	makeProject(t, s, "new", true)
	releaseOld := make(chan struct{})
	s.loadProject = func(ctx context.Context, sp *store.Project, _ index.Embedder) (*pipeline.Project, error) {
		if sp.ID == "old" {
			<-releaseOld
			return tinyProject(2), nil
		}
		return tinyProject(5), nil
	}

	doActivation(t, s, http.MethodPost, "old")
	doActivation(t, s, http.MethodPost, "new")
	waitStatus(t, s, "new", activationActive)
	close(releaseOld)
	time.Sleep(20 * time.Millisecond)

	if _, st, _ := doActivation(t, s, http.MethodGet, "old"); st.Status != activationInactive {
		t.Errorf("superseded activation = %q, want inactive", st.Status)
	}
	s.mu.RLock()
	activeID := s.activeID
	s.mu.RUnlock()
	if activeID != "new" {
		t.Errorf("active project = %q, want new", activeID)
	}
}

func TestActivationReportsLoadError(t *testing.T) {
	s := newTestServer(t, false)
	makeProject(t, s, "broken", true)
	s.loadProject = func(context.Context, *store.Project, index.Embedder) (*pipeline.Project, error) {
		return nil, errors.New("no papers in broken")
	}
	doActivation(t, s, http.MethodPost, "broken")
	if st := waitStatus(t, s, "broken", activationError); st.Message != "no papers in broken" {
		t.Errorf("message = %q", st.Message)
	}
}
