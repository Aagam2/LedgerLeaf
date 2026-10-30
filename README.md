# LedgerLeaf

LedgerLeaf is an offline desktop billing, inventory and accounting application built with Go, React, TypeScript and SQLite. A single executable serves the interface in your browser and stores records locally; no separate database server is required.

The daily workflow is **create a bill → save and review → finalize → download PDF or PNG → record payment**. Purchase orders, customers and suppliers, products, stock, expenses, reports, local users, and backup/restore are also supported. PDF is the primary document export; long PNG exports provide one image per page.

**Status:** Linux release candidate. Automated Linux verification covers 44 browser workflows, 12 frontend tests, Go race tests and recovery checks. Independent security review and target-machine acceptance remain outstanding; Windows runtime compatibility is unverified. See [readiness](docs/READINESS.md) and [test results](docs/TEST_REPORT.md) for scope and limitations.

## Prerequisites

For building from source on Linux:

| Tool | Purpose |
| --- | --- |
| Git | Clone the repository. |
| Go 1.27.3 | Toolchain used for the verified Linux candidate; the release script checks this exact version. |
| Node.js 22 and npm | Install and build the frontend; Node 22 was used for verification. |
| GCC and a C development toolchain | Required by the SQLite driver through CGO. |
| A current Chrome or Chromium browser | Use the application; also required for browser tests. |
| Python 3 | Required for lifecycle/recovery smoke tests, not for running the application. |

On Debian/Ubuntu, the native build tools can be installed with:

```sh
sudo apt update
sudo apt install git build-essential python3
```

Install Go, Node.js/npm and a browser separately using the appropriate instructions for your operating system. Verify the build tools are on your `PATH`:

```sh
git --version
go version
node --version
npm --version
gcc --version
```

Downloading source dependencies requires an internet connection initially. The built application runs offline and needs only its executable, writable local storage and a compatible browser. Go, Node.js, npm and Python are not runtime dependencies.

## Set up and build

Run these commands from any directory where you keep projects:

```sh
git clone https://github.com/Aagam2/LedgerLeaf.git
cd LedgerLeaf

# Install the pinned frontend and Go dependencies.
npm ci --prefix frontend
go mod download

# Build the frontend, then embed it in the Go executable.
npm run build --prefix frontend
mkdir -p release
CGO_ENABLED=1 go build -buildvcs=false -o release/ledgerleaf-linux-dev ./cmd/billbook
```

Generated executables and release archives are not committed to the repository, so a fresh clone needs the build step. Frontend assets are embedded in the executable: after changing the UI, rebuild both the frontend and the Go executable.

No `.env` file, API key, cloud account or external database is needed. Frontend builds explicitly disable automatic `.env` loading. SQLite is created and migrated automatically on first startup.

## Start the application

From the repository root after building:

```sh
./release/ledgerleaf-linux-dev --data-dir ./manual-test-data
```

This opens your browser and keeps the trial installation in `manual-test-data/`. That directory is ignored by Git. Relative data paths are resolved from the directory where you run the command; always use the same location when reopening an installation.

To use the default per-user data directory instead:

```sh
./release/ledgerleaf-linux-dev
```

The default is the operating system's user configuration directory under `BillBook` (typically `~/.config/BillBook` on Linux, or under `XDG_CONFIG_HOME` when set). For portable storage, pass `--data-dir` with a directory you own on a local disk. Avoid shared network folders and live cloud-sync folders for the working database.

Useful options:

```sh
# Print the launch link without opening a browser automatically.
./release/ledgerleaf-linux-dev --no-browser --data-dir ./manual-test-data

# Show build information without opening a database.
./release/ledgerleaf-linux-dev --version
```

Keep the terminal open while using LedgerLeaf. Use **Exit application** or Ctrl+C to stop it; closing a browser tab leaves the backend running. If the browser does not open, copy the launch link printed in the terminal into your browser. Treat that link as a local access credential. Only one process can use a given data directory at a time.

### First-use setup

1. Create your business and check its address, tax scheme and invoice details.
2. Create an administrator in **Users & permissions**, then sign in. Until a user is created, the local launch link grants owner access.
3. Add customers, suppliers and products. Enter opening stock before selling stocked products.
4. Enable automatic backups in **Backup & restore**; scheduling is opt-in. Keep a verified backup outside the computer as well.
5. Create a trial bill, use **Save & review**, confirm the amount and finalize it. Check **Download PDF**, optionally **Export image (PNG)**, then record a payment.

Records are created from your input; no demo transactions are inserted. See [the testing guide](docs/TESTING_GUIDE.md) for detailed walkthroughs.

## Run tests

From the repository root, after installing dependencies and building the application:

```sh
# Frontend unit tests.
npm test --prefix frontend

# Backend tests and static checks.
go test -race -count=1 ./...
go vet ./...

# Browser workflows against the executable built above.
LEDGERLEAF_TEST_BINARY="$PWD/release/ledgerleaf-linux-dev" npm run test:e2e --prefix frontend

# Compiled application lifecycle, financial integrity and recovery checks.
LEDGERLEAF_TEST_BINARY="$PWD/release/ledgerleaf-linux-dev" python3 scripts/smoke.py
```

Browser tests default to `/usr/bin/google-chrome`. Set `CHROME_PATH` to your installed Chrome/Chromium executable if needed, for example:

```sh
CHROME_PATH=/usr/bin/chromium LEDGERLEAF_TEST_BINARY="$PWD/release/ledgerleaf-linux-dev" npm run test:e2e --prefix frontend
```

The complete Linux verification script builds the frontend and `release/ledgerleaf-linux`, runs unit/race/browser tests and recovery smoke, and writes coverage results under `release/`:

```sh
./scripts/test-application.sh
```

Set `GO_BIN` if your Go executable is not named `go` on `PATH`, and `CHROME_PATH` if necessary. Tests use disposable application databases and local loopback servers; the environment must allow browser subprocesses and local socket binding.

## Release packages and Windows

To create a verified Linux candidate, first install dependencies and commit the source and rebuilt embedded assets, then run:

```sh
VERSION=0.1.0-rc.4 ./scripts/release-linux.sh
```

Use an unused version name. Packaging additionally requires GNU `tar`, `sha256sum`, `readelf` (binutils), and static C libraries. The script checks Go 1.27.3, uses installed dependencies without downloading, reruns verification and writes a static executable, operator guide, manifest, checksums and logs into an archive under `release/`. It requires a clean Git working tree. Passing automation creates a candidate; it does not waive the remaining production acceptance gates.

If you received a packaged Linux build separately, extract its archive, run `sha256sum -c SHA256SUMS` inside the extracted folder, then start `./ledgerleaf`. No source checkout or build tools are needed for that executable.

Windows build scripts are available but use a separate legacy compatibility toolchain: Go 1.20.14 and MinGW-w64 x86/x64 compilers. They generate `release/BillBook-Windows-x86.exe` and `release/BillBook-Windows-x64.exe`. Windows binaries have been cross-compiled, not runtime-qualified. See [building](docs/building.md) and [compatibility](docs/compatibility.md) before attempting a Windows build. SQLCipher working-database builds are also separate from the standard build above.

## Backups and local data

The standard working database and local snapshots are plaintext. Password-encrypted backup downloads are available; they do not encrypt the working database. Backups contain all businesses and users in the installation. Use SQLite backups for complete restores; CSV/XLSX exports are for reporting and inspection.

Before upgrades or recovery, stop the application and preserve the complete data directory, including any WAL/SHM files. See [the operator guide](docs/OPERATIONS.md) for verified backup, restore, administrator recovery and upgrade procedures.

Environment files, private keys, local trial data, database files, generated executables and installed dependencies are excluded from Git. Do not commit real business data, logs, backup files or access credentials.

## Documentation

- [Operator guide](docs/OPERATIONS.md): installation, first use, backups, upgrades and recovery.
- [Build instructions](docs/building.md): detailed Linux, legacy Windows and encryption builds.
- [Testing guide](docs/TESTING_GUIDE.md) and [test report](docs/TEST_REPORT.md): walkthroughs, executed checks and limits.
- [Readiness](docs/READINESS.md) and [implementation status](docs/IMPLEMENTATION_STATUS.md): current scope and outstanding work.
- [Architecture](docs/architecture.md), [database](docs/database.md) and [accounting](docs/accounting.md): technical design.
- [GST scope](docs/gst.md): configured tax behavior and preparation-export limitations.
- [Interface principles](docs/UX_PRINCIPLES.md), [localization](docs/LOCALIZATION.md) and [file map](docs/FILE_MAP.md).

Offline **Help** is available in the application in English, Hindi and Gujarati, with English fallback for untranslated messages.
