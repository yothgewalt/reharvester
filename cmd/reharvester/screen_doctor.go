package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *rootModel) updateDoctor(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "up", "k":
		if m.checkIdx > 0 {
			m.checkIdx--
		}
	case "down", "j":
		if m.checkIdx < len(m.checks)-1 {
			m.checkIdx++
		}
	case "r":
		m.checking = true
		return m, m.runChecksCmd()
	case "i", "enter":
		if m.checkIdx < len(m.checks) {
			return m.beginInstall(m.checks[m.checkIdx])
		}
	case "m":
		m.models = newModelList(m.settings)
		m.screen = screenModels
	case "esc", "q", "s":
		m.screen = screenMenu
	}
	return m, nil
}

func (m *rootModel) viewDoctor() string {
	var b strings.Builder
	b.WriteString(header("DOCTOR", "What this machine can do"))

	if m.checking && len(m.checks) == 0 {
		return b.String() + m.spin.View() + styleMuted.Render(" checking…")
	}

	b.WriteString(styleFaint.Render(
		"Nothing here is required to run Reharvester. Each row says what it unlocks.") + "\n\n")

	names := make([]string, len(m.checks))
	for i, c := range m.checks {
		names[i] = c.Name
	}
	w := columnWidth(names, 2)

	for i, c := range m.checks {
		cursor, style := "  ", styleBody
		if i == m.checkIdx {
			cursor, style = styleSel.Render("› "), styleSel
		}
		b.WriteString(cursor + statusMark(c.Status == StatusOK, c.Status == StatusWarn) + "  " +
			style.Render(padRight(c.Name, w)) + styleMuted.Render(c.Detail) + "\n")
		if c.Status != StatusOK {
			b.WriteString(fmt.Sprintf("%8s%s\n", "", styleFaint.Render("unlocks "+c.Unlocks)))
			if c.Fix != "" {
				b.WriteString(fmt.Sprintf("%8s%s\n", "", styleWarn.Render(c.Fix)))
			}
		}
	}

	if !uiEmbedded() {
		b.WriteString("\n" + block(styleFaint,
			"This build has no web UI embedded — the API runs without one.\n"+
				"\"Build from source\" compiles it in."))
	}
	b.WriteString(helpLine("↑↓ move · i install the selected item · m models · r re-check · esc menu"))
	return b.String()
}

// ---- install ----

type installModel struct {
	dep      *Dep
	plan     Plan
	resolved bool
	step     int
	running  bool
	done     bool
	err      error
}

type installStepDoneMsg struct{ err error }

func (m *rootModel) beginInstall(c Check) (tea.Model, tea.Cmd) {
	if c.Dep == nil {
		m.status = ternary(c.Status == StatusOK, "nothing to install", "this one has no installer — "+c.Fix)
		return m, nil
	}
	plan, ok := c.Dep.Resolve()
	m.install = installModel{dep: c.Dep, plan: plan, resolved: ok}
	m.screen = screenInstall
	return m, nil
}

func (m *rootModel) updateInstall(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	in := &m.install
	switch k.String() {
	case "esc", "q", "n":
		if in.running {
			m.status = "an install is running; let it finish"
			return m, nil
		}
		m.screen = screenDoctor
		if in.done {
			m.checking = true
			return m, m.runChecksCmd()
		}
	case "y", "enter":
		if in.running || in.done || !in.resolved {
			return m, nil
		}
		in.running = true
		return m, m.execInstallStep()
	}
	return m, nil
}

// execInstallStep hands the terminal to one command. tea.ExecProcess is what
// makes a sudo password prompt and a package manager's progress bar work: the
// TUI releases the screen for the duration rather than trying to proxy a TTY.
func (m *rootModel) execInstallStep() tea.Cmd {
	in := &m.install
	if in.step >= len(in.plan.Steps) {
		in.running, in.done = false, true
		return func() tea.Msg { return statusMsg(in.dep.Name + " finished — re-checking") }
	}
	st := in.plan.Steps[in.step]
	m.sink.Push("$ " + st.Prompt())
	return tea.ExecProcess(st.Command(), func(err error) tea.Msg {
		return installStepDoneMsg{err: err}
	})
}

func (m *rootModel) onInstallStepDone(msg installStepDoneMsg) (tea.Model, tea.Cmd) {
	in := &m.install
	if msg.err != nil {
		in.running, in.done, in.err = false, true, msg.err
		m.sink.Push("install failed: " + msg.err.Error())
		return m, nil
	}
	in.step++
	if in.step >= len(in.plan.Steps) {
		in.running, in.done = false, true
		m.sink.Push(in.dep.Name + ": install steps completed")
		m.checking = true
		return m, m.runChecksCmd()
	}
	return m, m.execInstallStep()
}

func (m *rootModel) viewInstall() string {
	in := &m.install
	var b strings.Builder
	b.WriteString(header("INSTALL", in.dep.Name))

	if !in.resolved {
		b.WriteString(styleWarn.Render("No installer applies to this machine.") + "\n\n")
		b.WriteString(styleBody.Render("Install it yourself from:") + "\n")
		b.WriteString(styleEyebrow.Render("  "+in.dep.Docs) + "\n")
		b.WriteString("\n" + block(styleFaint,
			"No package manager was found that this project knows how to drive on "+
				runtime.GOOS+",\nand guessing at one would be worse than saying so.") + "\n")
		b.WriteString(helpLine("esc back"))
		return b.String()
	}

	b.WriteString(styleMuted.Render("Method: " + in.plan.Label))
	if in.dep.Size != "" {
		b.WriteString(styleMuted.Render("  ·  download " + in.dep.Size))
	}
	b.WriteString("\n\n" + styleBody.Render("These commands will run, exactly as written:") + "\n\n")

	for i, st := range in.plan.Steps {
		mark := "  "
		switch {
		case in.done && in.err == nil, i < in.step:
			mark = styleOK.Render("✓ ")
		case i == in.step && in.running:
			mark = m.spin.View() + " "
		}
		b.WriteString(mark + styleCmd.Render(st.Prompt()) + "\n")
		if st.Note != "" {
			b.WriteString(styleFaint.Render("    "+st.Note) + "\n")
		}
		if st.Risky() {
			b.WriteString(styleWarn.Render("    runs with elevated privileges or executes a remote script") + "\n")
		}
	}

	for _, c := range in.plan.Caveats {
		b.WriteString("\n" + styleWarn.Render("! "+c) + "\n")
	}

	switch {
	case in.err != nil:
		b.WriteString("\n" + styleDanger.Render("Failed: "+in.err.Error()))
		b.WriteString("\n" + styleFaint.Render("Nothing was rolled back — check the output above before retrying."))
		b.WriteString(helpLine("esc back to Doctor"))
	case in.done:
		b.WriteString("\n" + styleOK.Render("Done. Re-checking what changed."))
		b.WriteString(helpLine("esc back to Doctor"))
	case in.running:
		b.WriteString("\n" + styleMuted.Render("Running — the terminal is handed over while it works."))
	default:
		b.WriteString("\n" + styleBody.Render("Run them?"))
		b.WriteString(helpLine("y run · n or esc cancel — nothing has run yet"))
	}
	return b.String()
}

// openBrowser opens a URL with the platform's own handler. Failure is reported
// rather than swallowed: the user can always copy the URL from the console.
func openBrowser(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		if err := cmd.Start(); err != nil {
			return statusMsg("could not open a browser — visit " + url)
		}
		return statusMsg("opened " + url)
	}
}
