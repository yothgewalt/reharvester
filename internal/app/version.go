package app

import (
	"regexp"
	"runtime/debug"
	"strings"
)

// Version is reported by /health and by `reharvester --version`. The release
// script overrides it with -ldflags "-X github.com/yothgewalt/reharvester/internal/app.Version=…"
// so the npm package version is the single source of truth. Left empty, a plain
// `go build` reports the version Go stamped from git: the tag itself on a tagged
// commit, otherwise "<next>-dev+<commit>" with ".dirty" for uncommitted changes.
var Version = ""

func init() {
	if Version != "" {
		return
	}
	Version = "dev"
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := fromModuleVersion(bi.Main.Version); v != "" {
			Version = v
		}
	}
}

var pseudoVersion = regexp.MustCompile(`^(\d+\.\d+\.\d+)-(?:.*\.)?\d{14}-([0-9a-f]{12})$`)

// fromModuleVersion turns a Go module version into a display version without
// the leading "v", or "" when Go recorded none.
func fromModuleVersion(v string) string {
	if v == "" || v == "(devel)" {
		return ""
	}
	v = strings.TrimPrefix(v, "v")
	v, dirty := strings.CutSuffix(v, "+dirty")
	if m := pseudoVersion.FindStringSubmatch(v); m != nil {
		v = m[1] + "-dev+" + m[2][:7]
		if dirty {
			v += ".dirty"
		}
		return v
	}
	if dirty {
		v += "+dirty"
	}
	return v
}
