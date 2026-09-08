package main

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yothgewalt/reharvester/internal/app"
	"github.com/yothgewalt/reharvester/internal/rag"
	"github.com/yothgewalt/reharvester/internal/store"
)

// Status is a check outcome. Missing is not the same as broken: most checks
// here gate an optional capability, and the system runs without them.
type Status int

const (
	StatusOK Status = iota
	StatusWarn
	StatusMissing
)

// Check is one row of the Doctor. Dep is non-nil when the TUI can offer to
// install what is missing; Fix is a hint shown when it cannot.
type Check struct {
	Name    string
	Unlocks string
	Status  Status
	Detail  string
	Dep     *Dep
	Fix     string
}

// goMinor is the Go minor version this module requires. An older toolchain is
// a failure rather than a pass: it will not compile the module at all.
const goMinor = 26

// runChecks inspects the machine. It never mutates anything, so it is safe to
// call on every launch and after every install.
func runChecks(ctx context.Context, s Settings) []Check {
	var out []Check

	o := rag.NewOllama(s.OllamaURL, s.EmbedModel, s.ChatModel)
	probe, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ollamaUp := o.Available(probe)

	out = append(out, Check{
		Name:    "Ollama",
		Unlocks: "dense tier T3 and generated wiki prose",
		Status:  statusIf(ollamaUp, StatusWarn),
		Detail:  ternary(ollamaUp, "reachable at "+s.OllamaURL, "no server at "+s.OllamaURL),
		Dep:     depOllama(),
	})

	if ollamaUp {
		hasEmbed := o.HasModel(probe, s.EmbedModel)
		out = append(out, Check{
			Name:    "Encoder " + s.EmbedModel,
			Unlocks: "tier T3 dense retrieval",
			Status:  statusIf(hasEmbed, StatusWarn),
			Detail:  ternary(hasEmbed, "pulled", "not pulled"),
			Dep:     depOllamaModel(s.EmbedModel),
		})
		canGen := o.CanGenerate(probe)
		out = append(out, Check{
			Name:    "Generation model",
			Unlocks: "wiki synthesis and answers",
			Status:  statusIf(canGen, StatusWarn),
			Detail:  ternary(canGen, "using "+o.ChatModel, "none suitable pulled"),
			Dep:     depOllamaModel(rag.SuggestChatModel()),
		})
	}

	goOK, goVer := detectGo()
	out = append(out, Check{
		Name:    "Go toolchain",
		Unlocks: "building from source",
		Status:  statusIf(goOK, StatusWarn),
		Detail:  ternary(goVer != "", goVer, "not installed"),
		Dep:     depGo(),
	})

	bunOK, bunVer := detectVersion("bun", "--version")
	out = append(out, Check{
		Name:    "bun",
		Unlocks: "rebuilding the web UI",
		Status:  statusIf(bunOK, StatusWarn),
		Detail:  ternary(bunOK, "bun "+bunVer, "not installed"),
		Dep:     depBun(),
	})

	gitOK, gitVer := detectVersion("git", "--version")
	out = append(out, Check{
		Name:    "git",
		Unlocks: "working from a source checkout",
		Status:  statusIf(gitOK, StatusWarn),
		Detail:  ternary(gitOK, strings.TrimPrefix(gitVer, "git version "), "not installed"),
		Dep:     depGit(),
	})

	free, portDetail := portFree(s.Addr)
	out = append(out, Check{
		Name:    "Port " + s.Addr,
		Unlocks: "serving the API and UI",
		Status:  statusIf(free, StatusWarn),
		Detail:  portDetail,
		Fix:     ternary(free, "", "choose another address in Settings, or stop the process holding it"),
	})

	if gb, ok := freeGB(s.DataDir); ok {
		enough := gb >= 2
		out = append(out, Check{
			Name:    "Disk space",
			Unlocks: "harvesting a corpus",
			Status:  statusIf(enough, StatusWarn),
			Detail:  fmt.Sprintf("%.1f GB free in %s", gb, absOr(s.DataDir)),
			Fix:     ternary(enough, "", "a full harvest and its indexes want a couple of GB"),
		})
	}

	out = append(out, corpusChecks(s)...)
	return out
}

// corpusChecks cover state rather than software: whether there is anything to
// serve, and whether the encoder that built it matches the one configured now.
func corpusChecks(s Settings) []Check {
	st, err := store.Open(s.DataDir)
	if err != nil {
		return []Check{{
			Name: "Data directory", Unlocks: "everything",
			Status: StatusMissing, Detail: err.Error(),
		}}
	}
	id := app.PickProject(st, s.Project)
	if id == "" {
		return []Check{{
			Name: "Corpus", Unlocks: "building, serving and analysis",
			Status: StatusWarn, Detail: "no project holds papers yet",
			Fix: "run Harvest first",
		}}
	}
	out := []Check{{
		Name: "Corpus", Unlocks: "building, serving and analysis",
		Status: StatusOK, Detail: "project " + id,
	}}
	// A corpus embedded with one encoder and queried with another returns an
	// empty dense result set with no error, so surface the mismatch here.
	if rec, ok := app.ReadEmbedRecord(st, id); ok && rec.Model != s.EmbedModel {
		out = append(out, Check{
			Name:    "Encoder match",
			Unlocks: "correct dense results",
			Status:  StatusMissing,
			Detail:  fmt.Sprintf("built with %q, configured %q", rec.Model, s.EmbedModel),
			Fix:     "rebuild, or set the encoder back — dense search silently returns nothing while these differ",
		})
	}
	return out
}

func detectGo() (bool, string) {
	ok, v := detectVersion("go", "version")
	if !ok {
		return false, ""
	}
	// "go version go1.27.1 darwin/arm64"
	fields := strings.Fields(v)
	if len(fields) < 3 {
		return false, v
	}
	ver := strings.TrimPrefix(fields[2], "go")
	parts := strings.Split(ver, ".")
	if len(parts) < 2 {
		return false, ver
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, ver
	}
	major, _ := strconv.Atoi(parts[0])
	if major > 1 || minor >= goMinor {
		return true, "go " + ver
	}
	return false, fmt.Sprintf("go %s — this module needs 1.%d or newer", ver, goMinor)
}

func detectVersion(bin string, args ...string) (bool, string) {
	if _, err := exec.LookPath(bin); err != nil {
		return false, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	b, err := exec.CommandContext(ctx, bin, args...).Output()
	if err != nil {
		return true, "" // present but unhappy; treat presence as the answer
	}
	return true, strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0])
}

// portFree reports whether the API can bind. It listens and immediately closes
// rather than probing with a dial, which would confuse "nothing is listening"
// with "something refused us".
func portFree(addr string) (bool, string) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false, "in use — " + err.Error()
	}
	_ = ln.Close()
	return true, "free"
}

func absOr(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

func statusIf(ok bool, otherwise Status) Status {
	if ok {
		return StatusOK
	}
	return otherwise
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// runDoctorCLI is the scriptable form: a plain table and a non-zero exit when
// something is missing outright, so CI can gate on it.
func runDoctorCLI(ctx context.Context, s Settings) int {
	s = withDefaults(s)
	checks := runChecks(ctx, s)
	worst := StatusOK
	for _, c := range checks {
		mark := "ok"
		switch c.Status {
		case StatusWarn:
			mark = "warn"
		case StatusMissing:
			mark = "MISSING"
		}
		fmt.Printf("%-8s %-22s %s\n", mark, c.Name, c.Detail)
		if c.Fix != "" && c.Status != StatusOK {
			fmt.Printf("%-8s %-22s %s\n", "", "", c.Fix)
		}
		if c.Status > worst {
			worst = c.Status
		}
	}
	if !uiEmbedded() {
		fmt.Println("\nnote: this build has no web UI embedded — the API serves without one")
	}
	if worst == StatusMissing {
		return 1
	}
	return 0
}
