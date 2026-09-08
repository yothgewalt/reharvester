package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Dep is something the TUI can install. Plans are keyed by GOOS; the first
// plan whose required tool is present wins, so a machine with Homebrew never
// sees the curl-into-shell fallback.
type Dep struct {
	Name  string
	Size  string // shown before a download so the user can judge the cost
	Plans []Plan
	Docs  string // where to go when no plan applies
}

// Plan is one way to install a Dep on one platform.
type Plan struct {
	GOOS    string
	Needs   string // binary that must exist for this plan to apply; "" always applies
	Label   string
	Steps   []Step
	Caveats []string
}

// Step is a single command. Cmd is displayed verbatim before anything runs —
// the user approves what they can read, not a description of it.
type Step struct {
	Cmd       []string
	Shell     string // when set, run via `sh -c` because the vendor ships a pipeline
	NeedsSudo bool
	Note      string
}

// Display renders the command as the user will see it in the confirmation.
func (s Step) Display() string {
	if s.Shell != "" {
		return s.Shell
	}
	return strings.Join(s.Cmd, " ")
}

// Resolve picks the plan for this machine, or ok=false when none applies and
// the user must follow Docs by hand. Never guess: an unresolvable dependency
// prints a URL rather than running something the user did not choose.
func (d *Dep) Resolve() (Plan, bool) {
	for _, p := range d.Plans {
		if p.GOOS != runtime.GOOS {
			continue
		}
		if p.Needs != "" {
			if _, err := exec.LookPath(p.Needs); err != nil {
				continue
			}
		}
		return p, true
	}
	return Plan{}, false
}

func depOllama() *Dep {
	return &Dep{
		Name: "Ollama",
		Docs: "https://ollama.com/download",
		Plans: []Plan{
			{GOOS: "darwin", Needs: "brew", Label: "Homebrew",
				Steps: []Step{{Cmd: []string{"brew", "install", "ollama"}}}},
			{GOOS: "linux", Label: "official install script",
				Steps: []Step{{
					Shell: "curl -fsSL https://ollama.com/install.sh | sh",
					Note:  "downloads and runs a script from ollama.com; it may ask for sudo",
				}},
				Caveats: []string{"This pipes a remote script into a shell. Read it first at https://ollama.com/install.sh if you would rather not."}},
			{GOOS: "windows", Needs: "winget", Label: "winget",
				Steps: []Step{{Cmd: []string{"winget", "install", "--id", "Ollama.Ollama", "-e"}}}},
			{GOOS: "windows", Needs: "scoop", Label: "scoop",
				Steps: []Step{{Cmd: []string{"scoop", "install", "ollama"}}}},
		},
	}
}

func depOllamaModel(model string) *Dep {
	size := "size varies"
	switch {
	case strings.HasPrefix(model, "all-minilm"):
		size = "about 45 MB"
	case strings.HasPrefix(model, "nomic-embed"):
		size = "about 274 MB"
	case strings.HasPrefix(model, "llama3.2"):
		size = "about 2 GB"
	}
	return &Dep{
		Name: "model " + model,
		Size: size,
		Docs: "https://ollama.com/library",
		Plans: []Plan{
			{GOOS: "darwin", Needs: "ollama", Label: "ollama pull",
				Steps: []Step{{Cmd: []string{"ollama", "pull", model}}}},
			{GOOS: "linux", Needs: "ollama", Label: "ollama pull",
				Steps: []Step{{Cmd: []string{"ollama", "pull", model}}}},
			{GOOS: "windows", Needs: "ollama", Label: "ollama pull",
				Steps: []Step{{Cmd: []string{"ollama", "pull", model}}}},
		},
	}
}

func depGo() *Dep {
	return &Dep{
		Name: "Go toolchain",
		Docs: "https://go.dev/dl/",
		Plans: []Plan{
			{GOOS: "darwin", Needs: "brew", Label: "Homebrew",
				Steps: []Step{{Cmd: []string{"brew", "install", "go"}}}},
			{GOOS: "linux", Needs: "apt-get", Label: "apt",
				Steps: []Step{
					{Cmd: []string{"apt-get", "update"}, NeedsSudo: true},
					{Cmd: []string{"apt-get", "install", "-y", "golang-go"}, NeedsSudo: true},
				},
				Caveats: []string{fmt.Sprintf("Distribution packages often lag; this module needs Go 1.%d or newer. If apt gives you an older one, install from https://go.dev/dl/ instead.", goMinor)}},
			{GOOS: "linux", Needs: "dnf", Label: "dnf",
				Steps:   []Step{{Cmd: []string{"dnf", "install", "-y", "golang"}, NeedsSudo: true}},
				Caveats: []string{fmt.Sprintf("Check the installed version reaches 1.%d.", goMinor)}},
			{GOOS: "linux", Needs: "pacman", Label: "pacman",
				Steps: []Step{{Cmd: []string{"pacman", "-S", "--noconfirm", "go"}, NeedsSudo: true}}},
			{GOOS: "linux", Needs: "zypper", Label: "zypper",
				Steps: []Step{{Cmd: []string{"zypper", "install", "-y", "go"}, NeedsSudo: true}}},
			{GOOS: "linux", Needs: "apk", Label: "apk",
				Steps: []Step{{Cmd: []string{"apk", "add", "go"}, NeedsSudo: true}}},
			{GOOS: "windows", Needs: "winget", Label: "winget",
				Steps: []Step{{Cmd: []string{"winget", "install", "--id", "GoLang.Go", "-e"}}}},
			{GOOS: "windows", Needs: "scoop", Label: "scoop",
				Steps: []Step{{Cmd: []string{"scoop", "install", "go"}}}},
		},
	}
}

func depBun() *Dep {
	return &Dep{
		Name: "bun",
		Docs: "https://bun.sh/docs/installation",
		Plans: []Plan{
			{GOOS: "darwin", Needs: "brew", Label: "Homebrew",
				Steps: []Step{{Cmd: []string{"brew", "install", "oven-sh/bun/bun"}}}},
			{GOOS: "darwin", Label: "official install script",
				Steps:   []Step{{Shell: "curl -fsSL https://bun.sh/install | bash"}},
				Caveats: []string{"This pipes a remote script into a shell, and it appends bun to your PATH in your shell rc."}},
			{GOOS: "linux", Label: "official install script",
				Steps:   []Step{{Shell: "curl -fsSL https://bun.sh/install | bash"}},
				Caveats: []string{"This pipes a remote script into a shell, and it appends bun to your PATH in your shell rc."}},
			{GOOS: "windows", Needs: "powershell", Label: "PowerShell installer",
				Steps:   []Step{{Cmd: []string{"powershell", "-c", "irm bun.sh/install.ps1 | iex"}}},
				Caveats: []string{"This downloads and runs a remote PowerShell script."}},
		},
	}
}

func depGit() *Dep {
	return &Dep{
		Name: "git",
		Docs: "https://git-scm.com/downloads",
		Plans: []Plan{
			{GOOS: "darwin", Needs: "brew", Label: "Homebrew",
				Steps: []Step{{Cmd: []string{"brew", "install", "git"}}}},
			{GOOS: "linux", Needs: "apt-get", Label: "apt",
				Steps: []Step{{Cmd: []string{"apt-get", "install", "-y", "git"}, NeedsSudo: true}}},
			{GOOS: "linux", Needs: "dnf", Label: "dnf",
				Steps: []Step{{Cmd: []string{"dnf", "install", "-y", "git"}, NeedsSudo: true}}},
			{GOOS: "linux", Needs: "pacman", Label: "pacman",
				Steps: []Step{{Cmd: []string{"pacman", "-S", "--noconfirm", "git"}, NeedsSudo: true}}},
			{GOOS: "windows", Needs: "winget", Label: "winget",
				Steps: []Step{{Cmd: []string{"winget", "install", "--id", "Git.Git", "-e"}}}},
		},
	}
}

// Command builds the process for a step. Callers run it through
// tea.ExecProcess, which hands over the real terminal — required for a sudo
// password prompt, and it also gives package managers a TTY so their progress
// output behaves.
//
// A Shell step runs through the platform's own interpreter: no Windows plan
// uses one today, since winget and scoop take argument vectors, but routing it
// to `sh` there would fail in a way that reads as a broken installer rather
// than a missing shell.
func (s Step) Command() *exec.Cmd {
	switch {
	case s.Shell != "":
		if runtime.GOOS == "windows" {
			return exec.Command("cmd", "/c", s.Shell)
		}
		return exec.Command("sh", "-c", s.Shell)
	case s.NeedsSudo:
		return exec.Command("sudo", s.Cmd...)
	default:
		return exec.Command(s.Cmd[0], s.Cmd[1:]...)
	}
}

// Prompt is the line shown before a step runs, exactly as it will be executed.
func (s Step) Prompt() string {
	if s.NeedsSudo {
		return "sudo " + s.Display()
	}
	return s.Display()
}

// Risky marks steps that deserve a louder confirmation: anything asking for
// root, or piping a remote script into a shell.
func (s Step) Risky() bool {
	return s.NeedsSudo || strings.Contains(s.Shell, "|")
}
