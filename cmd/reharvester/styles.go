package main

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

// Palette mirrors the project's design tokens: a cool neutral ramp, near-black
// for emphasis, and the accent reserved for links and eyebrow labels rather
// than fills. Adaptive pairs keep it legible on light and dark terminals.
var (
	colText   = lipgloss.AdaptiveColor{Light: "#1D1F20", Dark: "#E4E6E7"}
	colMuted  = lipgloss.AdaptiveColor{Light: "#676C6F", Dark: "#959A9D"}
	colFaint  = lipgloss.AdaptiveColor{Light: "#959A9D", Dark: "#4E5355"}
	colAccent = lipgloss.AdaptiveColor{Light: "#006399", Dark: "#4EA8DE"}
	colOK     = lipgloss.AdaptiveColor{Light: "#1B7F4B", Dark: "#5BD68F"}
	colWarn   = lipgloss.AdaptiveColor{Light: "#8A5A00", Dark: "#E0A94A"}
	colDanger = lipgloss.AdaptiveColor{Light: "#A11B1B", Dark: "#F08080"}
	colRule   = lipgloss.AdaptiveColor{Light: "#D9DBDD", Dark: "#2A2D2F"}
)

var (
	styleTitle   = lipgloss.NewStyle().Foreground(colText).Bold(true)
	styleEyebrow = lipgloss.NewStyle().Foreground(colAccent)
	styleBody    = lipgloss.NewStyle().Foreground(colText)
	styleMuted   = lipgloss.NewStyle().Foreground(colMuted)
	styleFaint   = lipgloss.NewStyle().Foreground(colFaint)
	styleOK      = lipgloss.NewStyle().Foreground(colOK)
	styleWarn    = lipgloss.NewStyle().Foreground(colWarn)
	styleDanger  = lipgloss.NewStyle().Foreground(colDanger).Bold(true)
	styleSel     = lipgloss.NewStyle().Foreground(colAccent).Bold(true)

	styleHelp   = lipgloss.NewStyle().Foreground(colFaint)
	styleBanner = lipgloss.NewStyle().Foreground(colAccent)
	stylePane   = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colRule).
			Padding(0, 1)
	styleCmd = lipgloss.NewStyle().
			Foreground(colText).
			Background(lipgloss.AdaptiveColor{Light: "#F2F3F4", Dark: "#1B1D1E"}).
			Padding(0, 1)
)

// Page padding. Every screen renders inside this, so content never touches the
// terminal edge; the width calculations elsewhere subtract it.
const (
	pagePadX = 2
	pagePadY = 1
)

var stylePage = lipgloss.NewStyle().Padding(pagePadY, pagePadX)

// padRight pads plain text out to a column width, counting runes.
//
// Never use fmt's %-Ns on styled text: the escape sequences lipgloss emits
// count toward the width, so a styled string is already "wider" than the verb
// asks for and no padding is added at all. It looks correct under `go test`,
// where lipgloss detects no terminal and emits no escapes, and misaligns the
// moment a real terminal is attached. Pad first, style second.
func padRight(s string, w int) string {
	if n := utf8.RuneCountInString(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// columnWidth sizes a label column to its longest entry, with a gutter.
func columnWidth(labels []string, gutter int) int {
	w := 0
	for _, l := range labels {
		if n := utf8.RuneCountInString(l); n > w {
			w = n
		}
	}
	return w + gutter
}

// block renders multi-line text one line at a time.
//
// Never pass a string containing "\n" to Style.Render directly: lipgloss pads
// every line out to the width of the widest one, so a blank or trailing line
// becomes a run of spaces, and whatever is written next starts at that column
// instead of the left margin. Empty lines are left untouched here so they stay
// genuinely empty.
func block(st lipgloss.Style, s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = st.Render(l)
		}
	}
	return strings.Join(lines, "\n")
}

// bannerLines is the REHARVESTER wordmark, 54 columns wide. Kept as separate
// lines so each is styled on its own — a single Render over the block would
// pad every row to the widest, which is the indent bug in styles.go's block().
var bannerLines = []string{
	"███  ████ █  █  ██  ███  █  █ ████  ███ ████ ████ ███ ",
	"█  █ █    █  █ █  █ █  █ █  █ █    █     █   █    █  █",
	"███  ███  ████ ████ ███  █  █ ███   ██   █   ███  ███ ",
	"█ █  █    █  █ █  █ █ █  █  █ █       █  █   █    █ █ ",
	"█  █ ████ █  █ █  █ █  █  ██  ████ ███   █   ████ █  █",
}

const bannerWidth = 54

// banner returns the wordmark, or "" when the terminal is too narrow for it.
// Callers fall back to the plain text header rather than printing something
// that wraps into unreadable fragments.
func banner(width int) string {
	if width < bannerWidth {
		return ""
	}
	out := make([]string, len(bannerLines))
	for i, l := range bannerLines {
		out[i] = styleBanner.Render(l)
	}
	return strings.Join(out, "\n")
}

// helpLine renders the key hints with a blank line above it.
//
// The blank line is a literal newline rather than lipgloss MarginTop, which
// renders the margin row padded out to the block width — a line of spaces
// rather than an empty one.
func helpLine(s string) string { return "\n" + styleHelp.Render(s) }

// header renders the screen title with a mono-caps eyebrow above it, the same
// pairing the web UI uses to label a section, and ends with a blank line.
//
// Built by concatenation rather than lipgloss.JoinVertical: that pads every
// line out to the width of the widest one, so a trailing empty line becomes a
// run of spaces with no newline after it, and whatever is written next lands
// indented to the title's width instead of at the left margin.
func header(eyebrow, title string) string {
	return styleEyebrow.Render(eyebrow) + "\n" +
		styleTitle.Render(title) + "\n\n"
}

// statusMark returns a single glyph for a check outcome. Never the only signal:
// every caller pairs it with text, since colour alone does not carry meaning.
func statusMark(ok bool, warn bool) string {
	switch {
	case ok:
		return styleOK.Render("ok  ")
	case warn:
		return styleWarn.Render("warn")
	default:
		return styleDanger.Render("miss")
	}
}
