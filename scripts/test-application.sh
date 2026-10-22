#!/usr/bin/env bash
# Linux development verification. Never opens an existing business database.
set -euo pipefail
cd "$(dirname "$0")/.."
GO_BIN=${GO_BIN:-go}
mkdir -p release
npm run build --prefix frontend
npm test --prefix frontend
"$GO_BIN" test -race -count=1 -coverpkg=./... -coverprofile=release/coverage.out ./...
"$GO_BIN" vet ./...
"$GO_BIN" tool cover -func=release/coverage.out > release/coverage-summary.txt
"$GO_BIN" build -buildvcs=false -o release/ledgerleaf-linux ./cmd/billbook
npm run test:e2e --prefix frontend
python3 scripts/smoke.py
