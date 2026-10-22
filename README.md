# Ledgerleaf

> Latest development batch (2026-10-22): use `./release/ledgerleaf-linux-dev --data-dir ./manual-test-data` with fresh test data for the new goods/bank dialogs, named-bank expenses and login/startup fixes. The latest local development build uses Go 1.27.3. Older release binaries are retained pending pinned-toolchain rebuilding. See [implementation status](docs/IMPLEMENTATION_STATUS.md) and [test results](docs/TEST_REPORT.md).


An original offline desktop business application built with Go, React, TypeScript, Tailwind and embedded SQLite. This is an **incomplete development build**, not the complete production system described in the supplied specification.

Working features include multiple businesses; products, parties and warehouses; GST and non-GST sales/purchases; partial returns; POS with held bills and split payments; commercial documents; partial purchase-order goods receipts and invoice matching; inventory; accounting and bank reconciliation; financial reports and CSV/XLSX/PDF exports; master imports; verified backup/restore and automatic backup retention; local users with business assignments; offline administrator recovery; rotating logs; alternate units; business logos/product images; and product-sales/HSN summaries. All business data comes from SQLite. No demo transactions are inserted.

The full specification remains incomplete. See [implementation status](docs/IMPLEMENTATION_STATUS.md) for the remaining work and [test results](docs/TEST_REPORT.md) for verification limits.

The latest source additionally includes financial-year/variant settings, recurring expenses, year closing, quantity cess, encrypted backup copies, independent dispatches, journal imports and offline restore assistance. Consolidated Linux verification is recorded in the status and test report above. Earlier consolidated verification used `release/ledgerleaf-linux`; the latest source uses the development artifact described above.

For the latest Linux trial, use the development executable:

```sh
./release/ledgerleaf-linux-dev --data-dir ./manual-test-data
```

It opens your browser and keeps test records in a separate installation. No build tools or internet connection are needed to run it. Keep its terminal open while using the application. The standard/encrypted Linux and Windows artifacts predate the latest changes; pinned release rebuilding remains pending. Build-from-source instructions are in [building.md](docs/building.md).

## Start testing

Read [Run and test Ledgerleaf yourself](docs/TESTING_GUIDE.md) for exact startup commands, a first transaction walkthrough, test expectations, backup/encryption checks and troubleshooting. [Project file map](docs/FILE_MAP.md) explains each project-owned file and generated artifact group.

Latest additions include atomic sales/purchase file imports, multi-dispatch invoice drafts, GSTR-1/GSTR-3B preparation exports and an optional SQLCipher Linux build. The current trial launch command is:

```bash
cd /home/aagam-linux/Desktop/Projects/MyBillBook
./release/ledgerleaf-linux-dev --data-dir ./manual-test-data
```

## Run

Windows artifacts are generated in `release/BillBook-Windows-x86.exe` and `release/BillBook-Windows-x64.exe`. Double-click the appropriate executable. It opens a compatible installed browser. No development runtime is needed. These binaries have been cross-compiled, **not runtime-tested on Windows**.

Chromium 109 or newer is the browser target; Internet Explorer is unsupported. Windows 7 requires an already-installed compatible browser. See [compatibility](docs/compatibility.md).

The console remains open. Use **Exit application** or Ctrl+C to stop the backend; closing a browser tab does not stop it. If automatic browser launching fails, open the launch URL printed in the console. Treat that URL as a local access credential.

Default Windows data directory: `%APPDATA%/BillBook`. Portable mode:

```powershell
.\BillBook-Windows-x64.exe --data-dir D:\LedgerleafData
```

Development host:

```sh
./release/ledgerleaf-linux --no-browser --data-dir /tmp/my-ledgerleaf-test
```

## Build and verification

Requires Go **1.20.14**, Node 22/npm, and MinGW-w64 x86/x64 compilers on the build machine. Chrome is required for browser tests. Dependencies are pinned; installation needs internet once, but the resulting application does not.

```sh
GO_BIN=/path/to/go1.20.14/bin/go GOCACHE=/tmp/billbook-go-cache ./scripts/build-windows.sh
```

The Linux script builds the UI, tests Go and frontend code, runs a browser workflow, compiles both Windows binaries, inspects their imports, and generates hashes and a manifest. See [building](docs/building.md) and [usage](docs/usage.md).

Documentation: [architecture](docs/architecture.md), [database](docs/database.md), [accounting](docs/accounting.md), [GST](docs/gst.md).

## Application tests

See [test results and verification limits](docs/TEST_REPORT.md). Run the complete Linux verification suite with installed dependencies:

```sh
GO_BIN=/path/to/go1.20.14/bin/go GOCACHE=/tmp/billbook-go-cache ./scripts/test-application.sh
```

Tests use disposable SQLite databases, exercise the real executable and browser, and check financial rollback, concurrent writes, PDF content and crash/restart persistence.

The application includes offline **Help** with workflow guides in English, Hindi and Gujarati. Choose **Language / भाषा / ભાષા** in the sidebar or login screen; English is the default. Navigation and common labels are localized, with English fallback for remaining messages. [Localization scope](docs/LOCALIZATION.md).

Administrators can download an all-business, all-table Excel workbook from **Backup & restore**. The workbook is an unencrypted inspection export; use SQLite backups for complete restores. **Import & export** provides blank product/customer/supplier Excel templates and validated invoice/journal imports. See [the testing guide](docs/TESTING_GUIDE.md#entire-database-workbook-and-excel-import-templates) for formats and units.

## Source control

The local repository uses `main`. Initial commit `7aa7c24` captures the existing source, documentation, tests, migrations and embedded web assets before further changes. Local executables, release artifacts, installed dependencies, Python caches, database files and `.env` files are excluded.
