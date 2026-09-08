// Command reharvester is the interactive front end to the Reharvester
// pipeline. Run with no arguments it opens a TUI; run with a subcommand it
// behaves like a conventional CLI, because a TUI-only binary cannot be
// scripted.
//
// Every operation is the same call the flag-driven harvester-server makes —
// both go through internal/app — so the two cannot drift.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yothgewalt/reharvester/internal/app"
	"github.com/yothgewalt/reharvester/internal/harvest"
	"github.com/yothgewalt/reharvester/internal/store"
	"github.com/yothgewalt/reharvester/internal/webui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-h", "--help", "help":
			usage()
			return
		case "-v", "--version", "version":
			fmt.Printf("reharvester %s\n", app.Version)
			return
		}
		if isCommand(os.Args[1]) {
			os.Exit(runCLI(os.Args[1], os.Args[2:]))
		}
		// A leading flag is not a subcommand: `reharvester --data ./corpus`
		// opens the TUI pointed somewhere else. Only a bare word can name a
		// command, so anything else is a genuine typo.
		if !strings.HasPrefix(os.Args[1], "-") {
			fmt.Fprintf(os.Stderr, "reharvester: unknown command %q\n\n", os.Args[1])
			usage()
			os.Exit(2)
		}
	}
	if err := runTUI(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "reharvester: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`reharvester — local-first research-literature discovery

  reharvester                 open the interactive menu
  reharvester serve           serve the API and UI on :8000
  reharvester harvest         fetch a corpus from arXiv
  reharvester build           build indexes, backbone and analytics
  reharvester analyze         report trends and gap candidates
  reharvester graphcheck      compare exact and pruned backbones
  reharvester doctor          check dependencies; exit 1 if any are missing
  reharvester --version

Every subcommand takes the same flags as harvester-server; run one with
-h to list them. With no arguments the menu covers all of it, plus the
console, project management and a reset.
`)
}

// commands are the headless subcommands. Anything else that is not a flag is a
// typo rather than a path or a file.
var commands = []string{"harvest", "build", "serve", "analyze", "graphcheck", "doctor"}

func isCommand(s string) bool {
	return slices.Contains(commands, s)
}

// runCLI is the headless path. It mirrors harvester-server's flag set so a
// script can move between the two binaries without translation.
func runCLI(cmd string, args []string) int {
	fs := flag.NewFlagSet("reharvester "+cmd, flag.ExitOnError)
	d := DefaultSettings()
	var (
		data       = fs.String("data", d.DataDir, "directory holding projects and artefacts")
		addr       = fs.String("addr", d.Addr, "listen address for the local API")
		project    = fs.String("project", d.Project, "project id to operate on")
		categories = fs.String("categories", d.Categories, "comma-separated arXiv categories")
		keywords   = fs.String("keywords", "", "comma-separated keywords to AND with the categories")
		from       = fs.Int("from", d.From, "earliest submission year")
		to         = fs.Int("to", d.To, "latest submission year")
		maxRecords = fs.Int("max", d.Max, "maximum records to retain")
		delay      = fs.Duration("delay", d.Delay, "politeness delay between API requests")
		approx     = fs.Bool("approx", false, "use the pruned backbone instead of the exact one")
		ollamaURL  = fs.String("ollama", d.OllamaURL, "local model server")
		embedModel = fs.String("embed-model", d.EmbedModel, "sentence encoder for tier T3")
		chatModel  = fs.String("chat-model", d.ChatModel, "generation model for wiki and answers")
		snapshot   = fs.Int("snapshot-nodes", d.Snapshot, "papers per graph snapshot")
	)
	_ = fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cmd == "doctor" {
		return runDoctorCLI(ctx, Settings{
			DataDir: *data, Addr: *addr, OllamaURL: *ollamaURL,
			EmbedModel: *embedModel, ChatModel: *chatModel, Project: *project,
		})
	}

	st, err := store.Open(*data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store: %v\n", err)
		return 1
	}

	switch cmd {
	case "harvest":
		err = app.Harvest(ctx, st, *project, harvest.Query{
			Categories: app.SplitList(*categories),
			Keywords:   app.SplitList(*keywords),
			From:       *from, To: *to, Max: *maxRecords,
		}, *delay)
	case "build":
		err = app.Build(ctx, st, *project, app.Embedder(ctx, *ollamaURL, *embedModel),
			app.BuildOptions{Approx: *approx, EmbedModel: *embedModel})
	case "analyze":
		err = app.Analyze(st, *project)
	case "graphcheck":
		err = app.GraphCheck(st, *project)
	case "serve":
		err = app.Serve(ctx, st, app.ServeConfig{
			Addr:       *addr,
			Project:    *project,
			Categories: app.SplitList(*categories),
			MaxRecords: *maxRecords,
			Delay:      *delay,
			Snapshot:   *snapshot,
			OllamaURL:  *ollamaURL,
			EmbedModel: *embedModel,
			ChatModel:  *chatModel,
		})
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "reharvester: %v\n", err)
		return 1
	}
	return 0
}

// runTUI starts the interactive program. The data directory is the one setting
// that must be known before the TUI exists, since it decides where preferences
// and logs are read from; everything else lives in the Settings screen.
func runTUI(args []string) error {
	fs := flag.NewFlagSet("reharvester", flag.ContinueOnError)
	dataDir := fs.String("data", DefaultSettings().DataDir, "directory holding projects and artefacts")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// `reharvester --data ./x doctor` would otherwise open the menu and quietly
	// ignore the command, which reads as the flag having broken something.
	if rest := fs.Args(); len(rest) > 0 {
		if isCommand(rest[0]) {
			return fmt.Errorf("flags go after the command — try: reharvester %s --data %s",
				rest[0], *dataDir)
		}
		return fmt.Errorf("unexpected argument %q", rest[0])
	}
	m, err := newRootModel(*dataDir)
	if err != nil {
		return err
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.program = p
	_, err = p.Run()
	m.sink.Close()
	return err
}

// uiEmbedded reports whether this build carries the web UI, which decides
// whether "Start server" can offer to open a browser.
func uiEmbedded() bool { return webui.Embedded() }

// browseURL turns the listen address into something openable.
func browseURL(addr string) string {
	host, port := "localhost", addr
	if j := lastColon(addr); j >= 0 {
		port = addr[j+1:]
		if j > 0 {
			host = addr[:j]
		}
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

func lastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}

// shortDur renders a duration for a status line without the noise of
// sub-second precision.
func shortDur(d time.Duration) string { return d.Round(time.Second).String() }
