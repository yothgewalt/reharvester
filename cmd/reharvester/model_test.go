package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/yothgewalt/reharvester/internal/store"
)

// newTestModel builds a model against a throwaway data directory. The TUI
// redirects the standard logger, so the original is restored on cleanup to
// keep test output readable.
func newTestModel(t *testing.T) *rootModel {
	t.Helper()
	dir := t.TempDir()
	out, flags := log.Writer(), log.Flags()
	t.Cleanup(func() {
		log.SetOutput(out)
		log.SetFlags(flags)
	})
	m, err := newRootModel(dir)
	if err != nil {
		t.Fatalf("newRootModel: %v", err)
	}
	t.Cleanup(m.sink.Close)
	m.w, m.h = 100, 40
	return m
}

// TestEveryScreenRenders pins that no screen panics or renders blank. A View
// that dereferences an uninitialised sub-model is the failure mode here, and
// it only shows up when a user navigates to that screen.
type screenCase struct {
	name   string
	screen screen
	setup  func(m *rootModel)
	want   string
}

func screenCases() []screenCase {
	return []screenCase{
		{"doctor", screenDoctor, func(m *rootModel) {
			m.checks = []Check{{Name: "Ollama", Unlocks: "tier T3", Status: StatusWarn, Detail: "no server"}}
		}, "Ollama"},
		{"menu", screenMenu, nil, "Start server"},
		{"console", screenConsole, nil, "CONSOLE"},
		{"harvest", screenHarvest, func(m *rootModel) {
			m.form = newHarvestForm(m.settings)
		}, "Categories"},
		{"settings", screenSettings, func(m *rootModel) {
			m.settingsForm = newSettingsForm(m.settings)
		}, "Encoder model"},
		{"progress", screenProgress, func(m *rootModel) {
			m.job, m.jobStage, m.jobPct = "build", "indexing", 40
		}, "indexing"},
		{"clean", screenClean, func(m *rootModel) {
			m.clean = newCleanModel(m.store, m.settings)
		}, "Factory reset"},
		{"projects", screenProjects, func(m *rootModel) {
			m.projects = newProjectList(m.store, m.settings.Project)
		}, "PROJECTS"},
		{"models", screenModels, func(m *rootModel) {
			m.models = listModel{eyebrow: "MODELS", title: "t", empty: "none"}
		}, "MODELS"},
		{"install", screenInstall, func(m *rootModel) {
			dep := depOllamaModel("all-minilm")
			plan, _ := dep.Resolve()
			m.install = installModel{dep: dep, plan: plan, resolved: true}
		}, "all-minilm"},
	}
}

func TestEveryScreenRenders(t *testing.T) {
	for _, tc := range screenCases() {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(t)
			if tc.setup != nil {
				tc.setup(m)
			}
			m.screen = tc.screen
			got := m.View()
			if strings.TrimSpace(got) == "" {
				t.Fatal("rendered nothing")
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("missing %q in render:\n%s", tc.want, got)
			}
		})
	}
}

// TestMenuExplainsWhyDisabled covers the promise that an unavailable action
// says why up front instead of failing after it is chosen.
func TestMenuExplainsWhyDisabled(t *testing.T) {
	m := newTestModel(t)
	byKey := map[string]menuItem{}
	for _, it := range m.menu() {
		byKey[it.key] = it
	}
	if byKey["b"].why == "" {
		t.Error("Build should be disabled with no corpus")
	}
	// Selecting it must report the reason rather than starting a job.
	m.screen = screenMenu
	for i, it := range m.menu() {
		if it.key == "b" {
			m.menuIdx = i
		}
	}
	next, _ := m.updateMenu(tea.KeyMsg{Type: tea.KeyEnter})
	rm := next.(*rootModel)
	if rm.job != "" {
		t.Fatalf("started job %q despite missing corpus", rm.job)
	}
	if rm.status == "" {
		t.Fatal("no reason surfaced to the user")
	}
}

// TestCleanRefusesWhileBusy guards the destructive screen: removing files
// under a running server or a mid-write job corrupts the project directory.
func TestCleanRefusesWhileBusy(t *testing.T) {
	m := newTestModel(t)
	if why := m.cleanBlocked(); why != "" {
		t.Fatalf("idle model should allow cleaning, got %q", why)
	}
	m.serverOn = true
	if m.cleanBlocked() == "" {
		t.Error("must refuse while the server holds the corpus")
	}
	m.serverOn, m.job = false, "harvest"
	if m.cleanBlocked() == "" {
		t.Error("must refuse while a job is writing")
	}
}

// TestCleanTargetsAreAbsoluteAndScoped pins that the screen shows real paths
// and never proposes deleting something outside the data directory.
func TestCleanTargetsAreAbsoluteAndScoped(t *testing.T) {
	m := newTestModel(t)
	root, err := filepath.Abs(m.settings.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	c := newCleanModel(m.store, m.settings)
	for _, a := range c.actions {
		if len(a.targets) == 0 {
			t.Errorf("%q has no targets", a.title)
		}
		for _, target := range a.targets {
			if !filepath.IsAbs(target) {
				t.Errorf("%q target %q is not absolute", a.title, target)
			}
			if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
				t.Errorf("%q target %q escapes the data directory %q", a.title, target, root)
			}
		}
	}
}

// TestIrreversibleActionsRequireTypedConfirmation: the two rows that cannot be
// undone by a rebuild must demand a typed word, not a keypress.
func TestIrreversibleActionsRequireTypedConfirmation(t *testing.T) {
	m := newTestModel(t)
	c := newCleanModel(m.store, m.settings)
	var typed int
	for _, a := range c.actions {
		if a.confirm != "" {
			typed++
		}
	}
	if typed != 2 {
		t.Fatalf("expected 2 typed-confirmation actions, got %d", typed)
	}
}

// TestCleanRemovesOnlyWhatItNames verifies the delete actually happens and
// stays inside its target.
func TestCleanRemovesOnlyWhatItNames(t *testing.T) {
	m := newTestModel(t)
	proj, err := m.store.Project(m.settings.Project)
	if err != nil {
		t.Fatal(err)
	}
	vectors := proj.Path("embeddings.bin")
	papers := proj.Path("papers.jsonl")
	for _, p := range []string{vectors, papers} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := newCleanModel(m.store, m.settings)
	m.performClean(c.actions[0]) // clear embeddings

	if _, err := os.Stat(vectors); !os.IsNotExist(err) {
		t.Error("embeddings.bin should have been removed")
	}
	if _, err := os.Stat(papers); err != nil {
		t.Error("papers.jsonl must survive clearing embeddings")
	}
}

func TestBrowseURL(t *testing.T) {
	cases := map[string]string{
		":8000":          "http://localhost:8000",
		"0.0.0.0:8000":   "http://localhost:8000",
		"127.0.0.1:9999": "http://127.0.0.1:9999",
		"localhost:8080": "http://localhost:8080",
	}
	for in, want := range cases {
		if got := browseURL(in); got != want {
			t.Errorf("browseURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := DefaultSettings()
	s.DataDir = dir
	s.Addr = ":9123"
	s.EmbedModel = "nomic-embed-text"
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := LoadSettings(dir)
	if got.Addr != ":9123" || got.EmbedModel != "nomic-embed-text" {
		t.Fatalf("round trip lost values: %+v", got)
	}
	if got.Delay != s.Delay {
		t.Errorf("delay = %v, want %v", got.Delay, s.Delay)
	}
}

// TestLoadSettingsSurvivesCorruption: a bad settings file must not stop the
// program from starting.
func TestLoadSettingsSurvivesCorruption(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(settingsPath(dir), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadSettings(dir)
	if got.Addr != DefaultSettings().Addr {
		t.Fatalf("expected defaults, got %+v", got)
	}
}

// TestDepPlansCoverEveryPlatform: a dependency with no plan for an OS leaves
// the user stuck, so each installable names one for all three.
func TestDepPlansCoverEveryPlatform(t *testing.T) {
	deps := map[string]*Dep{
		"ollama": depOllama(),
		"go":     depGo(),
		"bun":    depBun(),
		"git":    depGit(),
		"model":  depOllamaModel("all-minilm"),
	}
	for name, d := range deps {
		if d.Docs == "" {
			t.Errorf("%s has no documentation URL to fall back on", name)
		}
		for _, goos := range []string{"darwin", "linux", "windows"} {
			found := false
			for _, p := range d.Plans {
				if p.GOOS == goos {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s has no plan for %s", name, goos)
			}
		}
	}
}

// TestRiskyStepsAreMarked: sudo and curl-into-shell steps must be flagged so
// the confirmation can warn about them.
func TestRiskyStepsAreMarked(t *testing.T) {
	for _, d := range []*Dep{depOllama(), depGo(), depBun()} {
		for _, p := range d.Plans {
			for _, s := range p.Steps {
				pipesRemote := strings.Contains(s.Shell, "curl") && strings.Contains(s.Shell, "|")
				if (s.NeedsSudo || pipesRemote) && !s.Risky() {
					t.Errorf("%s/%s: step %q is not marked risky", d.Name, p.GOOS, s.Display())
				}
				if s.Prompt() == "" {
					t.Errorf("%s/%s: step renders no prompt", d.Name, p.GOOS)
				}
			}
		}
	}
}

func TestConsoleTabsPartitionOutput(t *testing.T) {
	c := newConsole(newLogSink(t.TempDir(), "test"))
	c.append("14:00:00 http: 200 GET /health (0s)")
	c.append("14:00:01 api: listening on :8000")
	c.append("14:00:02 build: tier T2, total 1.2s")

	for i, tab := range consoleTabs {
		c.tab = i
		got := len(c.visible())
		switch tab.name {
		case "All":
			if got != 3 {
				t.Errorf("All tab shows %d lines, want 3", got)
			}
		case "Access":
			if got != 1 {
				t.Errorf("Access tab shows %d lines, want 1", got)
			}
		case "Jobs":
			if got != 1 {
				t.Errorf("Jobs tab shows %d lines, want 1", got)
			}
		case "Server":
			if got != 1 {
				t.Errorf("Server tab shows %d lines, want 1", got)
			}
		}
	}
}

// TestLogSinkNeverBlocks: the sink is written by the API and the pipeline. If
// a full channel could block, a slow UI would stall the server.
func TestLogSinkNeverBlocks(t *testing.T) {
	s := newLogSink(t.TempDir(), "test")
	defer s.Close()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 5000; i++ {
			s.Push("line")
		}
		close(done)
	}()
	select {
	case <-done:
	case <-timeoutAfter():
		t.Fatal("writing to the sink blocked with nothing draining it")
	}
	if got := len(s.Lines()); got > logRing {
		t.Errorf("ring grew to %d, cap is %d", got, logRing)
	}
}

// timeoutAfter bounds the non-blocking check above.
func timeoutAfter() <-chan time.Time { return time.After(5 * time.Second) }

// TestHeaderDoesNotIndentWhatFollows is a regression test. lipgloss.JoinVertical
// pads every line to the width of the widest, so building the header with a
// trailing empty element produced a run of spaces with no newline after it —
// and the first menu row rendered indented to the title's width.
func TestHeaderDoesNotIndentWhatFollows(t *testing.T) {
	h := header("REHARVESTER 0.1.0", "What would you like to do?")
	if !strings.HasSuffix(h, "\n\n") {
		t.Fatalf("header must end with a blank line, got %q", h[max(0, len(h)-12):])
	}
	for i, line := range strings.Split(h, "\n") {
		if line != strings.TrimRight(line, " ") {
			t.Errorf("line %d has trailing padding: %q", i, line)
		}
	}
}

// TestMenuTitlesShareAColumn: every title must begin at the same column. The
// indent bug pushed only the first row out to the width of the heading, so a
// test comparing rows to each other is what catches it.
func TestMenuTitlesShareAColumn(t *testing.T) {
	m := newTestModel(t)
	m.screen = screenMenu
	rendered := stripANSI(m.viewMenu())

	var cols []int
	for _, it := range m.menu() {
		// Match the key and title together, not the bare title: a disabled
		// entry's reason names other entries — "needs bun — install it from
		// Doctor" — so searching for "Doctor" alone found it inside that
		// description, on an earlier row, at a completely different column.
		needle := it.key + "  " + it.title
		var found bool
		for _, line := range strings.Split(rendered, "\n") {
			i := strings.Index(line, needle)
			if i < 0 {
				continue
			}
			// Rune counts, not byte offsets: the selection cursor "› " is
			// wider in bytes than in columns.
			cols = append(cols,
				utf8.RuneCountInString(line[:i])+utf8.RuneCountInString(it.key)+2)
			found = true
			break
		}
		if !found {
			t.Fatalf("menu row %q never rendered", needle)
		}
	}
	for i, got := range cols {
		if got != cols[0] {
			t.Errorf("title %q starts at column %d, want %d", m.menu()[i].title, got, cols[0])
		}
	}
}

// stripANSI removes SGR sequences so column positions can be measured.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// TestNoScreenEmitsPaddedBlankLines is the general form of the indent bug.
//
// lipgloss pads every line of a Render call out to the width of the widest, so
// passing a string containing "\n" turns its blank lines into runs of spaces
// with no newline after them — and the next thing written starts at that
// column. The visible symptom is one row indented far to the right. A line
// that is only whitespace is always the fingerprint, so assert none exist.
func TestNoScreenEmitsPaddedBlankLines(t *testing.T) {
	for _, tc := range screenCases() {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(t)
			if tc.setup != nil {
				tc.setup(m)
			}
			m.screen = tc.screen
			// The screen body, before the page padding, which legitimately pads
			// every line to a common width.
			for i, line := range strings.Split(stripANSI(m.screenBody()), "\n") {
				if line != "" && strings.TrimSpace(line) == "" {
					t.Errorf("line %d is %d spaces, not an empty line — "+
						"something passed a newline into Style.Render", i+1, len(line))
				}
			}
		})
	}
}

// TestLayoutIsIndependentOfColour is the test the earlier alignment bugs
// slipped past.
//
// Under `go test` lipgloss finds no terminal and emits no escape sequences, so
// a column padded with fmt's %-Ns looks perfectly aligned — and misaligns the
// moment a real terminal turns colour on, because the escapes count toward the
// verb's width. Rendering each screen twice, once with colour forced on, and
// comparing the plain text pins that styling never changes layout.
func TestLayoutIsIndependentOfColour(t *testing.T) {
	for _, tc := range screenCases() {
		t.Run(tc.name, func(t *testing.T) {
			// One model rendered twice: a second model would sit in a
			// different t.TempDir and the paths on screen would differ for
			// reasons that have nothing to do with layout.
			m := newTestModel(t)
			if tc.setup != nil {
				tc.setup(m)
			}
			m.screen = tc.screen
			m.w, m.h = 100, 40

			render := func(p termenv.Profile) string {
				prev := lipgloss.ColorProfile()
				lipgloss.SetColorProfile(p)
				defer lipgloss.SetColorProfile(prev)
				return stripANSI(m.View())
			}
			plain := render(termenv.Ascii)
			coloured := render(termenv.TrueColor)
			if plain == coloured {
				return
			}
			pl, cl := strings.Split(plain, "\n"), strings.Split(coloured, "\n")
			for i := range max(len(pl), len(cl)) {
				var a, c string
				if i < len(pl) {
					a = pl[i]
				}
				if i < len(cl) {
					c = cl[i]
				}
				if a != c {
					t.Errorf("line %d differs once colour is on — a column was padded with the escapes counted:\n  without colour: %q\n  with colour:    %q", i+1, a, c)
				}
			}
		})
	}
}

// TestLaunchesOnTheMenu: the Doctor is a diagnostic, not a splash screen.
func TestLaunchesOnTheMenu(t *testing.T) {
	m := newTestModel(t)
	if m.screen != screenMenu {
		t.Fatalf("started on screen %v, want the menu", m.screen)
	}
}

func TestDoctorOpensOnlyWhenSomethingIsBroken(t *testing.T) {
	warn := []Check{
		{Name: "Ollama", Status: StatusWarn, Detail: "no server"},
		{Name: "Corpus", Status: StatusOK, Detail: "project dev"},
	}
	broken := []Check{
		{Name: "Ollama", Status: StatusWarn, Detail: "no server"},
		{Name: "Encoder match", Status: StatusMissing, Detail: "built with another model"},
	}

	t.Run("warnings alone do not interrupt", func(t *testing.T) {
		m := newTestModel(t)
		next, _ := m.Update(checksMsg(warn))
		if got := next.(*rootModel).screen; got != screenMenu {
			t.Errorf("screen = %v, want the menu — a missing optional capability is the normal state", got)
		}
	})

	t.Run("a real failure opens the doctor", func(t *testing.T) {
		m := newTestModel(t)
		next, _ := m.Update(checksMsg(broken))
		rm := next.(*rootModel)
		if rm.screen != screenDoctor {
			t.Fatalf("screen = %v, want the doctor", rm.screen)
		}
		if rm.checks[rm.checkIdx].Status != StatusMissing {
			t.Error("cursor should land on the failing row")
		}
		if rm.status == "" {
			t.Error("no explanation given for the redirect")
		}
	})

	t.Run("only on the first check", func(t *testing.T) {
		m := newTestModel(t)
		next, _ := m.Update(checksMsg(warn)) // consumes the one-time redirect
		rm := next.(*rootModel)
		rm.screen = screenConsole
		next, _ = rm.Update(checksMsg(broken))
		if got := next.(*rootModel).screen; got != screenConsole {
			t.Errorf("screen = %v, want the console — a re-check after a job must not move the user", got)
		}
	})
}

func TestDoctorSummaryReportsState(t *testing.T) {
	m := newTestModel(t)
	cases := []struct {
		name   string
		checks []Check
		want   string
	}{
		{"all clear", []Check{{Status: StatusOK}}, "everything this machine can do is available"},
		{"one optional", []Check{{Status: StatusWarn}}, "1 optional capability not installed"},
		{"two optional", []Check{{Status: StatusWarn}, {Status: StatusWarn}}, "2 optional capabilities not installed"},
		{"one broken", []Check{{Status: StatusMissing}}, "1 check needs attention"},
		{"mixed", []Check{{Status: StatusMissing}, {Status: StatusWarn}}, "1 check needs attention, 1 optional not installed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m.checks = tc.checks
			if got := m.doctorSummary(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestMenuHidesSourceBuildOutsideACheckout: a packaged install has no source
// tree, so the entry would sit permanently disabled — noise on every launch for
// something that cannot run there.
func TestMenuHidesSourceBuildOutsideACheckout(t *testing.T) {
	m := newTestModel(t)

	m.inCheckout = true
	if !hasKey(m.menu(), "r") {
		t.Error("Build from source should appear inside a checkout")
	}

	m.inCheckout = false
	if hasKey(m.menu(), "r") {
		t.Error("Build from source should be absent outside a checkout")
	}
}

// TestMenuStaysShort guards the collapse: screens that act on a corpus live
// under Projects and model management lives under Doctor, so they must not
// creep back onto the front page.
func TestMenuStaysShort(t *testing.T) {
	m := newTestModel(t)
	m.inCheckout = false

	items := m.menu()
	if len(items) > 8 {
		var titles []string
		for _, it := range items {
			titles = append(titles, it.title)
		}
		t.Errorf("menu has %d entries, want at most 8: %v", len(items), titles)
	}
	for _, gone := range []string{"m", "a", "x"} {
		if hasKey(items, gone) {
			t.Errorf("key %q is back on the front page; it belongs under Projects or Doctor", gone)
		}
	}
	for _, want := range []string{"s", "c", "h", "b", "p", "d", "t", "q"} {
		if !hasKey(items, want) {
			t.Errorf("key %q missing from the menu", want)
		}
	}
}

// TestCancelledJobIsNotReportedAsFailed: cancelling kills the child process,
// which surfaces as "signal: killed". Reporting that as a failure reads like a
// crash the user caused by asking the job to stop.
func TestCancelledJobIsNotReportedAsFailed(t *testing.T) {
	m := newTestModel(t)
	m.job = "source build"
	m.jobCancelled = true

	next, _ := m.Update(jobDoneMsg{name: "source build", err: errors.New("signal: killed")})
	rm := next.(*rootModel)

	if strings.Contains(rm.status, "failed") {
		t.Errorf("status = %q, want it to read as a cancellation", rm.status)
	}
	if !strings.Contains(rm.status, "cancelled") {
		t.Errorf("status = %q, want it to say cancelled", rm.status)
	}
	if rm.err != nil {
		t.Errorf("err = %v, want nil — the user asked for this", rm.err)
	}
	if rm.jobCancelled {
		t.Error("the cancellation flag must not leak into the next job")
	}
}

// TestGenuineFailureStillReportsAsFailed is the other half: without a
// cancellation, an error is still an error.
func TestGenuineFailureStillReportsAsFailed(t *testing.T) {
	m := newTestModel(t)
	m.job = "build"

	next, _ := m.Update(jobDoneMsg{name: "build", err: errors.New("no such file")})
	rm := next.(*rootModel)

	if !strings.Contains(rm.status, "failed") {
		t.Errorf("status = %q, want a failure", rm.status)
	}
	if rm.err == nil {
		t.Error("err should be set for a real failure")
	}
}

func hasKey(items []menuItem, key string) bool {
	for _, it := range items {
		if it.key == key {
			return true
		}
	}
	return false
}

// TestCollapsedNavigation pins where the moved screens now live, and that esc
// returns to the screen that opened them rather than jumping to the menu.
func TestCollapsedNavigation(t *testing.T) {
	key := func(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
	esc := tea.KeyMsg{Type: tea.KeyEsc}

	t.Run("doctor opens models", func(t *testing.T) {
		m := newTestModel(t)
		m.screen = screenDoctor
		next, _ := m.updateDoctor(key("m"))
		rm := next.(*rootModel)
		if rm.screen != screenModels {
			t.Fatalf("screen = %v, want models", rm.screen)
		}
		next, _ = rm.updateModels(esc)
		if got := next.(*rootModel).screen; got != screenDoctor {
			t.Errorf("esc from models went to %v, want back to doctor", got)
		}
	})

	t.Run("projects opens clean, scoped to the highlighted row", func(t *testing.T) {
		m := newTestModel(t)
		// Two projects so "the selected one" is distinguishable from "the
		// active one" — the whole point of scoping to the cursor.
		for _, id := range []string{"alpha", "beta"} {
			proj, err := m.store.Project(id)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(proj.Path("papers.jsonl"), []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			meta := store.Meta{ID: id, Name: id, Status: "complete", DocsIngested: 1, CreatedAt: time.Now().UTC()}
			if err := proj.SaveJSON("meta.json", &meta); err != nil {
				t.Fatal(err)
			}
		}
		m.projects = newProjectList(m.store, m.settings.Project)
		if len(m.projects.rows) < 2 {
			t.Fatalf("expected two projects, got %d", len(m.projects.rows))
		}
		m.projects.idx = 1
		want := m.projects.rows[1].id

		m.screen = screenProjects
		next, _ := m.updateProjects(key("x"))
		rm := next.(*rootModel)
		if rm.screen != screenClean {
			t.Fatalf("screen = %v, want clean", rm.screen)
		}
		if !strings.Contains(rm.clean.actions[2].title, want) {
			t.Errorf("clean screen targets %q, want the highlighted project %q",
				rm.clean.actions[2].title, want)
		}
		if m.settings.Project == want {
			t.Error("cleaning must not silently change the active project")
		}

		next, _ = rm.updateClean(esc)
		if got := next.(*rootModel).screen; got != screenProjects {
			t.Errorf("esc from clean went to %v, want back to projects", got)
		}
	})
}
