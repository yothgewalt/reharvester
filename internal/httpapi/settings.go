package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/yothgewalt/reharvester/internal/harvest"
	"github.com/yothgewalt/reharvester/internal/index"
	"github.com/yothgewalt/reharvester/internal/rag"
	"github.com/yothgewalt/reharvester/internal/settings"
)

// liveConfig is the part of the server's configuration a PATCH can change
// while it runs. chat is the client built from the current settings, whether
// or not it can generate right now; llm is chat once probeChat has confirmed
// it, and is what every other handler should call through — reading it
// costs nothing, unlike re-probing on every request. embedder follows the
// same pattern: modelGen guards it against a stale probe finishing after a
// newer settings change.
type liveConfig struct {
	settings settings.Settings
	chat     *rag.Ollama
	llm      *rag.Ollama
	embedder index.Embedder
	modelGen int
}

// cfgNow returns a copy of the live configuration. Safe to call from any
// handler; the copy is stale the instant it is taken, same as any other
// snapshot of concurrently-changing state.
func (s *Server) cfgNow() liveConfig {
	s.liveMu.RLock()
	defer s.liveMu.RUnlock()
	return s.live
}

// probeChat checks whether chat can generate right now and, when chat is
// still the server's configured client, promotes or demotes live.llm to
// match. It always probes a copy of chat: CanGenerate can rewrite ChatModel
// to a fallback it found pulled, and doing that on the shared client while
// another request reads it is a data race.
func (s *Server) probeChat(ctx context.Context, chat *rag.Ollama) bool {
	if chat == nil {
		return false
	}
	probe := *chat
	ok := probe.CanGenerate(ctx)
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	if s.live.chat != chat {
		return ok // superseded by a later settings change; do not promote
	}
	if ok {
		s.live.llm = &probe
	} else {
		s.live.llm = nil
	}
	return ok
}

// rebuildActiveSnapshot recomputes the active project's graph snapshot at n
// nodes and swaps it in, but only if the active project and the snapshot
// size are both still what they were when the rebuild was requested — a
// harvest that completes mid-rebuild, or a second PATCH, must win.
func (s *Server) rebuildActiveSnapshot(n int) {
	s.mu.RLock()
	p, id := s.project, s.activeID
	s.mu.RUnlock()
	if p == nil {
		return
	}
	snaps := BuildSnapshots(p, n)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeID == id && s.project == p && s.cfgNow().settings.Snapshot == n {
		s.snaps = snaps
	}
}

// settingsFields is the settings object inside a GET/PATCH response. Key
// values are never included — only whether one is set.
type settingsFields struct {
	Addr                  string `json:"addr"`
	Project               string `json:"project"`
	Source                string `json:"source"`
	Max                   int    `json:"max"`
	Delay                 string `json:"delay"`
	ArxivSnapshot         string `json:"arxivSnapshot"`
	OllamaURL             string `json:"ollamaUrl"`
	ChatModel             string `json:"chatModel"`
	EmbedModel            string `json:"embedModel"`
	SnapshotNodes         int    `json:"snapshotNodes"`
	HasOpenAlexKey        bool   `json:"hasOpenalexKey"`
	HasSemanticScholarKey bool   `json:"hasSemanticScholarKey"`
	HasOllamaKey          bool   `json:"hasOllamaKey"`
}

type harvestDefaultsView struct {
	Source string `json:"source"`
	From   int    `json:"from"`
	To     int    `json:"to"`
	Max    int    `json:"max"`
}

// settingsView is the full GET/PATCH /api/v1/settings response.
type settingsView struct {
	Settings        settingsFields      `json:"settings"`
	DataDir         string              `json:"dataDir"`
	Persisted       bool                `json:"persisted"`
	RestartRequired []string            `json:"restartRequired"`
	HarvestDefaults harvestDefaultsView `json:"harvestDefaults"`
	Sources         []string            `json:"sources"`
	HasSnapshotPath bool                `json:"hasSnapshotPath"`
	Notices         []string            `json:"notices"`
}

// viewOf builds the settings response. year drives harvestDefaults.from/to
// (now-7..now); notices is whatever the caller wants to report alongside the
// current state and may be nil.
func viewOf(live liveConfig, dataDir string, persist bool, year int, notices []string) settingsView {
	s := live.settings
	if notices == nil {
		notices = []string{}
	}
	return settingsView{
		Settings: settingsFields{
			Addr: s.Addr, Project: s.Project, Source: s.Source, Max: s.Max,
			Delay: s.Delay.String(), ArxivSnapshot: s.SnapshotPath,
			OllamaURL: s.OllamaURL, ChatModel: s.ChatModel, EmbedModel: s.EmbedModel,
			SnapshotNodes:         s.Snapshot,
			HasOpenAlexKey:        s.OpenAlexKey != "",
			HasSemanticScholarKey: s.SemanticScholarKey != "",
			HasOllamaKey:          s.OllamaKey != "",
		},
		DataDir:         dataDir,
		Persisted:       persist,
		RestartRequired: []string{"addr", "project"},
		HarvestDefaults: harvestDefaultsView{Source: s.Source, From: year - 7, To: year, Max: s.Max},
		Sources:         harvest.Sources,
		HasSnapshotPath: strings.TrimSpace(s.SnapshotPath) != "",
		Notices:         notices,
	}
}

// absDataDir resolves dir against the working directory, so the client always
// sees an absolute path regardless of how --data was written.
func absDataDir(dir string) string {
	if dir == "" {
		dir = "."
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	live := s.cfgNow()
	writeJSON(w, viewOf(live, absDataDir(live.settings.DataDir), s.cfg.Persist, time.Now().Year(), nil))
}

const (
	encoderChangedNotice  = "Encoder changed — rebuild existing projects so dense search uses it"
	restartRequiredNotice = "Listen address and project take effect on the next server start"
	sessionOnlyNotice     = "Not persisted — settings apply to this session only"
)

// maxPatchBody bounds the PATCH body. A settings patch is a handful of short
// fields; 64 KB is generous headroom, not a working limit.
const maxPatchBody = 64 << 10

// patchSettings applies a partial settings update and reports the result the
// same way GET does. It never returns a 5xx: a bad request is a 400 with
// {"errors": {...}}, and a failed disk write still applies live and is
// reported as a notice rather than an error.
func (s *Server) patchSettings(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPatchBody)
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeErrors(w, map[string]string{"_": "invalid JSON"})
		return
	}

	s.liveMu.Lock()
	cur := s.live.settings
	next, errs := settings.Apply(cur, raw)
	if len(errs) > 0 {
		s.liveMu.Unlock()
		writeErrors(w, errs)
		return
	}

	chatChanged := next.OllamaURL != cur.OllamaURL || next.ChatModel != cur.ChatModel || next.OllamaKey != cur.OllamaKey
	encoderOrURLChanged := next.OllamaURL != cur.OllamaURL || next.EmbedModel != cur.EmbedModel
	snapshotChanged := next.Snapshot != cur.Snapshot
	restartOnly := next.Addr != cur.Addr || next.Project != cur.Project

	s.live.settings = next
	var newChat *rag.Ollama
	if chatChanged {
		newChat = rag.NewChat(next.OllamaURL, next.ChatModel, next.OllamaKey)
		s.live.chat, s.live.llm = newChat, nil
	}
	gen := s.live.modelGen
	if encoderOrURLChanged {
		s.live.modelGen++
		gen = s.live.modelGen
	}
	s.liveMu.Unlock()

	if chatChanged {
		// The corpus did not change, but every cached synthesis was written
		// against the previous model.
		s.wikiSynth.Clear()
		s.synthing.Clear()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			s.probeChat(ctx, newChat)
		}()
	}
	if encoderOrURLChanged {
		url, model := next.OllamaURL, next.EmbedModel
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			emb := rag.ProbeEmbedder(ctx, url, model)
			s.liveMu.Lock()
			if s.live.modelGen == gen {
				s.live.embedder = emb
			}
			s.liveMu.Unlock()
		}()
	}
	if snapshotChanged {
		go s.rebuildActiveSnapshot(next.Snapshot)
	}

	var notices []string
	if next.EmbedModel != cur.EmbedModel {
		notices = append(notices, encoderChangedNotice)
	}
	if restartOnly {
		notices = append(notices, restartRequiredNotice)
	}
	if s.cfg.Persist {
		if _, err := settings.Update(next.DataDir, func(cur *settings.Settings) {
			if applied, aerr := settings.Apply(*cur, raw); len(aerr) == 0 {
				*cur = applied
			}
		}); err != nil {
			notices = append(notices, fmt.Sprintf(
				"Could not save settings to disk (%v) — changes apply to this session only", err))
		}
	} else {
		notices = append(notices, sessionOnlyNotice)
	}

	writeJSON(w, viewOf(s.cfgNow(), absDataDir(next.DataDir), s.cfg.Persist, time.Now().Year(), notices))
}
