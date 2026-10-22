#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
GO_BIN=${GO_BIN:-go}
if [[ "$($GO_BIN version)" != 'go version go1.20.14 '* ]]; then
  echo 'Release builds require Go 1.20.14. Set GO_BIN to its go executable.' >&2
  exit 1
fi
command -v node >/dev/null
command -v npm >/dev/null
command -v i686-w64-mingw32-gcc >/dev/null
command -v x86_64-w64-mingw32-gcc >/dev/null
npm ci --prefix frontend --ignore-scripts --no-audit --no-fund
npm run build --prefix frontend
npm test --prefix frontend
test -s internal/web/dist/index.html
"$GO_BIN" test ./...
"$GO_BIN" vet ./...
mkdir -p release
"$GO_BIN" build -buildvcs=false -o release/ledgerleaf-linux ./cmd/billbook
npm run test:e2e --prefix frontend
for arch in 386 amd64; do
  if [[ "$arch" == 386 ]]; then compiler=i686-w64-mingw32-gcc; name=x86; else compiler=x86_64-w64-mingw32-gcc; name=x64; fi
  CGO_ENABLED=1 GOOS=windows GOARCH="$arch" CC="$compiler" "$GO_BIN" build -buildvcs=false -trimpath -ldflags='-s -w -extldflags=-static' -o "release/BillBook-Windows-$name.exe" ./cmd/billbook
  "${compiler%-gcc}-objdump" -p "release/BillBook-Windows-$name.exe" > "release/imports-$name.txt"
  if rg 'DLL Name:' "release/imports-$name.txt" | rg -vi 'DLL Name: (KERNEL32.dll|msvcrt.dll)$'; then echo 'Unexpected DLL dependency' >&2; exit 1; fi
done
sha256sum release/BillBook-Windows-*.exe > release/SHA256SUMS
"$GO_BIN" version -m release/BillBook-Windows-x64.exe > release/build-modules.txt
node scripts/manifest.mjs
