package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/yothgewalt/reharvester/internal/app"
	"github.com/yothgewalt/reharvester/internal/store"
)

type screen int

const (
	screenDoctor screen = iota
	screenMenu
	screenConsole
	screenHarvest
	screenProgress
	screenClean
	screenProjects
	screenModels
	screenSettings
	screenInstall
)

type rootModel struct {
	program  *tea.Program
	sink     *logSink
	settings Settings
	store    *store.Store

	screen screen
	w, h   int

	spin spinner.Model

	// Doctor. firstCheck gates the one-time redirect: a later re-check, after
	// a build or an install, must not yank the user off whatever they are on.
	checks     []Check
	checkIdx   int
	checking   bool
	firstCheck bool

	menuIdx int

	console      consoleModel
	form         formModel
	clean        cleanModel
	projects     listModel
	models       listModel
	settingsForm formModel
	install      installModel

	// One job at a time: harvest, build and analyse all write the same project
	// directory, so overlapping them would corrupt it.
	job        string
	jobCancel  context.CancelFunc
	jobStart   time.Time
	jobPct     int
	jobStage   string
	progressCh chan progressMsg

	serverOn     bool
	serverCancel context.CancelFunc

	status string
	err    error
}

type (
	logLineMsg  string
	checksMsg   []Check
	progressMsg struct {
		pct   int
		stage string
	}
	jobDoneMsg struct {
		name string
		err  error
	}
	serverStoppedMsg struct{ err error }
	statusMsg        string
)

func newRootModel(dataDir string) (*rootModel, error) {
	s := LoadSettings(dataDir)
	sink := newLogSink(s.DataDir, "reharvester")
	// The TUI owns the terminal; log output must go to the sink or it would
	// tear the rendered frame apart.
	log.SetOutput(sink)
	log.SetFlags(0)

	st, err := store.Open(s.DataDir)
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colAccent)

	m := &rootModel{
		settings:   s,
		sink:       sink,
		store:      st,
		screen:     screenMenu,
		spin:       sp,
		checking:   true,
		firstCheck: true,
		progressCh: make(chan progressMsg, 64),
	}
	m.console = newConsole(sink)
	return m, nil
}

func (m *rootModel) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, waitLog(m.sink), m.runChecksCmd(), waitProgress(m.progressCh))
}

// waitLog turns the sink's channel into messages. Re-issued after every line,
// which is the standard bubbletea pattern for an external producer.
func waitLog(s *logSink) tea.Cmd {
	return func() tea.Msg { return logLineMsg(<-s.ch) }
}

func waitProgress(ch chan progressMsg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func (m *rootModel) runChecksCmd() tea.Cmd {
	s := m.settings
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return checksMsg(runChecks(ctx, s))
	}
}

func (m *rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.console.resize(msg.Width-2*pagePadX, msg.Height-8-2*pagePadY)
		return m, nil

	case logLineMsg:
		m.console.append(string(msg))
		return m, waitLog(m.sink)

	case progressMsg:
		m.jobPct, m.jobStage = msg.pct, msg.stage
		return m, waitProgress(m.progressCh)

	case checksMsg:
		m.checks, m.checking = msg, false
		if m.checkIdx >= len(m.checks) {
			m.checkIdx = 0
		}
		// Open the Doctor unprompted only when something is actually broken.
		// A StatusWarn is an optional capability the user chose not to install,
		// which is the normal state and no reason to interrupt a launch.
		if m.firstCheck {
			m.firstCheck = false
			if i, n := firstBroken(m.checks); n > 0 {
				m.checkIdx = i
				m.screen = screenDoctor
				m.status = fmt.Sprintf("%s — esc for the menu", pluralChecks(n))
			}
		}
		return m, nil

	case jobDoneMsg:
		m.job, m.jobCancel = "", nil
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.name + " failed: " + msg.err.Error()
		} else {
			m.status = fmt.Sprintf("%s finished in %s", msg.name, shortDur(time.Since(m.jobStart)))
		}
		return m, m.runChecksCmd()

	case serverStoppedMsg:
		m.serverOn, m.serverCancel = false, nil
		if msg.err != nil {
			m.status = "server stopped: " + msg.err.Error()
		} else {
			m.status = "server stopped"
		}
		return m, nil

	case statusMsg:
		m.status = string(msg)
		return m, nil

	case installStepDoneMsg:
		return m.onInstallStepDone(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *rootModel) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ctrl+c cancels the running job rather than killing the process, because
	// a harvest holds minutes of politeness-limited network and a build is
	// mid-write to the project directory.
	if k.Type == tea.KeyCtrlC {
		switch {
		case m.job != "" && m.jobCancel != nil:
			m.jobCancel()
			m.status = "cancelling " + m.job + "…"
			return m, nil
		case m.serverOn && m.serverCancel != nil:
			m.serverCancel()
			return m, nil
		default:
			return m, m.quit()
		}
	}

	switch m.screen {
	case screenDoctor:
		return m.updateDoctor(k)
	case screenMenu:
		return m.updateMenu(k)
	case screenConsole:
		return m.updateConsole(k)
	case screenHarvest:
		return m.updateForm(k)
	case screenSettings:
		return m.updateSettings(k)
	case screenProgress:
		if k.String() == "esc" && m.job == "" {
			m.screen = screenMenu
		}
		return m, nil
	case screenClean:
		return m.updateClean(k)
	case screenProjects:
		return m.updateProjects(k)
	case screenModels:
		return m.updateModels(k)
	case screenInstall:
		return m.updateInstall(k)
	}
	return m, nil
}

func (m *rootModel) quit() tea.Cmd {
	if m.serverCancel != nil {
		m.serverCancel()
	}
	if m.jobCancel != nil {
		m.jobCancel()
	}
	_ = m.settings.Save()
	return tea.Quit
}

func (m *rootModel) View() string {
	body := m.screenBody()
	if m.status != "" {
		body += "\n" + styleMuted.Render(m.status)
	}
	return stylePage.Render(body)
}

// screenBody renders the active screen without the page padding, which is the
// form assertions want: the padding legitimately pads every line to a common
// width, and would mask a stray whitespace-only line.
func (m *rootModel) screenBody() string {
	var body string
	switch m.screen {
	case screenDoctor:
		body = m.viewDoctor()
	case screenMenu:
		body = m.viewMenu()
	case screenConsole:
		body = m.viewConsole()
	case screenHarvest:
		body = m.viewForm()
	case screenSettings:
		body = m.viewSettings()
	case screenProgress:
		body = m.viewProgress()
	case screenClean:
		body = m.viewClean()
	case screenProjects:
		body = m.viewProjects()
	case screenModels:
		body = m.viewModels()
	case screenInstall:
		body = m.viewInstall()
	}
	return body
}

// contentWidth is the usable width inside the page padding. Screens that
// truncate lines measure against this, not the raw terminal width, or long
// lines wrap into the right-hand padding.
func (m *rootModel) contentWidth() int {
	if m.w == 0 {
		return 80
	}
	return m.w - 2*pagePadX
}

// ---- menu ----

type menuItem struct {
	key   string
	title string
	desc  string
	// why is non-empty when the item cannot run, and is shown in place of desc.
	why string
}

func (m *rootModel) menu() []menuItem {
	corpus := app.PickProject(m.store, m.settings.Project) != ""
	noCorpus := ""
	if !corpus {
		noCorpus = "no corpus yet — run Harvest first"
	}
	busy := ""
	if m.job != "" {
		busy = m.job + " is running"
	}
	goOK, _ := detectGo()
	bunOK, _ := detectVersion("bun", "--version")
	srcWhy := ""
	switch {
	case !goOK && !bunOK:
		srcWhy = "needs Go and bun — install them from Doctor"
	case !goOK:
		srcWhy = "needs Go — install it from Doctor"
	case !bunOK:
		srcWhy = "needs bun — install it from Doctor"
	}

	serverTitle := "Start server"
	serverDesc := "serve the API" + ternary(uiEmbedded(), " and UI", "") + " on " + m.settings.Addr
	if m.serverOn {
		serverTitle = "Stop server"
		serverDesc = "running at " + browseURL(m.settings.Addr)
	}

	return []menuItem{
		{"s", serverTitle, serverDesc, ""},
		{"c", "Console", "live server, request and job output", ""},
		{"h", "Harvest", "fetch a corpus from arXiv", busy},
		{"b", "Build", "indexes, backbone and analytics", firstNonEmpty(busy, noCorpus)},
		{"p", "Projects", "switch and inspect corpora", ""},
		{"m", "Models", "Ollama status and model pulls", ""},
		{"a", "Analyse", "trends and gap candidates", firstNonEmpty(busy, noCorpus)},
		{"r", "Build from source", "rebuild the UI and binary", srcWhy},
		{"d", "Doctor", m.doctorSummary(), ""},
		{"t", "Settings", "data directory, port, models", ""},
		{"x", "Clean data", "remove artefacts or reset entirely", busy},
		{"q", "Quit", "", ""},
	}
}

func (m *rootModel) updateMenu(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.menu()
	switch k.String() {
	case "up", "k":
		if m.menuIdx > 0 {
			m.menuIdx--
		}
	case "down", "j":
		if m.menuIdx < len(items)-1 {
			m.menuIdx++
		}
	case "enter":
		return m.activate(items[m.menuIdx])
	default:
		for _, it := range items {
			if k.String() == it.key {
				return m.activate(it)
			}
		}
	}
	return m, nil
}

func (m *rootModel) activate(it menuItem) (tea.Model, tea.Cmd) {
	if it.why != "" {
		m.status = it.why
		return m, nil
	}
	m.err, m.status = nil, ""
	switch it.key {
	case "s":
		return m, m.toggleServer()
	case "c":
		m.screen = screenConsole
	case "h":
		m.form = newHarvestForm(m.settings)
		m.screen = screenHarvest
	case "b":
		return m, m.startJob("build", m.runBuild)
	case "a":
		return m, m.startJob("analyse", m.runAnalyze)
	case "r":
		return m, m.startJob("source build", m.runSourceBuild)
	case "p":
		m.projects = newProjectList(m.store, m.settings.Project)
		m.screen = screenProjects
	case "m":
		m.models = newModelList(m.settings)
		m.screen = screenModels
	case "d":
		m.checking = true
		m.screen = screenDoctor
		return m, m.runChecksCmd()
	case "t":
		m.settingsForm = newSettingsForm(m.settings)
		m.screen = screenSettings
	case "x":
		m.clean = newCleanModel(m.store, m.settings)
		m.screen = screenClean
	case "q":
		return m, m.quit()
	}
	return m, nil
}

func (m *rootModel) viewMenu() string {
	var b strings.Builder
	if art := banner(m.contentWidth()); art != "" {
		b.WriteString(art + "\n\n")
		b.WriteString(styleFaint.Render(
			"v"+app.Version+"  ·  local-first research-literature discovery") + "\n\n")
	} else {
		b.WriteString(header("REHARVESTER "+app.Version, "What would you like to do?"))
	}
	items := m.menu()
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.title
	}
	w := columnWidth(names, 2)

	for i, it := range items {
		cursor, titleStyle := "  ", styleBody
		if i == m.menuIdx {
			cursor, titleStyle = styleSel.Render("› "), styleSel
		}
		desc, descStyle := it.desc, styleFaint
		if it.why != "" {
			desc, descStyle = it.why, styleWarn
			if i != m.menuIdx {
				titleStyle = styleFaint
			}
		}
		b.WriteString(cursor + styleFaint.Render(it.key) + "  " +
			titleStyle.Render(padRight(it.title, w)) + descStyle.Render(desc) + "\n")
	}
	b.WriteString(helpLine("↑↓ move · enter select · or press a letter · ctrl+c quit"))
	if m.err != nil {
		b.WriteString("\n" + styleDanger.Render(m.err.Error()))
	}
	return b.String()
}

// firstBroken returns the index of the first genuinely failing check and how
// many there are. Warnings do not count: they are capabilities the system is
// designed to run without.
func firstBroken(checks []Check) (idx, n int) {
	idx = -1
	for i, c := range checks {
		if c.Status == StatusMissing {
			if idx < 0 {
				idx = i
			}
			n++
		}
	}
	if idx < 0 {
		idx = 0
	}
	return idx, n
}

func pluralChecks(n int) string {
	if n == 1 {
		return "1 check needs attention"
	}
	return fmt.Sprintf("%d checks need attention", n)
}

// doctorSummary describes machine state on the menu row, so the user learns
// what is missing without being sent to the Doctor to find out.
func (m *rootModel) doctorSummary() string {
	if len(m.checks) == 0 {
		return "check dependencies and install what is missing"
	}
	var warn, missing int
	for _, c := range m.checks {
		switch c.Status {
		case StatusWarn:
			warn++
		case StatusMissing:
			missing++
		}
	}
	switch {
	case missing > 0 && warn > 0:
		return fmt.Sprintf("%s, %d optional not installed", pluralChecks(missing), warn)
	case missing > 0:
		return pluralChecks(missing)
	case warn == 1:
		return "1 optional capability not installed"
	case warn > 1:
		return fmt.Sprintf("%d optional capabilities not installed", warn)
	}
	return "everything this machine can do is available"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
