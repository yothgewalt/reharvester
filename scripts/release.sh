#!/usr/bin/env bash
# Builds the npm distribution: the web UI is exported, embedded into a Go
# binary per platform, and wrapped in one npm package per platform plus a root
# package that selects between them.
#
#   ./scripts/release.sh                 build only, into npm/
#   ./scripts/release.sh --version 0.2.0 build and set the version everywhere
#   ./scripts/release.sh --publish       build, then publish to npm
#   ./scripts/release.sh --provenance    sign with npm provenance (CI only)
#
# Publishing is never the default: a wrong version cannot be unpublished after
# 72 hours, so it has to be asked for explicitly.
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION=$(node -p "require('./npm/reharvester/package.json').version")
PUBLISH=0
PROVENANCE=0

while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --publish) PUBLISH=1; shift ;;
    --no-publish) PUBLISH=0; shift ;;
    # Provenance links the tarball to the commit and workflow that produced it.
    # It needs an OIDC token, so it only works from CI — a local publish that
    # passes it fails outright rather than publishing unsigned.
    --provenance) PROVENANCE=1; shift ;;
    -h|--help) sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

# platform triples: npm-suffix GOOS GOARCH
PLATFORMS=(
  "darwin-arm64 darwin arm64"
  "darwin-x64   darwin amd64"
  "linux-arm64  linux  arm64"
  "linux-x64    linux  amd64"
  "win32-x64    windows amd64"
  "win32-arm64  windows arm64"
)

say() { printf '\033[36m==>\033[0m %s\n' "$1"; }

for tool in go bun node npm; do
  command -v "$tool" >/dev/null || { echo "missing required tool: $tool" >&2; exit 1; }
done

say "version $VERSION"

# 1. Export the web UI.
#
# NEXT_PUBLIC_API_BASE_URL is forced empty so the bundle infers the API origin
# from the page it was served by. A developer's web/.env.local usually pins it
# to localhost:8000, which would break every install that serves on any other
# address.
say "building the web UI"
( cd web && bun install --frozen-lockfile 2>/dev/null || bun install )
( cd web && NEXT_PUBLIC_API_BASE_URL= bun run build )

[ -f web/dist/index.html ] || { echo "web/dist/index.html missing — the export did not produce a site" >&2; exit 1; }

# 2. Stage it where the go:embed directive can see it.
say "staging web/dist -> internal/webui/dist"
rm -rf internal/webui/dist
cp -R web/dist internal/webui/dist
# Keep the tracked placeholder: without it a clean checkout has no dist/
# directory and the go:embed directive fails to compile.
printf '# Placeholder so the go:embed directive in webui.go compiles from a clean\n# checkout. The release script overwrites this directory with web/dist.\n' \
  > internal/webui/dist/.gitkeep

# 3. Cross-compile. CGO off keeps the Linux binaries portable across libc
#    versions; -trimpath keeps build paths out of the artifact.
LDFLAGS="-s -w -X github.com/yothgewalt/reharvester/internal/app.Version=$VERSION"

for entry in "${PLATFORMS[@]}"; do
  read -r suffix goos goarch <<<"$entry"
  pkg="npm/reharvester-$suffix"
  exe="reharvester"
  [ "$goos" = "windows" ] && exe="reharvester.exe"

  say "building $suffix"
  rm -rf "$pkg"
  mkdir -p "$pkg/bin"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "$LDFLAGS" -o "$pkg/bin/$exe" ./cmd/reharvester

  node - "$pkg" "$suffix" "$goos" "$goarch" "$VERSION" "$exe" <<'NODE'
const fs = require("node:fs");
const [pkg, suffix, goos, goarch, version, exe] = process.argv.slice(2);
const os = { darwin: "darwin", linux: "linux", windows: "win32" }[goos];
const cpu = { amd64: "x64", arm64: "arm64" }[goarch];
fs.writeFileSync(`${pkg}/package.json`, JSON.stringify({
  name: `reharvester-${suffix}`,
  version,
  description: `reharvester binary for ${os}-${cpu}`,
  license: "MIT",
  repository: { type: "git", url: "git+https://github.com/yothgewalt/reharvester.git" },
  os: [os],
  cpu: [cpu],
  files: [`bin/${exe}`],
  preferUnplugged: true,
}, null, 2) + "\n");
NODE

  printf 'Platform binary for reharvester. Install the `reharvester` package instead.\n' > "$pkg/README.md"
  # npm packs from the package directory, so each one needs its own copy of the
  # licence its package.json declares.
  cp LICENSE "$pkg/LICENSE"
done

cp LICENSE npm/reharvester/LICENSE

# 4. Sync the root package version and its optional dependency pins.
say "syncing root package to $VERSION"
node - "$VERSION" <<'NODE'
const fs = require("node:fs");
const version = process.argv[2];
const p = "npm/reharvester/package.json";
const pkg = JSON.parse(fs.readFileSync(p, "utf8"));
pkg.version = version;
for (const dep of Object.keys(pkg.optionalDependencies || {})) {
  pkg.optionalDependencies[dep] = version;
}
fs.writeFileSync(p, JSON.stringify(pkg, null, 2) + "\n");
NODE

say "artifacts"
for entry in "${PLATFORMS[@]}"; do
  read -r suffix _ _ <<<"$entry"
  exe="reharvester"; case "$suffix" in win32-*) exe="reharvester.exe" ;; esac
  ls -lh "npm/reharvester-$suffix/bin/$exe" | awk '{printf "    %-10s %s\n", $5, $9}'
done

if [ "$PUBLISH" -eq 0 ]; then
  cat <<EOF

Built, not published. To try it locally:

    cd npm/reharvester && npm pack
    npm install -g ./reharvester-$VERSION.tgz

Publish with: $0 --version $VERSION --publish
EOF
  exit 0
fi

# 5. Publish platform packages first. The root package's optionalDependencies
#    resolve at install time, so publishing it before its platforms exist
#    produces installs with no binary.
PUBLISH_FLAGS=(--access public)
[ "$PROVENANCE" -eq 1 ] && PUBLISH_FLAGS+=(--provenance)

for entry in "${PLATFORMS[@]}"; do
  read -r suffix _ _ <<<"$entry"
  say "publishing reharvester-$suffix"
  ( cd "npm/reharvester-$suffix" && npm publish "${PUBLISH_FLAGS[@]}" )
done

say "publishing reharvester"
( cd npm/reharvester && npm publish "${PUBLISH_FLAGS[@]}" )

say "published $VERSION"
