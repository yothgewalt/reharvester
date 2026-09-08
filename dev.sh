#!/usr/bin/env sh
# Starts the Go API on :8000 and the Next dev server on :3000. Ctrl-C stops both.
# Args go to harvester-server (e.g. ./dev.sh -project paper).
# ponytail: kills the whole process group, so run it as its own job, not sourced.
set -e
cd "$(dirname "$0")"

trap 'trap - EXIT; kill 0' EXIT INT TERM

go run ./cmd/harvester-server "$@" &
(cd web && bun run dev) &
wait
