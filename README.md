# LedgerLeaf

> Current release scope (2026-10-23): simple everyday invoices, purchase orders, payments, stock, reports, backups and PDF-first/PNG exports. Linux production hardening includes startup integrity checks, safety snapshots before upgrades, graceful shutdown and verified packaging. See [readiness and release gates](docs/READINESS.md), [the operator guide](docs/OPERATIONS.md) and [build instructions](docs/building.md).

Latest verified Linux package: `release/Ledgerleaf-0.1.0-rc.3-linux-amd64.tar.gz` (application revision `d431c3c`). Extract it, verify `SHA256SUMS`, then run `./ledgerleaf`. Locally, `release/ledgerleaf-linux` and `release/ledgerleaf-linux-dev` contain that same executable. All 44 browser workflows, 12 frontend tests, the full Go race suite, vet and recovery smoke pass. This is a release candidate pending the remaining acceptance/security gates; it does not qualify the older Windows or SQLCipher artifacts.


An original offline desktop business application built with Go, React, TypeScript, Tailwind and embedded SQLite. The current target is a Linux release candidate for common daily work. Specialized features in the original specification are deferred; production acceptance gates are recorded explicitly.

Working features include multiple businesses; products, parties and warehouses; GST and non-GST sales/purchases; partial returns; POS with held bills and split payments; commercial documents; partial purchase-order goods receipts and invoice matching; inventory; accounting and bank reconciliation; financial reports and CSV/XLSX/PDF exports; master imports; verified backup/restore and automatic backup retention; local users with business assignments; offline administrator recovery; rotating logs; alternate units; business logos/product images; and product-sales/HSN summaries. All business data comes from SQLite. No demo transactions are inserted.

The full specification remains incomplete. See [application readiness](docs/READINESS.md) for the current completion assessment. See [implementation status](docs/IMPLEMENTATION_STATUS.md) for the remaining work and [test results](docs/TEST_REPORT.md) for verification limits.

The latest source additionally includes financial-year/variant settings, recurring expenses, year closing, quantity cess, encrypted backup copies, independent dispatches, journal imports and offline restore assistance. Consolidated Linux verification is recorded in the status and test report above. Earlier consolidated verification used `release/ledgerleaf-linux`; the latest source uses the development artifact described above.

For the latest Linux trial, use the development executable:

```sh
./release/ledgerleaf-linux-dev --data-dir ./manual-test-data
```

It opens your browser and keeps test records in a separate installation. No build tools or internet connection are needed to run it. Keep its terminal open while using the application. The standard Linux alias now matches RC3; encrypted Linux and Windows artifacts predate the latest changes and remain separately unqualified. Build-from-source instructions are in [building.md](docs/building.md).

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

Linux candidate packaging uses Go **1.27.3**, Node/npm, GCC and Chrome for browser tests. Legacy Windows builds use Go **1.20.14** and MinGW-w64 x86/x64. Dependencies are pinned and must be installed beforehand; candidate packaging downloads nothing and the resulting application needs no internet connection.

```sh
GO_BIN=/path/to/go1.20.14/bin/go GOCACHE=/tmp/billbook-go-cache ./scripts/build-windows.sh
```

The Linux script builds the UI, tests Go and frontend code, runs a browser workflow, compiles both Windows binaries, inspects their imports, and generates hashes and a manifest. See [building](docs/building.md) and [usage](docs/usage.md).

Documentation: [architecture](docs/architecture.md), [database](docs/database.md), [accounting](docs/accounting.md), [GST](docs/gst.md).

## Application tests

See [test results and verification limits](docs/TEST_REPORT.md). Run the complete Linux verification suite with installed dependencies:

```sh
GO_BIN=/snap/go/current/bin/go GOCACHE=/tmp/ledgerleaf-review-go-cache ./scripts/test-application.sh
```

Tests use disposable SQLite databases, exercise the real executable and browser, and check financial rollback, concurrent writes, PDF content and crash/restart persistence.

The application includes offline **Help** with workflow guides in English, Hindi and Gujarati. Choose **Language / भाषा / ભાષા** in the sidebar or login screen; English is the default. Navigation and common labels are localized, with English fallback for remaining messages. [Localization scope](docs/LOCALIZATION.md).

Administrators can download an all-business, all-table Excel workbook from **Backup & restore**. The workbook is an unencrypted inspection export; use SQLite backups for complete restores. **Import & export** provides blank product/customer/supplier Excel templates and validated invoice/journal imports. See [the testing guide](docs/TESTING_GUIDE.md#entire-database-workbook-and-excel-import-templates) for formats and units.

## Source control

The local repository uses `main`. Initial commit `7aa7c24` captures the existing source, documentation, tests, migrations and embedded web assets before further changes. Local executables, release artifacts, installed dependencies, Python caches, database files and `.env` files are excluded.

Simplicity and familiar billing/accounting workflows are core product requirements. See [interface principles](docs/UX_PRINCIPLES.md) for the conventions guiding future features.

Daily entry supports **Save & review → Edit draft or Finalize/Issue → Download/print PDF** in the same document dialog. **Orders & quotations** includes direct purchase-order entry. Running item amounts use entered prices; review uses the saved backend calculation. See [the daily-entry walkthrough](docs/TESTING_GUIDE.md#daily-invoice-and-order-entry).

**Download PDF** is the primary document export. **Export image (PNG)** is available alongside it for finalized invoices, issued orders/quotations, credit notes, dispatch challans and POS A4 invoices. Long documents provide a separate PNG download for each page.
