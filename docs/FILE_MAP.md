# Project file map

Updated 2026-10-19. Start with [TESTING_GUIDE.md](TESTING_GUIDE.md) to run and test the application. This inventory describes project-owned source, tests, configuration, documentation and assets; installed dependencies and generated build files are grouped separately below. `.env` files are neither required nor read for this guide.

## How the pieces connect

`cmd/billbook/main.go` opens the installation → `internal/database` applies migrations → `internal/server` handles local browser requests → accounting/stock/tax helpers perform transactional work → `frontend/src` presents the result. Vite compiles the frontend into `internal/web/dist`; Go embeds that directory in the executable. PDF rendering and CSV/XLSX processing are local libraries, not external services.

## Root and entry point

| File | Purpose |
| --- | --- |
| [.gitignore](../.gitignore) | Patterns for local generated files and data that should not be tracked. |
| [README.md](../README.md) | Project introduction, run/build entry points and documentation links. |
| [cmd/billbook/main.go](../cmd/billbook/main.go) | Application entry point: flags, data folders, lock, logging, recovery/encryption CLI, database opening, local HTTP server, browser launch and shutdown. |
| [go.mod](../go.mod) | Go module identity, required toolchain language version and pinned direct dependencies. |
| [go.sum](../go.sum) | Checksums for Go dependency modules. |
| [ledgerleaf](../ledgerleaf) | Older generated root executable; use release/ledgerleaf-linux for the current build. |

## Documentation

| File | Purpose |
| --- | --- |
| [docs/FILE_MAP.md](../docs/FILE_MAP.md) | This source/configuration/documentation file inventory. |
| [docs/IMPLEMENTATION_STATUS.md](../docs/IMPLEMENTATION_STATUS.md) | Implemented features, pending work and known limitations. |
| [docs/TESTING_GUIDE.md](../docs/TESTING_GUIDE.md) | User walkthrough, startup/rebuild commands, feature checks, encryption and troubleshooting. |
| [docs/TEST_REPORT.md](../docs/TEST_REPORT.md) | Executed checks, initial failures/rechecks, evidence and deferred verification. |
| [docs/accounting.md](../docs/accounting.md) | Posting model, ledger accounts and accounting behavior. |
| [docs/architecture.md](../docs/architecture.md) | Application layers, local security boundary and design choices. |
| [docs/building.md](../docs/building.md) | Build prerequisites, commands, artifacts and verification constraints. |
| [docs/compatibility.md](../docs/compatibility.md) | Windows/browser targets and compatibility verification limits. |
| [docs/database.md](../docs/database.md) | Schema/storage model and database invariants. |
| [docs/gst.md](../docs/gst.md) | Configured tax behavior, preparation reports, limitations and consulted references. |
| [docs/usage.md](../docs/usage.md) | Operational workflow reference and recovery commands. |

## Frontend

| File | Purpose |
| --- | --- |
| [frontend/index.html](../frontend/index.html) | HTML shell loaded by the embedded web application. |
| [frontend/package-lock.json](../frontend/package-lock.json) | Exact npm dependency graph and integrity hashes. |
| [frontend/package.json](../frontend/package.json) | Frontend dependency versions and build/unit-test/browser-test commands. |
| [frontend/playwright.config.ts](../frontend/playwright.config.ts) | Browser-test runner configuration, browser executable and result locations. |
| [frontend/postcss.config.cjs](../frontend/postcss.config.cjs) | CSS processing configuration. |
| [frontend/public/third-party-notices.txt](../frontend/public/third-party-notices.txt) | Third-party notices copied into the frontend build. |
| [frontend/src/accounting.tsx](../frontend/src/accounting.tsx) | Chart of accounts, balanced journal posting and account/period validation UI. |
| [frontend/src/api.ts](../frontend/src/api.ts) | Shared browser API request/error handling implementation. |
| [frontend/src/backup_encryption.tsx](../frontend/src/backup_encryption.tsx) | Password-protected backup download/upload endpoints or controls UI. |
| [frontend/src/backup_policy.tsx](../frontend/src/backup_policy.tsx) | Automatic-backup policy controls UI. |
| [frontend/src/backups.tsx](../frontend/src/backups.tsx) | Snapshot listing, download, import, verification and restore UI. |
| [frontend/src/banks.tsx](../frontend/src/banks.tsx) | Bank accounts, period locks and statement reconciliation controls UI. |
| [frontend/src/billing.tsx](../frontend/src/billing.tsx) | Invoice drafts, shared validation, finalization, stock/accounting posting and payments UI. |
| [frontend/src/business_picker.tsx](../frontend/src/business_picker.tsx) | Searchable, paginated business switching with preserved selection and server-enforced membership filtering. |
| [frontend/src/catalog_settings.tsx](../frontend/src/catalog_settings.tsx) | Financial-year settings, product groups and structured variants UI. |
| [frontend/src/credit_notes.tsx](../frontend/src/credit_notes.tsx) | Credit-note listing and PDF links UI. |
| [frontend/src/dashboard.tsx](../frontend/src/dashboard.tsx) | Business overview balances and activity summaries UI. |
| [frontend/src/diagnostics.tsx](../frontend/src/diagnostics.tsx) | Read-only integrity and accounting-reconciliation diagnostics UI. |
| [frontend/src/dispatch_consolidation.tsx](../frontend/src/dispatch_consolidation.tsx) | Choose quantities from several dispatches and create one billing draft UI. |
| [frontend/src/dispatch_selection.tsx](../frontend/src/dispatch_selection.tsx) | Partial dispatch invoice/return selection and allocation history UI. |
| [frontend/src/dispatches.tsx](../frontend/src/dispatches.tsx) | Independent dispatch challans, stock issue and warehouse returns UI. |
| [frontend/src/expense_management.tsx](../frontend/src/expense_management.tsx) | Recurring expenses, categories and expense attachments UI. |
| [frontend/src/import_documents.tsx](../frontend/src/import_documents.tsx) | CSV/XLSX invoice grouping, validation, atomic preview and draft/finalized import UI. |
| [frontend/src/import_journals.tsx](../frontend/src/import_journals.tsx) | CSV/XLSX journal grouping, validation, preview and atomic posting UI. |
| [frontend/src/imports.tsx](../frontend/src/imports.tsx) | Product/party import and import-page integration UI. |
| [frontend/src/main.tsx](../frontend/src/main.tsx) | React entry point, session bootstrap, business selection, app navigation and page composition. |
| [frontend/src/master_picker.tsx](../frontend/src/master_picker.tsx) | Shared paginated product, party and warehouse selector that retains selected records UI. |
| [frontend/src/masters.tsx](../frontend/src/masters.tsx) | Business/party/product/warehouse CRUD and inventory lookup or opening stock UI. |
| [frontend/src/media.tsx](../frontend/src/media.tsx) | Logo/product-image upload, validation, storage and retrieval UI. |
| [frontend/src/numbers.ts](../frontend/src/numbers.ts) | Exact decimal-to-integer conversion and displayed numeric formatting implementation. |
| [frontend/src/operations.tsx](../frontend/src/operations.tsx) | Stock additions/removals/damage/transfers and paid expenses UI. |
| [frontend/src/party_finance.tsx](../frontend/src/party_finance.tsx) | Opening party balances, advances, allocations and unused-advance refunds UI. |
| [frontend/src/permissions.ts](../frontend/src/permissions.ts) | Frontend role context and navigation/action visibility implementation. |
| [frontend/src/pos.tsx](../frontend/src/pos.tsx) | Point-of-sale cart, barcode lookup, held bills, checkout and receipts UI. |
| [frontend/src/pos_payments.ts](../frontend/src/pos_payments.ts) | POS payment-allocation, tender and change calculations implementation. |
| [frontend/src/prices.tsx](../frontend/src/prices.tsx) | Named product price lists UI. |
| [frontend/src/receipts.tsx](../frontend/src/receipts.tsx) | Purchase-order/direct goods receipts and receipt invoicing UI. |
| [frontend/src/reports.tsx](../frontend/src/reports.tsx) | Date-filtered financial/tax/inventory reports and CSV/XLSX/PDF exports UI. |
| [frontend/src/style.css](../frontend/src/style.css) | Shared layout, typography, colors, responsive rules and component styling. |
| [frontend/src/taxes.tsx](../frontend/src/taxes.tsx) | Effective-dated product HSN/SAC, configured GST and cess UI. |
| [frontend/src/units.tsx](../frontend/src/units.tsx) | Alternate-unit definitions, conversions and frozen base quantities UI. |
| [frontend/src/users.tsx](../frontend/src/users.tsx) | User setup, permissions, business assignments and password UI UI. |
| [frontend/src/year_closing.tsx](../frontend/src/year_closing.tsx) | Preview and post year-end income/expense closing to retained earnings UI. |
| [frontend/tailwind.config.cjs](../frontend/tailwind.config.cjs) | Tailwind content discovery and theme configuration. |
| [frontend/tests/numbers.test.mjs](../frontend/tests/numbers.test.mjs) | Regression tests for Exact decimal-to-integer conversion and displayed numeric formatting. |
| [frontend/tests/pos_payments.test.mjs](../frontend/tests/pos_payments.test.mjs) | Regression tests for POS payment-allocation, tender and change calculations. |
| [frontend/tsconfig.json](../frontend/tsconfig.json) | TypeScript compiler/type-checking settings. |
| [frontend/vite.config.ts](../frontend/vite.config.ts) | Frontend build target and output into internal/web/dist for Go embedding. |

## Backend modules and tests

| File | Purpose |
| --- | --- |
| [internal/accounting/accounting.go](../internal/accounting/accounting.go) | Chart of accounts, balanced journal posting and account/period validation implementation. |
| [internal/accounting/accounting_test.go](../internal/accounting/accounting_test.go) | Regression tests for Chart of accounts, balanced journal posting and account/period validation. |
| [internal/backup/backup.go](../internal/backup/backup.go) | Standalone snapshot creation, schema/integrity verification and safe import staging. |
| [internal/backup/encryption.go](../internal/backup/encryption.go) | Authenticated password-encrypted backup stream format and decryption. |
| [internal/backup/encryption_test.go](../internal/backup/encryption_test.go) | Regression tests for encryption. |
| [internal/database/database.go](../internal/database/database.go) | SQLite/SQLCipher connection settings and ordered transactional migration runner. |
| [internal/database/database_test.go](../internal/database/database_test.go) | Regression tests for database. |
| [internal/database/dispatch_upgrade_test.go](../internal/database/dispatch_upgrade_test.go) | Regression tests for dispatch upgrade. |
| [internal/database/encryption.go](../internal/database/encryption.go) | Encryption capability/password checks and transactional keyed/plaintext database export. |
| [internal/database/encryption_test.go](../internal/database/encryption_test.go) | Regression tests for encryption. |
| [internal/database/receipt_upgrade_test.go](../internal/database/receipt_upgrade_test.go) | Regression tests for receipt upgrade. |
| [internal/database/recovery_test.go](../internal/database/recovery_test.go) | Regression tests for recovery. |
| [internal/export/read.go](../internal/export/read.go) | Bounded CSV/XLSX table reader and formula/structure validation. |
| [internal/export/xlsx.go](../internal/export/xlsx.go) | Streaming spreadsheet writer and cell serialization. |
| [internal/export/xlsx_test.go](../internal/export/xlsx_test.go) | Regression tests for xlsx. |
| [internal/gst/gst.go](../internal/gst/gst.go) | GSTIN/PAN/state/HSN validation and configured tax arithmetic. |
| [internal/gst/gst_test.go](../internal/gst/gst_test.go) | Regression tests for gst. |
| [internal/instance/lock_test.go](../internal/instance/lock_test.go) | Regression tests for lock. |
| [internal/instance/lock_unix.go](../internal/instance/lock_unix.go) | Unix process-held exclusive installation lock. |
| [internal/instance/lock_windows.go](../internal/instance/lock_windows.go) | Windows process-held exclusive installation lock. |
| [internal/logging/rotate.go](../internal/logging/rotate.go) | Size-based runtime-log rotation. |
| [internal/logging/rotate_test.go](../internal/logging/rotate_test.go) | Regression tests for rotate. |
| [internal/money/money.go](../internal/money/money.go) | Checked integer arithmetic and deterministic fixed-point rounding. |
| [internal/money/money_test.go](../internal/money/money_test.go) | Regression tests for money. |
| [internal/printing/DejaVuSans.ttf](../internal/printing/DejaVuSans.ttf) | Bundled font used in generated PDFs; binary asset. |
| [internal/printing/FONT_LICENSE](../internal/printing/FONT_LICENSE) | License for the bundled PDF font. |
| [internal/printing/pdf.go](../internal/printing/pdf.go) | A4 invoice/commercial-document rendering and embedded font setup. |
| [internal/printing/pdf_test.go](../internal/printing/pdf_test.go) | Regression tests for pdf. |
| [internal/printing/report.go](../internal/printing/report.go) | Tabular report PDF rendering. |
| [internal/printing/thermal.go](../internal/printing/thermal.go) | 58/80 mm POS receipt rendering. |
| [internal/printing/thermal_test.go](../internal/printing/thermal_test.go) | Regression tests for thermal. |
| [internal/recovery/admin.go](../internal/recovery/admin.go) | Offline existing-administrator password reset with preserved backup/audit. |
| [internal/recovery/admin_test.go](../internal/recovery/admin_test.go) | Regression tests for admin. |
| [internal/recovery/encryption.go](../internal/recovery/encryption.go) | Offline plaintext-to-SQLCipher conversion and encryption of online restore staging files. |
| [internal/recovery/encryption_test.go](../internal/recovery/encryption_test.go) | Regression tests for encryption. |
| [internal/recovery/restore.go](../internal/recovery/restore.go) | Offline plaintext/encrypted-backup restore while retaining original DB/WAL/SHM files. |
| [internal/recovery/restore_test.go](../internal/recovery/restore_test.go) | Regression tests for restore. |
| [internal/server/auth.go](../internal/server/auth.go) | Authentication, role enforcement, users, sessions and login throttling implementation. |
| [internal/server/auth_test.go](../internal/server/auth_test.go) | Regression tests for Authentication, role enforcement, users, sessions and login throttling. |
| [internal/server/backup_encryption.go](../internal/server/backup_encryption.go) | Password-protected backup download/upload endpoints or controls implementation. |
| [internal/server/backup_schedule.go](../internal/server/backup_schedule.go) | Scheduled backup execution and retention implementation. |
| [internal/server/backup_schedule_test.go](../internal/server/backup_schedule_test.go) | Regression tests for Scheduled backup execution and retention. |
| [internal/server/backups.go](../internal/server/backups.go) | Snapshot listing, download, import, verification and restore implementation. |
| [internal/server/backups_test.go](../internal/server/backups_test.go) | Regression tests for Snapshot listing, download, import, verification and restore. |
| [internal/server/billing.go](../internal/server/billing.go) | Invoice drafts, shared validation, finalization, stock/accounting posting and payments implementation. |
| [internal/server/billing_tax.go](../internal/server/billing_tax.go) | Invoice tax preparation, configured tax calculation and tax journal components implementation. |
| [internal/server/billing_test.go](../internal/server/billing_test.go) | Regression tests for Invoice drafts, shared validation, finalization, stock/accounting posting and payments. |
| [internal/server/catalog_settings.go](../internal/server/catalog_settings.go) | Financial-year settings, product groups and structured variants implementation. |
| [internal/server/commercial.go](../internal/server/commercial.go) | Estimates, quotations, proformas, purchase orders and invoice conversion implementation. |
| [internal/server/commercial_test.go](../internal/server/commercial_test.go) | Regression tests for Estimates, quotations, proformas, purchase orders and invoice conversion. |
| [internal/server/controls.go](../internal/server/controls.go) | Posting-period locks, bank accounts and reconciliation API implementation. |
| [internal/server/controls_test.go](../internal/server/controls_test.go) | Regression tests for Posting-period locks, bank accounts and reconciliation API. |
| [internal/server/dashboard.go](../internal/server/dashboard.go) | Business overview balances and activity summaries implementation. |
| [internal/server/diagnostics.go](../internal/server/diagnostics.go) | Read-only integrity and accounting-reconciliation diagnostics implementation. |
| [internal/server/diagnostics_test.go](../internal/server/diagnostics_test.go) | Regression tests for Read-only integrity and accounting-reconciliation diagnostics. |
| [internal/server/dispatch_allocations.go](../internal/server/dispatch_allocations.go) | Remaining dispatch balances, cumulative cost allocation and multi-dispatch matching implementation. |
| [internal/server/dispatch_allocations_test.go](../internal/server/dispatch_allocations_test.go) | Regression tests for Remaining dispatch balances, cumulative cost allocation and multi-dispatch matching. |
| [internal/server/dispatches.go](../internal/server/dispatches.go) | Independent dispatch challans, stock issue and warehouse returns implementation. |
| [internal/server/expense_management.go](../internal/server/expense_management.go) | Recurring expenses, categories and expense attachments implementation. |
| [internal/server/gst_preparation.go](../internal/server/gst_preparation.go) | Frozen-invoice/credit-note queries for GSTR-1 and GSTR-3B preparation worksheets implementation. |
| [internal/server/gst_preparation_test.go](../internal/server/gst_preparation_test.go) | Regression tests for Frozen-invoice/credit-note queries for GSTR-1 and GSTR-3B preparation worksheets. |
| [internal/server/import_documents.go](../internal/server/import_documents.go) | CSV/XLSX invoice grouping, validation, atomic preview and draft/finalized import implementation. |
| [internal/server/import_documents_test.go](../internal/server/import_documents_test.go) | Regression tests for CSV/XLSX invoice grouping, validation, atomic preview and draft/finalized import. |
| [internal/server/import_journals.go](../internal/server/import_journals.go) | CSV/XLSX journal grouping, validation, preview and atomic posting implementation. |
| [internal/server/import_journals_validation_test.go](../internal/server/import_journals_validation_test.go) | Regression tests for File-level journal validation edge cases. |
| [internal/server/imports.go](../internal/server/imports.go) | Product/party import and import-page integration implementation. |
| [internal/server/imports_test.go](../internal/server/imports_test.go) | Regression tests for Product/party import and import-page integration. |
| [internal/server/integrity_test.go](../internal/server/integrity_test.go) | Regression tests for integrity. |
| [internal/server/invoice_adjustments.go](../internal/server/invoice_adjustments.go) | Allocate invoice-level discounts/charges and calculate rupee rounding implementation. |
| [internal/server/invoice_adjustments_test.go](../internal/server/invoice_adjustments_test.go) | Regression tests for Allocate invoice-level discounts/charges and calculate rupee rounding. |
| [internal/server/journal.go](../internal/server/journal.go) | Manual journal entry and related posting validation implementation. |
| [internal/server/journal_profile_test.go](../internal/server/journal_profile_test.go) | Regression tests for journal profile. |
| [internal/server/master_picker_test.go](../internal/server/master_picker_test.go) | Regression tests for Shared paginated product, party and warehouse selector that retains selected records. |
| [internal/server/masters.go](../internal/server/masters.go) | Business/party/product/warehouse CRUD and inventory lookup or opening stock implementation. |
| [internal/server/masters_test.go](../internal/server/masters_test.go) | Regression tests for Business/party/product/warehouse CRUD and inventory lookup or opening stock. |
| [internal/server/media.go](../internal/server/media.go) | Logo/product-image upload, validation, storage and retrieval implementation. |
| [internal/server/media_test.go](../internal/server/media_test.go) | Regression tests for Logo/product-image upload, validation, storage and retrieval. |
| [internal/server/memberships_test.go](../internal/server/memberships_test.go) | Regression tests for memberships. |
| [internal/server/operations.go](../internal/server/operations.go) | Stock additions/removals/damage/transfers and paid expenses implementation. |
| [internal/server/operations_test.go](../internal/server/operations_test.go) | Regression tests for Stock additions/removals/damage/transfers and paid expenses. |
| [internal/server/party_finance.go](../internal/server/party_finance.go) | Opening party balances, advances, allocations and unused-advance refunds implementation. |
| [internal/server/party_finance_test.go](../internal/server/party_finance_test.go) | Regression tests for Opening party balances, advances, allocations and unused-advance refunds. |
| [internal/server/pending_batch_test.go](../internal/server/pending_batch_test.go) | Regression tests for pending batch. |
| [internal/server/prices.go](../internal/server/prices.go) | Named product price lists implementation. |
| [internal/server/printing.go](../internal/server/printing.go) | HTTP PDF generation for invoices, commercial documents and thermal receipts implementation. |
| [internal/server/receipt_matching.go](../internal/server/receipt_matching.go) | Quantity-based receipt-to-invoice matching and cumulative valuation implementation. |
| [internal/server/receipt_phase_test.go](../internal/server/receipt_phase_test.go) | Regression tests for receipt phase. |
| [internal/server/receipt_reversal.go](../internal/server/receipt_reversal.go) | Reverse wholly uninvoiced receipts while preserving stock/valuation chronology implementation. |
| [internal/server/receipts.go](../internal/server/receipts.go) | Purchase-order/direct goods receipts and receipt invoicing implementation. |
| [internal/server/receipts_test.go](../internal/server/receipts_test.go) | Regression tests for Purchase-order/direct goods receipts and receipt invoicing. |
| [internal/server/report_analytics.go](../internal/server/report_analytics.go) | Customer/monthly/product sales and HSN summary query helpers implementation. |
| [internal/server/report_analytics_test.go](../internal/server/report_analytics_test.go) | Regression tests for Customer/monthly/product sales and HSN summary query helpers. |
| [internal/server/reports.go](../internal/server/reports.go) | Date-filtered financial/tax/inventory reports and CSV/XLSX/PDF exports implementation. |
| [internal/server/reversals.go](../internal/server/reversals.go) | Immutable invoice credit notes, returns, refunds and stock/accounting reversals implementation. |
| [internal/server/reversals_test.go](../internal/server/reversals_test.go) | Regression tests for Immutable invoice credit notes, returns, refunds and stock/accounting reversals. |
| [internal/server/security_validation_test.go](../internal/server/security_validation_test.go) | Regression tests for security validation. |
| [internal/server/server.go](../internal/server/server.go) | HTTP routing, request guards, application state and shared response helpers implementation. |
| [internal/server/server_test.go](../internal/server/server_test.go) | Regression tests for HTTP routing, request guards, application state and shared response helpers. |
| [internal/server/split_payments.go](../internal/server/split_payments.go) | Atomic POS split-payment validation and posting implementation. |
| [internal/server/split_payments_test.go](../internal/server/split_payments_test.go) | Regression tests for Atomic POS split-payment validation and posting. |
| [internal/server/taxes.go](../internal/server/taxes.go) | Effective-dated product HSN/SAC, configured GST and cess implementation. |
| [internal/server/taxes_test.go](../internal/server/taxes_test.go) | Regression tests for Effective-dated product HSN/SAC, configured GST and cess. |
| [internal/server/units.go](../internal/server/units.go) | Alternate-unit definitions, conversions and frozen base quantities implementation. |
| [internal/server/units_test.go](../internal/server/units_test.go) | Regression tests for Alternate-unit definitions, conversions and frozen base quantities. |
| [internal/server/working_encryption_test.go](../internal/server/working_encryption_test.go) | Regression tests for working encryption. |
| [internal/server/year_closing.go](../internal/server/year_closing.go) | Preview and post year-end income/expense closing to retained earnings implementation. |
| [internal/web/web.go](../internal/web/web.go) | Embeds compiled frontend files into the Go executable. |

## Database migrations

| File | Purpose |
| --- | --- |
| [internal/database/migrations/001_foundation.sql](../internal/database/migrations/001_foundation.sql) | Migration 001: business foundation and schema metadata. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/002_masters.sql](../internal/database/migrations/002_masters.sql) | Migration 002: parties, products, warehouses and stock. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/003_immutable_ledgers.sql](../internal/database/migrations/003_immutable_ledgers.sql) | Migration 003: immutable accounting/stock ledger constraints. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/004_billing.sql](../internal/database/migrations/004_billing.sql) | Migration 004: draft/finalized invoices, payments and numbering. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/005_corrections.sql](../internal/database/migrations/005_corrections.sql) | Migration 005: corrections and reversals. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/006_users.sql](../internal/database/migrations/006_users.sql) | Migration 006: users, authentication and actor audit. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/007_business_profile.sql](../internal/database/migrations/007_business_profile.sql) | Migration 007: expanded business profile and invoice details. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/008_tax.sql](../internal/database/migrations/008_tax.sql) | Migration 008: GST settings and effective-dated taxes. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/009_credit_notes.sql](../internal/database/migrations/009_credit_notes.sql) | Migration 009: partial credit notes and settlement view. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/010_commercial_documents.sql](../internal/database/migrations/010_commercial_documents.sql) | Migration 010: commercial documents and conversion links. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/011_party_finance.sql](../internal/database/migrations/011_party_finance.sql) | Migration 011: party openings, advances and allocation records. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/012_controls.sql](../internal/database/migrations/012_controls.sql) | Migration 012: period locks, bank accounts and reconciliations. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/013_catalog.sql](../internal/database/migrations/013_catalog.sql) | Migration 013: catalog pricing and party credit terms. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/014_receipts.sql](../internal/database/migrations/014_receipts.sql) | Migration 014: purchase-order goods receipts. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/015_backup_policy.sql](../internal/database/migrations/015_backup_policy.sql) | Migration 015: automatic-backup policy. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/016_memberships.sql](../internal/database/migrations/016_memberships.sql) | Migration 016: business-specific user assignments. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/017_product_units.sql](../internal/database/migrations/017_product_units.sql) | Migration 017: alternate product units. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/018_media.sql](../internal/database/migrations/018_media.sql) | Migration 018: immutable stored media and image links. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/019_invoice_adjustments.sql](../internal/database/migrations/019_invoice_adjustments.sql) | Migration 019: invoice adjustments/rounding account. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/020_receipt_matching.sql](../internal/database/migrations/020_receipt_matching.sql) | Migration 020: partial receipt matching and reversal records. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/021_catalog_settings.sql](../internal/database/migrations/021_catalog_settings.sql) | Migration 021: financial-year and structured variant settings. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/022_expense_management.sql](../internal/database/migrations/022_expense_management.sql) | Migration 022: recurring expenses, categories and attachments. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/023_year_closing.sql](../internal/database/migrations/023_year_closing.sql) | Migration 023: year-end closing records. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/024_quantity_cess.sql](../internal/database/migrations/024_quantity_cess.sql) | Migration 024: fixed quantity-based cess. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/025_dispatches.sql](../internal/database/migrations/025_dispatches.sql) | Migration 025: independent dispatch challans. Applied once, in order; do not edit an already-deployed migration. |
| [internal/database/migrations/026_dispatch_allocations.sql](../internal/database/migrations/026_dispatch_allocations.sql) | Migration 026: immutable per-product dispatch allocations and historical-link migration. Applied once, in order; do not edit an already-deployed migration. |

## Build and test scripts

| File | Purpose |
| --- | --- |
| [scripts/benchmark.py](../scripts/benchmark.py) | Disposable large-dataset application benchmark harness. |
| [scripts/build-linux-encrypted.sh](../scripts/build-linux-encrypted.sh) | Download/check/build pinned SQLCipher and compile the native Linux encrypted executable. |
| [scripts/build-windows.ps1](../scripts/build-windows.ps1) | Windows-host build counterpart; host/runtime verification remains deferred. |
| [scripts/build-windows.sh](../scripts/build-windows.sh) | Linux-hosted standard-build checks and Windows x86/x64 cross-compilation. |
| [scripts/encryption-smoke.py](../scripts/encryption-smoke.py) | Disposable Linux CLI verification of encrypted startup, conversion, restart and wrong-password handling. |
| [scripts/manifest.mjs](../scripts/manifest.mjs) | Generate build artifact metadata/manifests. |
| [scripts/smoke.py](../scripts/smoke.py) | Compiled-app smoke, persistence, crash/recovery and encrypted-backup restore checks. |
| [scripts/test-application.sh](../scripts/test-application.sh) | Full standard Linux frontend/backend/browser/smoke verification pipeline. |
| [scripts/verify-build.ps1](../scripts/verify-build.ps1) | Windows artifact hash/import inspection helper. |

## Browser tests and fixtures

| File | Purpose |
| --- | --- |
| [tests/e2e/regressions.spec.ts](../tests/e2e/regressions.spec.ts) | Browser regression workflows for billing, permissions, imports, inventory, GST, backup and newer features. |
| [tests/e2e/workflow.spec.ts](../tests/e2e/workflow.spec.ts) | Core browser workflow: create masters, receive stock, invoice and collect payment. |
| [tests/fixtures/logo.png](../tests/fixtures/logo.png) | Small image fixture for upload and immutable-logo tests. |

## Generated files and installed dependencies

| Location | Purpose |
| --- | --- |
| `release/ledgerleaf-linux` | Current standard Linux application; embedded SQLite and web assets. |
| `release/ledgerleaf-linux-encrypted` | Native Linux SQLCipher variant; requires OpenSSL/zlib runtime libraries. |
| `release/SQLCipher-LICENSE.md` | License shipped with the encryption-capable build. |
| `release/BillBook-Windows-*.exe` | Earlier cross-compiled Windows artifacts; not refreshed/tested for the latest work. |
| `release/verification-*/` | Saved verification logs; each batch records which checks actually ran. |
| `release/test-results/`, coverage files, benchmark JSON, hashes/manifests/import reports | Generated test evidence and build metadata, not application source. |
| `internal/web/dist/index.html` | Generated browser HTML entry point. |
| `internal/web/dist/assets/*.js`, `*.css` | Hashed compiled frontend bundles; regenerate from frontend source. |
| `internal/web/dist/third-party-notices.txt` | Copied frontend license notices. |
| `frontend/node_modules/` | Installed pinned JavaScript dependencies; regenerate with npm from the lockfile. |
| `scripts/__pycache__/` | Python bytecode cache generated by running development scripts. |
| Go module/build cache and temporary SQLCipher source/build directory | Installed/downloaded development dependencies and compiler cache outside application data. |
| Test-data directory chosen with `--data-dir` | Runtime records; see the testing guide. It is not source code and should not be committed. |

Tests with `fixture`, `setupTrading`, or `gstTrading` helpers create disposable installations. They are not runtime data-seeding paths. Regular startup never inserts demo businesses or transactions.

## Daily-use batch additions

| File | Purpose |
| --- | --- |
| [frontend/src/payment_history.tsx](../frontend/src/payment_history.tsx) | Paginated direct-payment history within a finalized bill, with amount, date, mode and account code. |
| [internal/server/document_lookup_test.go](../internal/server/document_lookup_test.go) | Verifies outstanding-bill filtering, settled-bill exclusion and document/business isolation of payment history. |

| File | Purpose |
| --- | --- |
| [frontend/src/modal.tsx](../frontend/src/modal.tsx) | Shared native dialog with focus restoration, Escape handling, background scroll lock and an unsaved-change confirmation on dismissal. |
| [frontend/src/business_directory.tsx](../frontend/src/business_directory.tsx) | Searchable, paginated business list with explicit add, edit and open actions. |

| File | Purpose |
| --- | --- |
| [frontend/src/action_dialog.tsx](../frontend/src/action_dialog.tsx) | Reusable configuration action button and dialog. Successful saves close the form via a revision key; validation errors preserve input. |

| File | Purpose |
| --- | --- |
| [frontend/src/i18n.ts](../frontend/src/i18n.ts) | English fallback, Hindi/Gujarati UI dictionary, browser language preference and language-change subscription. |
| [frontend/src/help.tsx](../frontend/src/help.tsx) | Language selector, searchable offline help UI, contextual topic choice and permission-aware page shortcuts. |
| [frontend/src/help_content.ts](../frontend/src/help_content.ts) | Eleven step-by-step help topics in English, Hindi and Gujarati. |
| [docs/LOCALIZATION.md](LOCALIZATION.md) | Translation scope, language behavior and instructions for maintaining safe UI translations. |

### Database-wide Excel export

- `internal/server/database_excel.go`: administrator-only all-table workbook export from a single read transaction; secure temporary spooling of large/binary values; blank product/party import templates.
- `internal/server/database_excel_test.go`: all-table coverage, cross-business data, binary reconstruction, formula-safe output, template headers and access-control regression checks.
- `internal/export/xlsx.go`: shared streaming single/multiple-sheet OOXML writer, worksheet-name validation and Excel cell/row/column limits.

- `frontend/src/business_assignments.tsx`: paginated business checkbox selection for restricted accounts; preserves selections across searches and exposes selected IDs for removal.

- `frontend/src/banks.tsx`: bank-account creation and statement-matching dialogs alongside reconciliation, period locks and year closing.
- `frontend/src/i18n.ts`: language dictionary plus React external-store subscription, including same-tab and cross-tab preference updates.

- `frontend/src/payment_account.tsx`: shared cash/named-bank selector for ordinary and recurring expenses, with loading/error handling.
- `internal/server/expense_bank_test.go`: named-bank expense and recurring-payment posting, duplicate prevention, cross-business/account rejection and ledger integrity regression.

- `frontend/src/receipts.tsx`: goods receipt list with creation, invoice-quantity matching and correction dialogs.
- `frontend/src/dispatches.tsx`: dispatch list with a creation dialog and responsive table wrapper; existing allocation/history/consolidation controls remain available.

- `frontend/src/main.tsx`: now gates workspace rendering until authentication/startup completes, exposing a loading state and retryable startup errors.

- `frontend/src/dispatch_selection.tsx`: dialog for allocation history, selected-quantity invoicing and returns; rejects negative selections.
- `frontend/src/dispatch_consolidation.tsx`: dialog for combining compatible dispatch quantities into one editable invoice draft.
