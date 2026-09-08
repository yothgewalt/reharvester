#!/usr/bin/env sh

set -e
cd "$(dirname "$0")"

[ $# -eq 0 ] && set -- -project dev

trap 'trap - EXIT; kill 0' EXIT INT TERM

go run ./cmd/harvester-server "$@" &
(cd web && bun run dev) &
wait
