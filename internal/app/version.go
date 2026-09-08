package app

// Version is reported by /health and by `reharvester --version`. The release
// script overrides it with -ldflags "-X github.com/yothgewalt/reharvester/internal/app.Version=…"
// so the npm package version is the single source of truth; this default is
// what a plain `go build` reports.
var Version = "0.1.0"
