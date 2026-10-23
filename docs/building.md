# Building and testing

The Linux release-candidate path uses Go 1.27.3. Legacy Windows builds separately pin Go 1.20.14; that legacy compiler is not the production Linux compiler. Development dependencies: Node/npm, GCC, MinGW-w64 x86/x64 GCC and binutils for Windows, and Chrome for browser E2E. Go dependencies: go-sqlite3 v1.14.22 and gofpdf v1.16.2 (checksums in go.sum). Font: embedded DejaVu Sans, license in internal/printing/FONT_LICENSE. UI dependencies are pinned in package-lock.json. Verification uses disposable app databases and loopback; it never opens existing business data. Frontend builds explicitly disable `.env` loading.

## Linux release candidate

With dependencies already installed, commit the source and embedded UI, then run:

```sh
GO_BIN=/snap/go/current/bin/go GOCACHE=/tmp/ledgerleaf-review-go-cache \
  VERSION=0.1.0-rc.1 ./scripts/release-linux.sh
```

The script operates offline except for app-only loopback tests. It runs frontend compilation/unit tests, all Go tests with race detection, vet, the entire browser suite against the packaged executable, and the lifecycle/recovery smoke test. A static native executable, operator guide, readiness assessment, manifest, SHA-256 checksums and verification logs are archived under `release/`. An error stops packaging; logs remain under `release/linux-candidate-*` for diagnosis. Reusing an existing archive name is rejected. `EXPECTED_GO_VERSION` may be changed only after independently qualifying a new compiler. The current script packages standard SQLite, not a SQLCipher working database. The application needs no build tools or network at runtime.

Passing this automation creates a release candidate. Clean-machine acceptance and the remaining gates in [READINESS.md](READINESS.md) are still required for production approval. Windows runtime claims remain separate.

## Legacy Windows builds

On Linux:

```sh
export GO_BIN=/path/to/go1.20.14/bin/go
export GOCACHE=/tmp/billbook-go-cache
./scripts/build-windows.sh
```

For already-installed dependencies, targeted commands:

```sh
npm run build --prefix frontend
npm test --prefix frontend
"$GO_BIN" test ./...
"$GO_BIN" vet ./...
"$GO_BIN" build -buildvcs=false -o release/ledgerleaf-linux ./cmd/billbook
npm run test:e2e --prefix frontend
python3 scripts/smoke.py
```

Python is only used by the optional development smoke test, never by the distributed executable. The E2E runner creates a disposable data directory, starts a loopback server and tests Chromium UI actions. Set CHROME_PATH to a compatible installed Chromium executable. The environment must permit local socket binding and browser subprocesses.

`build-windows.sh` requires ripgrep (`rg`) for import verification. `build-windows.ps1` is provided for Windows hosts with both cross compilers on PATH but has not been executed here. It does not run the Linux-specific browser harness. `verify-build.ps1` checks imports and hashes. Do not mistake dependency inspection for Windows runtime verification.

Artifacts: x86/x64 EXEs, SHA256SUMS, manifest.json, module/build metadata and PE import reports. Frontend and all resources are embedded. The project is version-controlled on `main` as of 2026-10-22. Build scripts retain `-buildvcs=false` so compilation also works from source archives. Generated release executables and local runtime data are excluded from Git; embedded frontend assets are tracked so the Go application can compile from the checkout.

Before release: test clean Windows 7 SP1/8.1 x86/x64 and Windows 10/11 x64 with internet disabled; verify startup, browser opening, CRUD, invoicing, persistence, printing, exit and crash recovery. All such Windows tests are still pending, along with Chrome 109 verification and performance benchmarks.

## Optional SQLCipher Linux build

Use `GO_BIN=/path/to/go1.20.14/bin/go ./scripts/build-linux-encrypted.sh`. This builds a separate `release/ledgerleaf-linux-encrypted` executable using a checksum-pinned SQLCipher 4.19.0 official source archive. It needs GCC/make, Tcl, OpenSSL and zlib development headers; the resulting native Linux executable currently depends on system OpenSSL/zlib runtime libraries. SQLCipher's static library replaces the normal SQLite library via go-sqlite3's `libsqlite3` build tag. Default builds continue to use the original embedded SQLite.

The script defaults its source/build cache to `/tmp/ledgerleaf-sqlcipher-build-4.19.0`; `SQLCIPHER_BUILD_DIR` can override that location. It does not open a database or run tests. Functional CLI verification is `python3 scripts/encryption-smoke.py` after both executables are built. Windows encrypted builds and independent security/dependency review are final-stage work.

See [TESTING_GUIDE.md](TESTING_GUIDE.md) for password input, conversion and the distinction between working-database encryption and backup encryption.

SQLCipher integration follows its [official API documentation](https://www.zetetic.net/sqlcipher/sqlcipher-api/) and [4.19.0 source release](https://github.com/sqlcipher/sqlcipher/releases/tag/v4.19.0).

### Local development verification on 2026-10-22

The temporary Go 1.20.14 and SQLCipher build directories from the earlier session were no longer available. Current frontend changes were compiled into `release/ledgerleaf-linux-dev` with the installed Go 1.27.2 compiler (direct `/snap/go/current/bin/go`, offline module resolution). This is a **development-only Linux executable**, not a pinned release build. The previous standard and encrypted release binaries were retained and do not contain these latest bank-dialog/language changes. Restore the pinned toolchains before refreshing those release artifacts.

The regression browser harness accepts an absolute `LEDGERLEAF_TEST_BINARY` override. For example:

```sh
LEDGERLEAF_TEST_BINARY="$PWD/release/ledgerleaf-linux-dev" npm run test:e2e --prefix frontend -- --grep 'bank creation and statement|language selection is available'
```

### Offline interface checks

After building the frontend, run `npm run test:ui:offline --prefix frontend`. The harness bundles the real expense, stock and bank/period components with an in-memory API stub, then drives installed Chrome (`CHROME_PATH` can override the executable). Browser contexts are offline and all browser network requests are blocked. No application server or database is opened. These checks cover form behavior and request payloads; they do not verify server accounting or database transactions.

The Git/dialog continuation on 2026-10-22 rebuilt `release/ledgerleaf-linux-dev` offline with installed Go **1.27.3**, superseding the development-artifact version recorded earlier on this page. Compilation and vet pass; the executable was not launched under the current no-database/no-service restriction. Pinned Go 1.20.14 release builds remain pending.
