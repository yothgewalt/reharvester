package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/yothgewalt/reharvester/internal/store"
)

// cleanAction is one row of the reset screen. Confirm is non-empty when the
// action destroys something a rebuild cannot recover — the user types that
// exact word, because a y/n keypress is too easy to hit by reflex.
type cleanAction struct {
	title   string
	detail  string
	undo    string
	targets []string // absolute paths, resolved up front and shown to the user
	confirm string
}

type cleanModel struct {
	actions []cleanAction
	idx     int
	typing  bool
	input   textinput.Model
	result  string
}

func newCleanModel(st *store.Store, s Settings) cleanModel {
	dataDir := absOr(s.DataDir)
	projectID := s.Project
	projDir := filepath.Join(dataDir, "projects", projectID)

	actions := []cleanAction{
		{
			title:  "Clear embeddings",
			detail: "the dense vectors for " + projectID,
			undo:   "rebuild — minutes of encoding, no network",
			targets: []string{
				filepath.Join(projDir, "embeddings.bin"),
				filepath.Join(projDir, "embedding.json"),
			},
		},
		{
			title:  "Clear derived artefacts",
			detail: "graph, analytics and vectors for " + projectID + "; keeps the papers",
			undo:   "rebuild — seconds",
			targets: []string{
				filepath.Join(projDir, "graph.json"),
				filepath.Join(projDir, "analytics.json"),
				filepath.Join(projDir, "embeddings.bin"),
				filepath.Join(projDir, "embedding.json"),
			},
		},
		{
			title:   "Delete project " + projectID,
			detail:  "every artefact including the harvested papers",
			undo:    "re-harvest — networked, rate-limited to about 3 s per 200 records",
			targets: []string{projDir},
			confirm: projectID,
		},
		{
			title:   "Factory reset",
			detail:  "the whole data directory: every project, log and setting",
			undo:    "re-harvest everything from scratch",
			targets: []string{dataDir},
			confirm: "RESET",
		},
	}

	in := textinput.New()
	in.Prompt = ""
	in.Width = 32
	return cleanModel{actions: actions, input: in}
}

func (m *rootModel) updateClean(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := &m.clean
	if c.typing {
		switch k.String() {
		case "esc":
			c.typing = false
			c.input.Blur()
			c.input.SetValue("")
			return m, nil
		case "enter":
			act := c.actions[c.idx]
			if strings.TrimSpace(c.input.Value()) != act.confirm {
				c.result = "that does not match — nothing was deleted"
				return m, nil
			}
			c.typing = false
			c.input.Blur()
			c.input.SetValue("")
			return m, m.performClean(act)
		}
		var cmd tea.Cmd
		c.input, cmd = c.input.Update(k)
		return m, cmd
	}

	switch k.String() {
	case "esc", "q":
		m.screen = screenMenu
	case "up", "k":
		if c.idx > 0 {
			c.idx--
			c.result = ""
		}
	case "down", "j":
		if c.idx < len(c.actions)-1 {
			c.idx++
			c.result = ""
		}
	case "enter", "d":
		if why := m.cleanBlocked(); why != "" {
			c.result = why
			return m, nil
		}
		act := c.actions[c.idx]
		if act.confirm != "" {
			c.typing = true
			c.input.Focus()
			return m, nil
		}
		return m, m.performClean(act)
	}
	return m, nil
}

// cleanBlocked refuses to delete underneath a running process. The server
// holds an open corpus and a job is mid-write; removing files under either
// produces corruption that looks like a bug in the pipeline.
func (m *rootModel) cleanBlocked() string {
	if m.serverOn {
		return "stop the server first — it has the corpus open"
	}
	if m.job != "" {
		return m.job + " is running — let it finish or cancel it first"
	}
	return ""
}

func (m *rootModel) performClean(act cleanAction) tea.Cmd {
	var removed, failed int
	for _, t := range act.targets {
		if _, err := os.Stat(t); err != nil {
			continue
		}
		if err := os.RemoveAll(t); err != nil {
			m.sink.Push("clean: could not remove " + t + ": " + err.Error())
			failed++
			continue
		}
		m.sink.Push("clean: removed " + t)
		removed++
	}
	switch {
	case failed > 0:
		m.clean.result = fmt.Sprintf("removed %d, failed %d — see the console", removed, failed)
	case removed == 0:
		m.clean.result = "nothing to remove; those paths do not exist"
	default:
		m.clean.result = fmt.Sprintf("removed %d path(s)", removed)
	}
	// The store caches nothing across a delete, but the checks certainly change.
	m.clean = newCleanModel(m.store, m.settings)
	m.checking = true
	return m.runChecksCmd()
}

func (m *rootModel) viewClean() string {
	c := &m.clean
	var b strings.Builder
	b.WriteString(header("CLEAN DATA", "Remove artefacts, or reset entirely"))

	if why := m.cleanBlocked(); why != "" {
		b.WriteString(styleWarn.Render("! "+why) + "\n\n")
	}

	for i, a := range c.actions {
		cursor := "  "
		title := styleBody.Render(a.title)
		if i == c.idx {
			cursor = styleSel.Render("› ")
			title = styleSel.Render(a.title)
		}
		if a.confirm != "" {
			title = styleDanger.Render(a.title)
			if i == c.idx {
				title = styleDanger.Render("› " + a.title)
				cursor = ""
			}
		}
		b.WriteString(cursor + title + "\n")
		b.WriteString("    " + styleMuted.Render(a.detail) + "\n")
		if i == c.idx {
			for _, t := range a.targets {
				size, exists := pathSize(t)
				if !exists {
					b.WriteString("    " + styleFaint.Render(t+"  (not present)") + "\n")
					continue
				}
				b.WriteString("    " + styleFaint.Render(fmt.Sprintf("%s  %s", t, humanBytes(size))) + "\n")
			}
			b.WriteString("    " + styleWarn.Render("to undo: "+a.undo) + "\n")
		}
	}

	if c.typing {
		act := c.actions[c.idx]
		b.WriteString("\n" + styleDanger.Render("This cannot be undone.") + "\n")
		b.WriteString(styleBody.Render("Type ") + styleCmd.Render(act.confirm) +
			styleBody.Render(" to confirm: ") + c.input.View() + "\n")
		b.WriteString(helpLine("enter confirm · esc cancel"))
		return b.String()
	}

	if c.result != "" {
		b.WriteString("\n" + styleMuted.Render(c.result))
	}
	b.WriteString(helpLine("↑↓ move · enter run the selected action · esc back"))
	return b.String()
}

// pathSize measures a file or directory. Reporting the real number is the
// point: "delete 1.4 GB" reads differently from "delete project".
func pathSize(p string) (int64, bool) {
	info, err := os.Stat(p)
	if err != nil {
		return 0, false
	}
	if !info.IsDir() {
		return info.Size(), true
	}
	var total int64
	_ = filepath.Walk(p, func(_ string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	return total, true
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
