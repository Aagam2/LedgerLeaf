#!/usr/bin/env bash
# Build and verify one Linux release candidate using installed dependencies only.
# All runtime checks use disposable application data and loopback connections.
set -euo pipefail
cd "$(dirname "$0")/.."
GO_BIN=${GO_BIN:-go}
VERSION=${VERSION:-0.1.0-rc.3}
EXPECTED_GO_VERSION=${EXPECTED_GO_VERSION:-go1.27.3}
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.-]+)?$ ]] || { echo 'Invalid VERSION' >&2; exit 1; }
[[ "$("$GO_BIN" env GOVERSION)" == "$EXPECTED_GO_VERSION" ]] || { echo "Requires $EXPECTED_GO_VERSION; set EXPECTED_GO_VERSION only after qualifying another compiler." >&2; exit 1; }
[[ "$("$GO_BIN" env GOHOSTOS)" == linux ]] || { echo 'Run on Linux.' >&2; exit 1; }
[[ -z "$(git status --porcelain --untracked-files=normal)" ]] || { echo 'Commit the source and embedded UI before packaging.' >&2; exit 1; }
for command in npm node python3 gcc tar sha256sum readelf; do command -v "$command" >/dev/null; done
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
# Neither native extension loading nor the system DNS resolver is needed by a
# loopback-only ledger. Remove their dlopen/glibc runtime dependencies.
export GOFLAGS='-tags=netgo,sqlite_omit_load_extension'
export NO_UPDATE_NOTIFIER=1 NPM_CONFIG_UPDATE_NOTIFIER=false
export CGO_ENABLED=1 GOOS=linux
GOARCH=$("$GO_BIN" env GOHOSTARCH)
export GOARCH
REVISION=$(git rev-parse HEAD)
SOURCE_DATE_EPOCH=$(git show -s --format=%ct HEAD)
export SOURCE_DATE_EPOCH
BUILT_AT=$(date -u -d "@$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ)
mkdir -p release
NAME="Ledgerleaf-$VERSION-linux-$GOARCH"
ARCHIVE="$PWD/release/$NAME.tar.gz"
[[ ! -e "$ARCHIVE" ]] || { echo 'Archive already exists; use a new VERSION.' >&2; exit 1; }
STAGE=$(mktemp -d "$PWD/release/linux-candidate-XXXXXX")
PACKAGE="$STAGE/$NAME"
mkdir -p "$PACKAGE/verification"
echo "Building and verifying $NAME; logs: $PACKAGE/verification"
"$GO_BIN" mod verify > "$PACKAGE/verification/go-module-checksums.log" 2>&1
npm ls --prefix frontend --depth=0 > "$PACKAGE/verification/frontend-dependencies.log" 2>&1
npm run build --prefix frontend > "$PACKAGE/verification/frontend-build.log" 2>&1
git diff --exit-code -- internal/web/dist > "$PACKAGE/verification/embedded-assets.log"
npm test --prefix frontend > "$PACKAGE/verification/frontend-tests.log" 2>&1
"$GO_BIN" test -race -count=1 ./... > "$PACKAGE/verification/go-tests.log" 2>&1
"$GO_BIN" vet ./... > "$PACKAGE/verification/go-vet.log" 2>&1
"$GO_BIN" build -buildvcs=false -trimpath \
  -ldflags="-s -w -extldflags=-static -X main.version=$VERSION -X main.revision=$REVISION -X main.builtAt=$BUILT_AT" \
  -o "$PACKAGE/ledgerleaf" ./cmd/billbook > "$PACKAGE/verification/binary-build.log" 2>&1
readelf -l "$PACKAGE/ledgerleaf" > "$PACKAGE/verification/elf-program-headers.txt"
python3 - "$PACKAGE/verification/elf-program-headers.txt" <<'PY'
import pathlib, sys
assert 'INTERP' not in pathlib.Path(sys.argv[1]).read_text(), 'Executable requires a dynamic loader'
PY
export LEDGERLEAF_TEST_BINARY="$PACKAGE/ledgerleaf"
npm run test:e2e --prefix frontend -- --reporter=line --output="$STAGE/browser-results" > "$PACKAGE/verification/browser-tests.log" 2>&1
python3 scripts/smoke.py > "$PACKAGE/verification/lifecycle-smoke.log" 2>&1
[[ "$(git rev-parse HEAD)" == "$REVISION" && -z "$(git status --porcelain --untracked-files=normal)" ]] || { echo 'Source changed during verification; packaging cancelled.' >&2; exit 1; }
"$PACKAGE/ledgerleaf" --version > "$PACKAGE/verification/version.txt"
"$GO_BIN" version -m "$PACKAGE/ledgerleaf" > "$PACKAGE/verification/go-build.txt"
cp docs/OPERATIONS.md "$PACKAGE/START-HERE.md"
cp docs/READINESS.md "$PACKAGE/READINESS.md"
python3 - "$PACKAGE" "$VERSION" "$REVISION" "$BUILT_AT" "$GO_BIN" <<'PY'
import hashlib, json, pathlib, subprocess, sys
root, version, revision, built_at, go = sys.argv[1:]
root = pathlib.Path(root)
binary = root / 'ledgerleaf'
manifest = {
    'product': 'Ledgerleaf', 'version': version, 'revision': revision, 'built_at': built_at,
    'status': 'Linux release candidate; see READINESS.md for release gates',
    'go': subprocess.check_output([go, 'version'], text=True).strip(),
    'node': subprocess.check_output(['node', '--version'], text=True).strip(),
    'encryption': 'Standard SQLite; encrypted backup downloads supported; working database is plaintext',
    'build_tags': ['netgo', 'sqlite_omit_load_extension'],
    'dependency_locks': {str(p): hashlib.sha256(p.read_bytes()).hexdigest() for p in [pathlib.Path('go.sum'), pathlib.Path('frontend/package-lock.json')]},
    'verified': ['frontend build and unit tests', 'Go race tests', 'Go vet', 'full browser suite', 'crash, restart, shutdown and recovery smoke'],
    'binary': {'name': binary.name, 'bytes': binary.stat().st_size, 'sha256': hashlib.sha256(binary.read_bytes()).hexdigest()},
}
(root / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
PY
(cd "$PACKAGE" && sha256sum ledgerleaf START-HERE.md READINESS.md manifest.json > SHA256SUMS)
tar --sort=name --mtime="@$SOURCE_DATE_EPOCH" --owner=0 --group=0 --numeric-owner -czf "$ARCHIVE" -C "$STAGE" "$NAME"
(cd release && sha256sum "$NAME.tar.gz" > "$NAME.tar.gz.sha256")
echo "Verified candidate: $ARCHIVE"
echo 'Target-machine acceptance and independent security/dependency review remain release gates.'
