package httpapi

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/yothgewalt/reharvester/internal/index"
	"github.com/yothgewalt/reharvester/internal/pipeline"
	"github.com/yothgewalt/reharvester/internal/store"
)

// Activation states reported by the activate and activation endpoints.
const (
	activationActive   = "active"
	activationLoading  = "loading"
	activationInactive = "inactive"
	activationError    = "error"
)

// activationStatus is the body of POST /projects/{id}/activate and GET
// /projects/{id}/activation. Poll the GET while Status is "loading"; a stale
// project is rebuilt before it becomes active, which can take minutes.
type activationStatus struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type activationState struct {
	status  string
	message string
}

// projectLoader matches pipeline.Load; tests replace Server.loadProject with a stub.
type projectLoader func(ctx context.Context, sp *store.Project, emb index.Embedder) (*pipeline.Project, error)

// lookupProject validates a project id from the URL and finds it on disk
// without creating anything. On failure it has already written the 4xx.
func (s *Server) lookupProject(w http.ResponseWriter, r *http.Request) (*store.Project, bool) {
	id := r.PathValue("projectId")
	if id == "" || strings.ContainsAny(id, `/\.`) {
		writeErrors(w, map[string]string{"_": "Invalid project id"})
		return nil, false
	}
	sp, ok := s.store.ExistingProject(id)
	if !ok {
		writeErrorsStatus(w, http.StatusNotFound, map[string]string{"_": "No such project"})
		return nil, false
	}
	if !sp.Exists("papers.jsonl") {
		writeErrorsStatus(w, http.StatusConflict, map[string]string{"_": "This project has no corpus yet"})
		return nil, false
	}
	return sp, true
}

func (s *Server) activateProject(w http.ResponseWriter, r *http.Request) {
	sp, ok := s.lookupProject(w, r)
	if !ok {
		return
	}
	if st := s.activationOf(sp.ID); st.Status == activationActive {
		writeJSON(w, st)
		return
	}

	s.activateMu.Lock()
	if s.activations[sp.ID].status == activationLoading {
		s.activateMu.Unlock()
		writeAccepted(w, activationStatus{ID: sp.ID, Status: activationLoading})
		return
	}
	s.activateGen++
	gen := s.activateGen
	for id, prev := range s.activations {
		if prev.status == activationLoading {
			s.activations[id] = activationState{status: activationInactive}
		}
	}
	s.activations[sp.ID] = activationState{status: activationLoading}
	s.activateMu.Unlock()

	go s.runActivation(context.WithoutCancel(r.Context()), sp, gen)
	writeAccepted(w, activationStatus{ID: sp.ID, Status: activationLoading})
}

func writeAccepted(w http.ResponseWriter, v activationStatus) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, v)
}

func (s *Server) activation(w http.ResponseWriter, r *http.Request) {
	sp, ok := s.lookupProject(w, r)
	if !ok {
		return
	}
	writeJSON(w, s.activationOf(sp.ID))
}

func (s *Server) activationOf(id string) activationStatus {
	s.mu.RLock()
	active := s.activeID == id && s.project != nil
	s.mu.RUnlock()
	if active {
		return activationStatus{ID: id, Status: activationActive}
	}
	s.activateMu.Lock()
	defer s.activateMu.Unlock()
	st, ok := s.activations[id]
	if !ok || st.status == activationActive {
		return activationStatus{ID: id, Status: activationInactive}
	}
	return activationStatus{ID: id, Status: st.status, Message: st.message}
}

func (s *Server) runActivation(ctx context.Context, sp *store.Project, gen uint64) {
	p, err := s.loadProject(ctx, sp, s.cfgNow().embedder)

	s.activateMu.Lock()
	defer s.activateMu.Unlock()
	if gen != s.activateGen {
		return
	}
	if err != nil {
		log.Printf("api: could not activate project %q: %v", sp.ID, err)
		s.activations[sp.ID] = activationState{status: activationError, message: err.Error()}
		return
	}
	s.setActive(sp.ID, p)
	s.activations[sp.ID] = activationState{status: activationActive}
	log.Printf("api: active project %q — %d papers, tier %s", sp.ID, len(p.Papers), p.Tier())
}
