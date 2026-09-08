package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/yothgewalt/reharvester/internal/harvest"
	"github.com/yothgewalt/reharvester/internal/httpapi"
	"github.com/yothgewalt/reharvester/internal/rag"
)

// Settings is what the TUI remembers between runs. It is deliberately the same
// set of knobs the harvester-server flags expose, so a value chosen here and a
// value passed on the command line mean the same thing.
type Settings struct {
	DataDir    string        `json:"dataDir"`
	Addr       string        `json:"addr"`
	Project    string        `json:"project"`
	Categories string        `json:"categories"`
	Keywords   string        `json:"keywords"`
	From       int           `json:"from"`
	To         int           `json:"to"`
	Max        int           `json:"max"`
	Delay      time.Duration `json:"delay"`
	Approx     bool          `json:"approx"`
	OllamaURL  string        `json:"ollamaUrl"`
	EmbedModel string        `json:"embedModel"`
	ChatModel  string        `json:"chatModel"`
	Snapshot   int           `json:"snapshotNodes"`
}

// DefaultSettings matches the harvester-server flag defaults exactly. Drifting
// from them would make the TUI and the documented commands disagree about what
// "default" means.
func DefaultSettings() Settings {
	return Settings{
		DataDir:    ".reharvester",
		Addr:       ":8000",
		Project:    "default",
		Categories: "cs.IR,cs.DL,cs.CL,cs.SI,cs.DB",
		From:       2013,
		To:         time.Now().Year(),
		Max:        2000,
		Delay:      harvest.DefaultDelay,
		OllamaURL:  rag.DefaultBaseURL,
		EmbedModel: rag.DefaultEmbedding,
		ChatModel:  rag.DefaultChatModel,
		Snapshot:   httpapi.DefaultSnapshotNodes,
	}
}

// settingsPath keeps preferences next to the corpus, so pointing --data at a
// different directory gives that workspace its own settings rather than
// silently inheriting the last one used.
func settingsPath(dataDir string) string {
	return filepath.Join(dataDir, "tui-settings.json")
}

// LoadSettings reads saved preferences, falling back to defaults for anything
// missing or unreadable. A corrupt file is not worth failing a launch over.
func LoadSettings(dataDir string) Settings {
	s := DefaultSettings()
	s.DataDir = dataDir
	b, err := os.ReadFile(settingsPath(dataDir))
	if err != nil {
		return s
	}
	var saved Settings
	if json.Unmarshal(b, &saved) != nil {
		return s
	}
	saved.DataDir = dataDir // the flag always wins over the saved copy
	return withDefaults(saved)
}

// withDefaults fills zero fields, so a settings file written by an older
// version gains new keys instead of running with empty strings.
func withDefaults(s Settings) Settings {
	d := DefaultSettings()
	if s.Addr == "" {
		s.Addr = d.Addr
	}
	if s.Project == "" {
		s.Project = d.Project
	}
	if s.Categories == "" {
		s.Categories = d.Categories
	}
	if s.From == 0 {
		s.From = d.From
	}
	if s.To == 0 {
		s.To = d.To
	}
	if s.Max == 0 {
		s.Max = d.Max
	}
	if s.Delay == 0 {
		s.Delay = d.Delay
	}
	if s.OllamaURL == "" {
		s.OllamaURL = d.OllamaURL
	}
	if s.EmbedModel == "" {
		s.EmbedModel = d.EmbedModel
	}
	if s.ChatModel == "" {
		s.ChatModel = d.ChatModel
	}
	if s.Snapshot == 0 {
		s.Snapshot = d.Snapshot
	}
	return s
}

// Save persists preferences. Failure is reported to the caller rather than
// swallowed, but is never fatal: the TUI works fine without a settings file.
func (s Settings) Save() error {
	if err := os.MkdirAll(s.DataDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath(s.DataDir), b, 0o644)
}
