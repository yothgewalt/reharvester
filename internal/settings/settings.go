// Package settings owns the preferences shared between the TUI and the web
// API: listen address, active project, harvest defaults, the model server
// and its API keys. Both cmd/reharvester and the web /settings page read and
// write the same file, <dataDir>/tui-settings.json, so every writer should go
// through Update rather than Save directly — otherwise a change made on one
// surface can be silently overwritten by a stale copy held by the other.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/yothgewalt/reharvester/internal/harvest"
	"github.com/yothgewalt/reharvester/internal/rag"
)

// DefaultSnapshotNodes caps how many papers a graph snapshot carries by
// default; httpapi.DefaultSnapshotNodes aliases this.
const DefaultSnapshotNodes = 4000

// Settings is what the TUI and the web API remember between runs. It is
// deliberately the same set of knobs harvester-server's flags expose, so a
// value chosen here and a value passed on the command line mean the same
// thing.
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
	// Source is one of harvest.Sources; SnapshotPath is the Kaggle arXiv dump
	// the kaggle source reads.
	Source       string `json:"source"`
	SnapshotPath string `json:"arxivSnapshot"`
	// OpenAlexKey, SemanticScholarKey and OllamaKey are API keys. They make the
	// settings file a secret, which is why Save writes it owner-only. OllamaKey
	// only takes effect for a ChatModel ending in -cloud; see rag.NewChat.
	OpenAlexKey        string `json:"openalexKey,omitempty"`
	SemanticScholarKey string `json:"semanticScholarKey,omitempty"`
	OllamaKey          string `json:"ollamaKey,omitempty"`
	Approx             bool   `json:"approx"`
	OllamaURL          string `json:"ollamaUrl"`
	EmbedModel         string `json:"embedModel"`
	ChatModel          string `json:"chatModel"`
	Snapshot           int    `json:"snapshotNodes"`
}

// Default matches the harvester-server flag defaults exactly. Drifting from
// them would make the TUI and the documented commands disagree about what
// "default" means.
func Default() Settings {
	return Settings{
		DataDir:    ".reharvester",
		Addr:       ":8000",
		Project:    "default",
		Categories: "",
		From:       2013,
		To:         time.Now().Year(),
		Max:        2000,
		Delay:      harvest.DefaultDelay,
		Source:     harvest.SourceArxiv,
		OllamaURL:  rag.DefaultBaseURL,
		EmbedModel: rag.DefaultEmbedding,
		ChatModel:  rag.DefaultChatModel,
		Snapshot:   DefaultSnapshotNodes,
	}
}

// Path is where preferences live under a data directory, so pointing --data
// at a different directory gives that workspace its own settings rather than
// silently inheriting the last one used.
func Path(dataDir string) string {
	return filepath.Join(dataDir, "tui-settings.json")
}

// Load reads saved preferences, falling back to defaults for anything
// missing or unreadable. A corrupt file is not worth failing a launch over.
func Load(dataDir string) Settings {
	s := Default()
	s.DataDir = dataDir
	b, err := os.ReadFile(Path(dataDir))
	if err != nil {
		return s
	}
	var saved Settings
	if json.Unmarshal(b, &saved) != nil {
		return s
	}
	saved.DataDir = dataDir // the caller's directory always wins over the saved copy
	return WithDefaults(saved)
}

// WithDefaults fills zero fields, so a settings file written by an older
// version gains new keys instead of running with empty strings.
func WithDefaults(s Settings) Settings {
	d := Default()
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
	if s.Source == "" {
		s.Source = d.Source
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

// Save persists preferences, readable by the owner only because they can hold
// API keys. Failure is reported to the caller rather than swallowed, but is
// never fatal: both the TUI and the API work fine without a settings file.
func (s Settings) Save() error {
	if err := os.MkdirAll(s.DataDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := Path(s.DataDir)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	// WriteFile keeps an existing file's mode, and older versions wrote 0644.
	return os.Chmod(path, 0o600)
}

// Update loads the settings currently on disk, applies fn, and saves the
// result. Every writer — the TUI's forms and the web API's PATCH handler —
// should go through Update rather than Save directly, and fn should change
// only the fields it owns: a concurrent write from the other surface,
// already on disk, survives untouched. The returned Settings is always fn's
// result, even when the save fails; the error reports only the write.
func Update(dataDir string, fn func(*Settings)) (Settings, error) {
	s := Load(dataDir)
	fn(&s)
	return s, s.Save()
}
