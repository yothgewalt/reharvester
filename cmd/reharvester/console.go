package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// consoleModel shows what the process is doing. Everything arrives through one
// sink — the log package is the single output path for the API, the pipeline
// and the TUI's own child commands — so the tabs are views over that stream
// rather than separate capture channels.
type consoleModel struct {
	sink   *logSink
	lines  []string
	tab    int
	w, h   int
	offset int // lines scrolled up from the bottom; 0 means following
}

type consoleTab struct {
	name  string
	match func(string) bool
}

var consoleTabs = []consoleTab{
	{"All", func(string) bool { return true }},
	{"Server", func(l string) bool { return !hasAnyPrefix(l, "http:") && !isJobLine(l) }},
	{"Access", func(l string) bool { return hasAnyPrefix(l, "http:") }},
	{"Jobs", isJobLine},
	{"Frontend", func(string) bool { return false }},
}

func isJobLine(l string) bool {
	return hasAnyPrefix(l, "harvest:", "build:", "analyze:", "graph:", "encoder:", "$ ", "──", "  [")
}

// hasAnyPrefix ignores the leading timestamp the sink adds.
func hasAnyPrefix(line string, prefixes ...string) bool {
	if i := strings.IndexByte(line, ' '); i > 0 && len(line) > i+1 {
		if len(line) >= 8 && line[2] == ':' && line[5] == ':' {
			line = line[i+1:]
		}
	}
	for _, p := range prefixes {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}

func newConsole(s *logSink) consoleModel {
	return consoleModel{sink: s, lines: s.Lines(), h: 20, w: 80}
}

func (c *consoleModel) append(line string) {
	c.lines = append(c.lines, line)
	if len(c.lines) > logRing {
		c.lines = c.lines[len(c.lines)-logRing:]
	}
}

func (c *consoleModel) resize(w, h int) { c.w, c.h = w, clamp(h, 5, 200) }

func (c *consoleModel) visible() []string {
	m := consoleTabs[c.tab].match
	out := make([]string, 0, len(c.lines))
	for _, l := range c.lines {
		if m(l) {
			out = append(out, l)
		}
	}
	return out
}

// tail renders the last n lines regardless of tab, for the progress screen.
func (c *consoleModel) tail(n int) string {
	lines := c.lines
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(styleFaint.Render(truncate(l, c.w-2)) + "\n")
	}
	return b.String()
}

func (m *rootModel) updateConsole(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := &m.console
	switch k.String() {
	case "esc", "q":
		m.screen = screenMenu
	case "tab", "right", "l":
		c.tab = (c.tab + 1) % len(consoleTabs)
		c.offset = 0
	case "shift+tab", "left", "h":
		c.tab = (c.tab - 1 + len(consoleTabs)) % len(consoleTabs)
		c.offset = 0
	case "up", "k":
		c.offset++
	case "down", "j":
		if c.offset > 0 {
			c.offset--
		}
	case "g":
		c.offset = len(c.visible())
	case "G", "end":
		c.offset = 0
	case "o":
		if m.serverOn {
			return m, openBrowser(browseURL(m.settings.Addr))
		}
		m.status = "server is not running"
	case "s":
		return m, m.toggleServer()
	}
	return m, nil
}

func (m *rootModel) viewConsole() string {
	c := &m.console
	var b strings.Builder

	state := styleFaint.Render("stopped")
	if m.serverOn {
		state = styleOK.Render("running") + styleFaint.Render(" · "+browseURL(m.settings.Addr))
	}
	b.WriteString(header("CONSOLE", "Server "+state))

	var tabs []string
	for i, t := range consoleTabs {
		if i == c.tab {
			tabs = append(tabs, styleSel.Render("["+t.name+"]"))
		} else {
			tabs = append(tabs, styleFaint.Render(" "+t.name+" "))
		}
	}
	b.WriteString(strings.Join(tabs, " ") + "\n\n")

	if consoleTabs[c.tab].name == "Frontend" {
		b.WriteString(m.frontendPane())
	} else {
		lines := c.visible()
		height := clamp(m.h-12, 5, 200)
		end := len(lines) - c.offset
		if end < 0 {
			end = 0
		}
		start := end - height
		if start < 0 {
			start = 0
		}
		if len(lines) == 0 {
			b.WriteString(styleFaint.Render("nothing yet on this tab") + "\n")
		}
		for _, l := range lines[start:end] {
			b.WriteString(truncate(l, clamp(m.contentWidth(), 20, 500)) + "\n")
		}
		if c.offset > 0 {
			b.WriteString(styleWarn.Render(fmt.Sprintf("\n— scrolled up %d lines, G to follow again —", c.offset)))
		}
	}

	if p := m.sink.Path(); p != "" {
		b.WriteString("\n" + styleFaint.Render("log file: "+p))
	}
	b.WriteString(helpLine("tab switch · ↑↓ scroll · G follow · s start/stop server · o open browser · esc back"))
	return b.String()
}

// frontendPane explains rather than showing an empty tab. In a packaged
// install there is genuinely no frontend process — the UI is static files
// inside this binary — and pretending otherwise would read as a bug.
func (m *rootModel) frontendPane() string {
	if _, err := repoRoot(); err != nil {
		return styleMuted.Render("No frontend process in a packaged install.") + "\n\n" +
			block(styleFaint,
				"The web UI is compiled into this binary and served by the API itself,\n"+
					"so there is nothing separate to log. Browser-side errors appear in\n"+
					"your browser's own console.") + "\n"
	}
	if uiEmbedded() {
		return styleMuted.Render("This build already embeds the UI.") + "\n\n" +
			block(styleFaint,
				"The API serves it directly, so no dev server is needed. Use\n"+
					"`bun run dev` in web/ if you want hot reload on :3000.") + "\n"
	}
	return styleMuted.Render("No UI embedded in this build.") + "\n\n" +
		block(styleFaint,
			"Run \"Build from source\" to compile the UI into the binary, or start\n"+
				"the Next dev server yourself with `bun run dev` in web/.") + "\n"
}

func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
