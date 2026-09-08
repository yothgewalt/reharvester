package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yothgewalt/reharvester/internal/app"
	"github.com/yothgewalt/reharvester/internal/rag"
	"github.com/yothgewalt/reharvester/internal/store"
)

type listRow struct {
	id     string
	title  string
	detail string
	note   string
	dep    *Dep // when set, enter offers to install
}

type listModel struct {
	eyebrow string
	title   string
	empty   string
	help    string
	rows    []listRow
	idx     int
}

func (l *listModel) view() string {
	var b strings.Builder
	b.WriteString(header(l.eyebrow, l.title))
	if len(l.rows) == 0 {
		b.WriteString(styleMuted.Render(l.empty) + "\n")
	}
	names := make([]string, len(l.rows))
	for i, r := range l.rows {
		names[i] = r.title
	}
	w := columnWidth(names, 2)

	for i, r := range l.rows {
		cursor, style := "  ", styleBody
		if i == l.idx {
			cursor, style = styleSel.Render("› "), styleSel
		}
		b.WriteString(cursor + style.Render(padRight(r.title, w)) + styleMuted.Render(r.detail) + "\n")
		if r.note != "" {
			b.WriteString("    " + styleFaint.Render(r.note) + "\n")
		}
	}
	b.WriteString(helpLine(l.help))
	return b.String()
}

func (l *listModel) move(d int) {
	l.idx += d
	if l.idx < 0 {
		l.idx = 0
	}
	if l.idx >= len(l.rows) {
		l.idx = max(0, len(l.rows)-1)
	}
}

// ---- projects ----

func newProjectList(st *store.Store, active string) listModel {
	l := listModel{
		eyebrow: "PROJECTS",
		title:   "Corpora on disk",
		empty:   "No projects yet — run Harvest to create one.",
		help:    "↑↓ move · enter make active · esc back",
	}
	metas, err := st.Projects()
	if err != nil {
		l.empty = "could not read projects: " + err.Error()
		return l
	}
	for _, mt := range metas {
		detail := fmt.Sprintf("%d papers", mt.DocsIngested)
		if !mt.CreatedAt.IsZero() {
			detail += " · " + mt.CreatedAt.Local().Format("2006-01-02")
		}
		note := mt.Query
		if sp, err := st.Project(mt.ID); err == nil {
			if size, ok := pathSize(filepath.Dir(sp.Path("meta.json"))); ok {
				detail += " · " + humanBytes(size)
			}
			if !sp.Exists("papers.jsonl") {
				note = "no papers.jsonl — harvest did not complete"
			}
		}
		if rec, ok := app.ReadEmbedRecord(st, mt.ID); ok {
			note = strings.TrimSpace(note + "  · encoder " + rec.Model)
		}
		title := mt.ID
		if mt.ID == active {
			title += "  (active)"
		}
		l.rows = append(l.rows, listRow{id: mt.ID, title: title, detail: detail, note: note})
	}
	return l
}

func (m *rootModel) updateProjects(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", "q":
		m.screen = screenMenu
	case "up", "k":
		m.projects.move(-1)
	case "down", "j":
		m.projects.move(1)
	case "enter":
		if len(m.projects.rows) == 0 {
			return m, nil
		}
		id := m.projects.rows[m.projects.idx].id
		m.settings.Project = id
		_ = m.settings.Save()
		m.projects = newProjectList(m.store, id)
		m.status = "active project is now " + id
		m.checking = true
		return m, m.runChecksCmd()
	}
	return m, nil
}

func (m *rootModel) viewProjects() string { return m.projects.view() }

// ---- models ----

// pulledModels asks the model server what it has. It talks to /api/tags
// directly rather than reaching into internal/rag, keeping the package the
// paper's claims rest on untouched.
func pulledModels(ctx context.Context, baseURL string) ([]string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/tags", nil)
	if err != nil {
		return nil, false
	}
	res, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		return nil, false
	}
	defer res.Body.Close()
	var body struct {
		Models []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"models"`
	}
	if json.NewDecoder(res.Body).Decode(&body) != nil {
		return nil, true
	}
	out := make([]string, 0, len(body.Models))
	for _, mm := range body.Models {
		out = append(out, fmt.Sprintf("%s|%s", mm.Name, humanBytes(mm.Size)))
	}
	return out, true
}

func newModelList(s Settings) listModel {
	l := listModel{
		eyebrow: "MODELS",
		title:   "Ollama at " + s.OllamaURL,
		help:    "↑↓ move · enter install or pull · esc back",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	names, up := pulledModels(ctx, s.OllamaURL)
	if !up {
		l.rows = append(l.rows, listRow{
			title: "Ollama", detail: "not reachable",
			note: "the system runs at tier T2 without it — dense retrieval and generated prose are what you gain",
			dep:  depOllama(),
		})
		l.empty = ""
		return l
	}

	has := func(prefix string) bool {
		for _, n := range names {
			if strings.HasPrefix(strings.ToLower(n), strings.ToLower(prefix)) {
				return true
			}
		}
		return false
	}

	encDetail, encNote := "pulled", ""
	var encDep *Dep
	if !has(s.EmbedModel) {
		encDetail, encNote = "not pulled", "tier T3 stays off until this is present"
		encDep = depOllamaModel(s.EmbedModel)
	}
	l.rows = append(l.rows, listRow{
		title: "Encoder · " + s.EmbedModel, detail: encDetail, note: encNote, dep: encDep,
	})

	chatDetail, chatNote := "pulled", ""
	var chatDep *Dep
	if !has(s.ChatModel) {
		chatDetail = "not pulled"
		chatNote = "wiki pages fall back to template synthesis without a generation model"
		chatDep = depOllamaModel(s.ChatModel)
	}
	l.rows = append(l.rows, listRow{
		title: "Chat · " + s.ChatModel, detail: chatDetail, note: chatNote, dep: chatDep,
	})

	l.rows = append(l.rows, listRow{
		title:  "Suggested chat model",
		detail: rag.SuggestChatModel(),
		note:   "used automatically when the configured one is absent",
		dep:    depOllamaModel(rag.SuggestChatModel()),
	})

	for _, n := range names {
		parts := strings.SplitN(n, "|", 2)
		size := ""
		if len(parts) == 2 {
			size = parts[1]
		}
		l.rows = append(l.rows, listRow{title: "  " + parts[0], detail: size})
	}
	return l
}

func (m *rootModel) updateModels(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", "q":
		m.screen = screenMenu
	case "up", "k":
		m.models.move(-1)
	case "down", "j":
		m.models.move(1)
	case "r":
		m.models = newModelList(m.settings)
	case "enter":
		if len(m.models.rows) == 0 {
			return m, nil
		}
		row := m.models.rows[m.models.idx]
		if row.dep == nil {
			m.status = "nothing to do for this one"
			return m, nil
		}
		return m.beginInstall(Check{Name: row.title, Dep: row.dep})
	}
	return m, nil
}

func (m *rootModel) viewModels() string {
	v := m.models.view()
	if rec, ok := app.ReadEmbedRecord(m.store, m.settings.Project); ok && rec.Model != m.settings.EmbedModel {
		v += "\n" + styleDanger.Render(fmt.Sprintf(
			"Corpus was built with %q but %q is configured.", rec.Model, m.settings.EmbedModel)) +
			"\n" + styleWarn.Render(
			"Dense search returns nothing in this state, and /health still reports T3. Rebuild, or set the encoder back.")
	}
	return v
}
