package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/yothgewalt/reharvester/internal/app"
	"github.com/yothgewalt/reharvester/internal/harvest"
)

// tuiReporter feeds pipeline progress into the model. Sends are non-blocking:
// a build must never stall because the UI is mid-frame.
type tuiReporter struct {
	sink *logSink
	ch   chan progressMsg
}

func (r tuiReporter) Log(level, msg string) { r.sink.Push("  [" + level + "] " + msg) }

func (r tuiReporter) Progress(pct int, stage string) {
	select {
	case r.ch <- progressMsg{pct: pct, stage: stage}:
	default:
	}
}

// startJob runs one operation in the background. Only one may run at a time:
// harvest, build and analyse all touch the same project directory.
func (m *rootModel) startJob(name string, run func(context.Context) error) tea.Cmd {
	if m.job != "" {
		m.status = m.job + " is already running"
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.job, m.jobCancel, m.jobCancelled = name, cancel, false
	m.jobStart, m.jobPct, m.jobStage = time.Now(), 0, "starting"
	m.screen = screenProgress
	m.sink.Push("── " + name + " started ──")
	return func() tea.Msg {
		err := run(ctx)
		cancel()
		return jobDoneMsg{name: name, err: err}
	}
}

func (m *rootModel) runBuild(ctx context.Context) error {
	emb := app.Embedder(ctx, m.settings.OllamaURL, m.settings.EmbedModel)
	return app.Build(ctx, m.store, m.settings.Project, emb, app.BuildOptions{
		Approx:     m.settings.Approx,
		EmbedModel: m.settings.EmbedModel,
		Reporter:   tuiReporter{sink: m.sink, ch: m.progressCh},
	})
}

func (m *rootModel) runAnalyze(context.Context) error {
	return app.Analyze(m.store, m.settings.Project)
}

func (m *rootModel) runHarvest(ctx context.Context) error {
	s := m.settings
	return app.Harvest(ctx, m.store, s.Project, harvest.Query{
		Categories: app.SplitList(s.Categories),
		Keywords:   app.SplitList(s.Keywords),
		From:       s.From, To: s.To, Max: s.Max,
	}, s.Delay)
}

// runSourceBuild rebuilds the UI and the binary from a source checkout. It is
// a no-op anywhere else, and says so rather than failing obscurely.
func (m *rootModel) runSourceBuild(ctx context.Context) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	steps := []struct {
		dir  string
		name string
		args []string
	}{
		{filepath.Join(root, "web"), "bun", []string{"install"}},
		{filepath.Join(root, "web"), "bun", []string{"run", "build"}},
	}
	for _, s := range steps {
		if err := m.streamCmd(ctx, s.dir, s.name, s.args...); err != nil {
			return err
		}
	}
	// Stage the export where the embed directive can see it, then build.
	dst := filepath.Join(root, "internal", "webui", "dist")
	if err := replaceDir(filepath.Join(root, "web", "dist"), dst); err != nil {
		return fmt.Errorf("staging web/dist: %w", err)
	}
	m.sink.Push("staged web/dist -> internal/webui/dist")
	return m.streamCmd(ctx, root, "go", "build", "-o", filepath.Join(root, "bin", "reharvester"), "./cmd/reharvester")
}

// streamCmd runs a command with its output going to the console, so a long
// build reports progress rather than appearing hung.
func (m *rootModel) streamCmd(ctx context.Context, dir, name string, args ...string) error {
	m.sink.Push("$ " + name + " " + strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		m.sink.Push(sc.Text())
	}
	return cmd.Wait()
}

// repoRoot finds the checkout by walking up for go.mod. A packaged install has
// no source tree, which is a clear message rather than a confusing failure.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no source checkout here — building from source needs the repository, not a packaged install")
		}
		dir = parent
	}
}

// replaceDir swaps dst for a copy of src. The old directory is moved aside and
// only removed once the copy succeeds, so a failed build does not leave the
// embed directory empty.
func replaceDir(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	backup := dst + ".prev"
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(dst); err == nil {
		if err := os.Rename(dst, backup); err != nil {
			return err
		}
	}
	if err := copyTree(src, dst); err != nil {
		_ = os.RemoveAll(dst)
		_ = os.Rename(backup, dst)
		return err
	}
	_ = os.RemoveAll(backup)
	return nil
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, b, info.Mode().Perm())
	})
}

// toggleServer starts or stops the API. It runs in-process rather than as a
// child so the console sees its log directly.
func (m *rootModel) toggleServer() tea.Cmd {
	if m.serverOn {
		if m.serverCancel != nil {
			m.serverCancel()
		}
		return nil
	}
	if free, detail := portFree(m.settings.Addr); !free {
		m.status = "cannot start: " + m.settings.Addr + " " + detail
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.serverOn, m.serverCancel = true, cancel
	m.screen = screenConsole
	s := m.settings
	st := m.store
	return func() tea.Msg {
		err := app.Serve(ctx, st, app.ServeConfig{
			Addr:       s.Addr,
			Project:    s.Project,
			MaxRecords: s.Max,
			Delay:      s.Delay,
			Snapshot:   s.Snapshot,
			OllamaURL:  s.OllamaURL,
			EmbedModel: s.EmbedModel,
			ChatModel:  s.ChatModel,
		})
		cancel()
		return serverStoppedMsg{err: err}
	}
}

func (m *rootModel) viewProgress() string {
	var b strings.Builder
	title := m.job
	if title == "" {
		title = "Finished"
	}
	b.WriteString(header("RUNNING", title))

	if m.job != "" {
		bar := progress.New(progress.WithoutPercentage(), progress.WithWidth(clamp(m.contentWidth()-18, 20, 60)))
		pct := float64(m.jobPct) / 100
		b.WriteString(fmt.Sprintf("%s %s %3d%%  %s\n\n",
			m.spin.View(), bar.ViewAs(pct), m.jobPct, styleMuted.Render(m.jobStage)))
		b.WriteString(styleFaint.Render(fmt.Sprintf("elapsed %s", shortDur(time.Since(m.jobStart)))))
		b.WriteString("\n\n")
	}
	b.WriteString(m.console.tail(clamp(m.h-16, 5, 20)))
	if m.job != "" {
		b.WriteString(helpLine("ctrl+c cancel — a partial harvest is kept, not discarded"))
	} else {
		b.WriteString(helpLine("esc back to the menu"))
	}
	return b.String()
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
