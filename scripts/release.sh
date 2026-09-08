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

# 3. Cross-compile every target into the one package. CGO off keeps the Linux
#    binaries portable across libc versions; -trimpath keeps build paths out of
#    the artifact.
#
#    All six ship together: one npm package rather than a launcher plus six
#    platform packages, which costs about 29 MB of download for roughly 4.7 MB
#    of usable binary, and buys a single publish with no ordering rule and no
#    way to half-publish a version.
LDFLAGS="-s -w -X github.com/yothgewalt/reharvester/internal/app.Version=$VERSION"
BINROOT="npm/reharvester/bin"

for entry in "${PLATFORMS[@]}"; do
  read -r suffix goos goarch <<<"$entry"
  exe="reharvester"
  [ "$goos" = "windows" ] && exe="reharvester.exe"

  say "building $suffix"
  rm -rf "${BINROOT:?}/$suffix"
  mkdir -p "$BINROOT/$suffix"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "$LDFLAGS" -o "$BINROOT/$suffix/$exe" ./cmd/reharvester
done

# npm packs from the package directory, so it needs its own copy of the licence
# its package.json declares.
cp LICENSE npm/reharvester/LICENSE

# 4. Sync the package version.
say "setting version to $VERSION"
node - "$VERSION" <<'NODE'
const fs = require("node:fs");
const p = "npm/reharvester/package.json";
const pkg = JSON.parse(fs.readFileSync(p, "utf8"));
pkg.version = process.argv[2];
fs.writeFileSync(p, JSON.stringify(pkg, null, 2) + "\n");
NODE

say "artifacts"
for entry in "${PLATFORMS[@]}"; do
  read -r suffix _ _ <<<"$entry"
  exe="reharvester"; case "$suffix" in win32-*) exe="reharvester.exe" ;; esac
  ls -lh "$BINROOT/$suffix/$exe" | awk '{printf "    %-10s %s\n", $5, $9}'
done
printf '    %-10s %s\n' "$(du -sh npm/reharvester | cut -f1)" "npm/reharvester (uncompressed)"

if [ "$PUBLISH" -eq 0 ]; then
  cat <<EOF

Built, not published. To try it locally:

    cd npm/reharvester && npm pack
    npm install -g ./reharvester-$VERSION.tgz

Publish with: $0 --version $VERSION --publish
EOF
  exit 0
fi

# 5. Check authentication before uploading anything.
#
# npm refuses to reuse a version even after an unpublish, so a failure partway
# through the loop burns the version: some packages exist at it and the rest
# never will. Catching a bad token here costs one request and keeps the version
# reusable. It cannot detect a token that authenticates but is barred from
# publishing by a 2FA policy — that only surfaces on the first PUT — which is
# what the recovery message below is for.
say "checking npm authentication"
if ! npm_user=$(npm whoami 2>&1); then
  cat >&2 <<EOF
npm is not authenticated: $npm_user

In CI, set the NPM_TOKEN secret to a granular access token with read/write on
all packages and two-factor bypass enabled. Locally, run: npm login
EOF
  exit 1
fi
say "authenticated as $npm_user"

# 6. Publish. One package means no ordering rule and no partial state: the
#    release either lands whole or leaves the version free to reuse.
PUBLISH_FLAGS=(--access public)
[ "$PROVENANCE" -eq 1 ] && PUBLISH_FLAGS+=(--provenance)

say "publishing reharvester@$VERSION"
if ! ( cd npm/reharvester && npm publish "${PUBLISH_FLAGS[@]}" ); then
  echo >&2
  echo "Publish failed. Nothing reached the registry, so version $VERSION is" >&2
  echo "still free — fix the cause and run this again." >&2
  exit 1
fi

say "published $VERSION"
