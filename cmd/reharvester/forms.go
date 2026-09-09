package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// formModel is a plain vertical field list. Values are parsed on submit rather
// than on every keystroke, so a half-typed number is not an error.
type formModel struct {
	title   string
	eyebrow string
	submit  string
	fields  []formField
	idx     int
	err     string
}

type formField struct {
	label string
	hint  string
	input textinput.Model
	apply func(s *Settings, v string) error
}

func newField(label, hint, value string, apply func(*Settings, string) error) formField {
	in := textinput.New()
	in.SetValue(value)
	in.Prompt = ""
	in.CharLimit = 200
	in.Width = 48
	return formField{label: label, hint: hint, input: in, apply: apply}
}

func newHarvestForm(s Settings) formModel {
	f := formModel{
		eyebrow: "HARVEST",
		title:   "Fetch a corpus from arXiv",
		submit:  "start harvest",
		fields: []formField{
			newField("Project", "directory name under the data dir", s.Project,
				func(s *Settings, v string) error {
					if strings.TrimSpace(v) == "" {
						return fmt.Errorf("project cannot be empty")
					}
					s.Project = strings.TrimSpace(v)
					return nil
				}),
			newField("Categories", "optional, blank infers them from the keywords", s.Categories,
				func(s *Settings, v string) error { s.Categories = v; return nil }),
			newField("Keywords", "each one is queried separately for an equal share", s.Keywords,
				func(s *Settings, v string) error {
					if len(SplitTrim(v)) == 0 && len(SplitTrim(s.Categories)) == 0 {
						return fmt.Errorf("give at least one keyword, or a category to search")
					}
					s.Keywords = v
					return nil
				}),
			newField("From year", "earliest submission year", strconv.Itoa(s.From),
				func(s *Settings, v string) error { return setYear(&s.From, v) }),
			newField("To year", "latest submission year", strconv.Itoa(s.To),
				func(s *Settings, v string) error { return setYear(&s.To, v) }),
			newField("Max records", "upper bound on papers retained", strconv.Itoa(s.Max),
				func(s *Settings, v string) error {
					n, err := strconv.Atoi(strings.TrimSpace(v))
					if err != nil || n <= 0 {
						return fmt.Errorf("max must be a positive number")
					}
					s.Max = n
					return nil
				}),
		},
	}
	f.fields[0].input.Focus()
	return f
}

func newSettingsForm(s Settings) formModel {
	f := formModel{
		eyebrow: "SETTINGS",
		title:   "Saved alongside your corpus",
		submit:  "save",
		fields: []formField{
			newField("Listen address", "host:port for the API and UI", s.Addr,
				func(s *Settings, v string) error {
					if strings.TrimSpace(v) == "" {
						return fmt.Errorf("address cannot be empty")
					}
					s.Addr = strings.TrimSpace(v)
					return nil
				}),
			newField("Project", "active project id", s.Project,
				func(s *Settings, v string) error { s.Project = strings.TrimSpace(v); return nil }),
			newField("Ollama URL", "local model server", s.OllamaURL,
				func(s *Settings, v string) error { s.OllamaURL = strings.TrimSpace(v); return nil }),
			newField("Encoder model", "changing this needs a rebuild", s.EmbedModel,
				func(s *Settings, v string) error { s.EmbedModel = strings.TrimSpace(v); return nil }),
			newField("Chat model", "wiki synthesis and answers", s.ChatModel,
				func(s *Settings, v string) error { s.ChatModel = strings.TrimSpace(v); return nil }),
			newField("Politeness delay", "between arXiv requests, e.g. 3s", s.Delay.String(),
				func(s *Settings, v string) error {
					d, err := time.ParseDuration(strings.TrimSpace(v))
					if err != nil || d < 0 {
						return fmt.Errorf("delay must be a duration like 3s")
					}
					s.Delay = d
					return nil
				}),
			newField("Snapshot nodes", "papers per graph snapshot", strconv.Itoa(s.Snapshot),
				func(s *Settings, v string) error {
					n, err := strconv.Atoi(strings.TrimSpace(v))
					if err != nil || n <= 0 {
						return fmt.Errorf("snapshot nodes must be positive")
					}
					s.Snapshot = n
					return nil
				}),
		},
	}
	f.fields[0].input.Focus()
	return f
}

func setYear(dst *int, v string) error {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 1990 || n > 2100 {
		return fmt.Errorf("year must be between 1990 and 2100")
	}
	*dst = n
	return nil
}

// SplitTrim is the form-side twin of app.SplitList, kept here so validation
// does not depend on the pipeline package.
func SplitTrim(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (f *formModel) focus(i int) {
	for j := range f.fields {
		if j == i {
			f.fields[j].input.Focus()
		} else {
			f.fields[j].input.Blur()
		}
	}
	f.idx = i
}

// commit applies every field to a copy, so a validation failure late in the
// form leaves the live settings untouched.
func (f *formModel) commit(cur Settings) (Settings, bool) {
	next := cur
	for _, fl := range f.fields {
		if err := fl.apply(&next, fl.input.Value()); err != nil {
			f.err = err.Error()
			return cur, false
		}
	}
	f.err = ""
	return next, true
}

func (f *formModel) view() string {
	var b strings.Builder
	b.WriteString(header(f.eyebrow, f.title))

	names := make([]string, len(f.fields))
	for i, fl := range f.fields {
		names[i] = fl.label
	}
	w := columnWidth(names, 2)

	for i, fl := range f.fields {
		cursor, style := "  ", styleMuted
		if i == f.idx {
			cursor, style = styleSel.Render("› "), styleSel
		}
		b.WriteString(cursor + style.Render(padRight(fl.label, w)) + fl.input.View() + "\n")
		if i == f.idx && fl.hint != "" {
			b.WriteString("    " + styleFaint.Render(fl.hint) + "\n")
		}
	}
	if f.err != "" {
		b.WriteString("\n" + styleDanger.Render(f.err))
	}
	b.WriteString(helpLine("tab/↑↓ move · enter " + f.submit + " · esc cancel"))
	return b.String()
}

func (m *rootModel) updateForm(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := &m.form
	switch k.String() {
	case "esc":
		m.screen = screenMenu
		return m, nil
	case "tab", "down":
		f.focus((f.idx + 1) % len(f.fields))
		return m, nil
	case "shift+tab", "up":
		f.focus((f.idx - 1 + len(f.fields)) % len(f.fields))
		return m, nil
	case "enter":
		next, ok := f.commit(m.settings)
		if !ok {
			return m, nil
		}
		m.settings = next
		_ = m.settings.Save()
		return m, m.startJob("harvest", m.runHarvest)
	}
	var cmd tea.Cmd
	f.fields[f.idx].input, cmd = f.fields[f.idx].input.Update(k)
	return m, cmd
}

func (m *rootModel) viewForm() string { return m.form.view() }

func (m *rootModel) updateSettings(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := &m.settingsForm
	switch k.String() {
	case "esc":
		m.screen = screenMenu
		return m, nil
	case "tab", "down":
		f.focus((f.idx + 1) % len(f.fields))
		return m, nil
	case "shift+tab", "up":
		f.focus((f.idx - 1 + len(f.fields)) % len(f.fields))
		return m, nil
	case "enter":
		next, ok := f.commit(m.settings)
		if !ok {
			return m, nil
		}
		changedEncoder := next.EmbedModel != m.settings.EmbedModel
		m.settings = next
		if err := m.settings.Save(); err != nil {
			m.status = "could not save settings: " + err.Error()
		} else {
			m.status = "settings saved"
		}
		if changedEncoder {
			m.status = "encoder changed — rebuild before serving, or dense search returns nothing"
		}
		m.screen = screenMenu
		m.checking = true
		return m, m.runChecksCmd()
	}
	var cmd tea.Cmd
	f.fields[f.idx].input, cmd = f.fields[f.idx].input.Update(k)
	return m, cmd
}

func (m *rootModel) viewSettings() string {
	return m.settingsForm.view() + "\n" +
		styleFaint.Render("data directory: "+absOr(m.settings.DataDir)+"  (set with --data at launch)")
}
