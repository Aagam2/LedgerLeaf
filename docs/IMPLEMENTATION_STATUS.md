# Implementation status

Updated 2026-10-22. **Incomplete development build; not production-ready.** This is a working, tested subset of the supplied specification. The full application has not been completed.

## Current remaining work — 2026-10-22

This summary supersedes older continuation notes below; dated sections retain implementation history.

- **Latest verified batch:** goods and dispatch dialogs are implemented, including quantity selection, history, corrections and consolidation. The broad 41-workflow run plus affected reruns is recorded below; the latest four dispatch/login/layout checks passed. Startup now waits for authentication before rendering workspace controls. Broader release verification remains pending.
- **Current runnable build:** `release/ledgerleaf-linux-dev` contains the 2026-10-22 changes and was tested with Go 1.27.2. The standard/encrypted release binaries are older; rebuilding with pinned Go 1.20.14 and SQLCipher remains pending because those temporary toolchains are unavailable.
- **Implementation gaps:** specialized GST/place-of-supply/valuation and advance-tax handling; additional accounting correction workflows; automatic startup repair; browser compatibility assistance.
- **Usability/localization:** receipt/dispatch creation, receipt matching/correction, dispatch allocation/history and consolidation now use dialogs. Remaining inline specialist forms, including ordinary expense/stock operations and period/year-close controls, have not all been converted. Full interface translation, advanced messages and PDF complex-script support are incomplete. New user-management dialogs and searchable business assignments are implemented; they are no longer pending features.
- **Final-stage verification (deferred by request):** Windows runtime and encryption packaging, physical printers, complete accessibility/security/compliance reviews, clean-machine/offline installation, 2 GB hardware benchmarks, migration interruption/power-loss checks, and support/release readiness.
- **Scope boundaries:** GSTR-1/GSTR-3B exports prepare data rather than file portal-ready returns. The all-table Excel workbook is an unencrypted inspection export, not a restorable SQLite backup. Each restricted account supports up to 200 assigned businesses, selected from the entire directory.

Use TEST_REPORT.md for exact verification results and TESTING_GUIDE.md for startup and manual workflows. The application remains an incomplete Linux trial build, not a completed production release.

## Implemented and connected to SQLite

- Go 1.20.14 application, embedded React/TypeScript/Tailwind UI and PDF font; static SQLite engine; Windows x86/x64 compilation.
- Loopback dynamic port, automatic browser launch, random launch-token authentication, HttpOnly session cookie, origin/Host checks, OS-held single-instance lock, portable data directory, log file, graceful Exit/Ctrl+C.
- Transactional migrations, foreign keys, WAL/FULL synchronization, future-schema rejection, business-scoped composite foreign keys, indexes and append-only ledger/audit triggers.
- Business creation/edit/list/switch, contact/PAN/bank details and invoice terms/footer; default chart of accounts and main warehouse. GSTIN checksum/PAN validation and unregistered, regular and composition registration modes.
- Customer/supplier create/edit/archive and paginated search. Product/service create/edit/archive, SKU/barcode search, unit, price and minimum stock.
- Warehouse creation/list; opening stock; stock balances/history; opening inventory/equity journals. Opening stock cannot be posted twice.
- Sale and purchase bill drafts, draft editing, immutable finalization, financial-year sequences, frozen document snapshots, item discounts, stock shortage validation and moving weighted-average costing.
- Purchase finalization receives goods and posts inventory/payable. Sale finalization posts receivable/revenue plus inventory/COGS. Transactions commit together or roll back.
- Partial/full manual payments and document allocation; overpayment rejection; idempotency; cash/bank accounting. Outstanding is derived from payments.
- Local A4 PDF generation with embedded font, downloadable for printing; frozen business and bank details. Returned-document status is shown in views and PDFs without mutating the original snapshot.
- Full sales/purchase returns and cancellations, paid-document refunds, stock restoration/removal and balanced reversal journals. Partial quantities issue immutable credit notes with cumulative rounding, refund limits, preview and PDF.
- Stock additions/removals/damage and atomic warehouse transfers with weighted-average valuation.
- Paid cash/bank expenses, custom chart accounts, balanced manual journals and journal/expense reversals. Direct manual writes to inventory, receivable and payable control accounts are prohibited.
- Date-filtered trial balance, profit and loss, balance sheet, general ledger, cash/bank books, sales/purchases, stock valuation and audit reports; paginated tables, CSV/XLSX/PDF exports, financial-year preset and financial summaries. Party statements and invoice ageing reconcile documents, receipts and credits. Overview shows actual balances and monthly net revenue.
- Verified standalone SQLite backups, download, import up to 512 MB, integrity/hash verification, restore across all businesses and a retained pre-restore recovery snapshot. Restore invalidates user sessions.
- Optional password authentication: first administrator setup, bcrypt password hashes, ADMIN/ACCOUNTANT/BILLER/VIEWER roles, password changes/resets, deactivation, session revocation, last-admin protection and actor audit. Biller price/discount overrides and purchase writes are rejected.
- Effective-dated HSN/SAC and configured GST/percentage cess, inclusive/exclusive pricing, intra/inter-state components, export/SEZ treatments, reverse-charge entries, explicit eligible input credit, composition bill-of-supply PDF and GST preparation rows. See docs/gst.md for scenario and legal limits.
- Estimates, quotations, proformas and purchase-order drafts, immutable issue/PDF and one-time conversion into billing drafts. Delivery challans accompany finalized invoices without reposting stock.
- POS barcode/SKU search, held/resumed SQLite drafts, keyboard shortcuts, checkout review, atomic split payments and 58/80 mm receipt PDFs.
- CSV/XLSX product/party import, 10,000 rows per upload, transactional rollback, duplicate protection, bounded ZIP parsing and formula rejection. Active master-data export uses matching headers.
- Party opening balances and settlement, customer/supplier advances, allocations to invoices and unused-advance refunds, with journal posting and remaining-balance checks.
- Light/dark UI, responsive layouts, validation/error feedback, empty/loading states, confirmations, table pagination and keyboard-accessible native controls. No mock dashboard values or hidden dummy data.

- Financial-period locks, named bank accounts, manual statement matching and unmatching; all postings respect locked dates.
- Product categories/brands/variant labels, purchase price/MRP, named price lists and party credit limits/periods.
- Partial goods receipts against issued purchase orders, received-quantity limits, stock/received-not-invoiced journals, supplier invoice matching and purchase-cost variance without receiving quantities twice.
- Automatic verified backups while the application is running, startup catch-up, configurable 1–720 hour interval and 1–100 automatic-copy retention. Only automatic snapshots are pruned after a successful new backup; manual/imported/pre-restore copies remain.
- Offline administrator recovery CLI requires an explicit existing data directory and the exclusive instance lock. Saves a verified pre-reset backup, generates a random password, audits recovery, and never promotes a non-administrator.
- Logs rotate at 5 MB with three retained files. Role-aware menus, read-only master/billing actions, biller sales controls and administrator-only period controls.
- Optional per-business user assignments, enforced on APIs and business selection. Existing users retain their installation-wide access on upgrade; administrators always retain all-business access. Assignment changes revoke that user's sessions.
- Duplicate invoice/commercial-document action opens a fresh editable draft with source/receipt links cleared. It does not post stock, payments or journals.

- Product-specific alternate units in Billing, POS and commercial documents. Invoices retain selected quantities/prices; stock uses exact base-unit quantities. Frozen factors also apply to partial returns and goods receipts; receipt matching compares base quantities. Base-unit prices and price lists scale to the selected unit. Up to 50 immutable conversions per product; quantities that cannot be represented exactly to three base-unit decimals are rejected.
- Business logos and product images (PNG/JPEG, up to 512 KB and 2048 × 2048). Images live in SQLite and are included in backup/restore. Logo replacements/removal preserve immutable historical assets; saved invoice, credit-note and commercial-document PDFs use their frozen logo references. Product thumbnails appear in the catalog.
- Product-sales report with net sales, original cost of sales and gross profit; HSN preparation summary grouped by document kind, base unit, rate and treatment. Both include date-filtered credit notes and normalize quantities using frozen conversion factors. CSV/XLSX/PDF exports are connected.

- Invoice-level discounts and allocated charges with proportional line allocation before tax; optional nearest-rupee rounding in account 5300. Mixed rates, inclusive prices, reverse charge, purchase valuation, partial returns/refunds, commercial documents, goods-receipt matching, POS preservation and A4/thermal PDFs are connected. Charges use the allocated items’ tax treatments; separately classified charges are outside this feature.
- Administrator-run integrity diagnostics check SQLite, foreign keys, balanced journals, nonnegative stock, inventory ledger reconciliation and snapshot totals. Counts-only JSON download and recovery guidance are available in Backup & restore. Checks do not change data or automatically repair it.

- Direct receipts without orders use estimated costs; quantity-based matching splits receipts across invoices or consolidates receipts from the same supplier/warehouse. Frozen base units and cumulative value allocation avoid double stock receipt and lost paise. Finalization rechecks availability, and stale competing drafts fail atomically. Historical whole-receipt links migrate to immutable allocations.
- Wholly uninvoiced receipts can be reversed with a reason, stock availability/chronology checks, period locks, weighted-average stock removal and a separate variance entry. Order quantities reopen after reversal. Invoiced receipts require purchase returns, and allocations remain historical.

## Latest source continuation (schema 25)

The following features are present in source. Earlier focused checks were reported in the preceding session; the additions described here have not yet received the deferred batch verification. Do not infer runtime certification from implementation status.

- Configurable financial-year starting month, locked after numbering/posting, and structured product groups with normalized variant attributes.
- Recurring expense schedules with explicit payment confirmation, categories, PDF/image attachments and category reports.
- Permanent year-end closing to retained earnings, with a new posting preview. Closing entries remain in the ledger and are excluded from profit-and-loss reporting.
- Configured fixed cess per base stock unit alongside percentage cess. Legal classification and specialized valuation remain outside the implementation.
- Authenticated encrypted backup exports/imports. The standard build and local snapshots remain plaintext; the optional SQLCipher continuation below adds working-database encryption.
- Independent full-dispatch challans, subsequent whole-dispatch invoicing without a second stock issue, and full uninvoiced-dispatch returns. Partial dispatch invoicing/returns are implemented in schema 26 (see continuation below).
- CSV/XLSX financial-journal imports with transactional posting, reference/idempotency protection, a downloadable template and paginated review before posting. File validation now precedes database access; account existence, locks and duplicate-reference checks remain transactional. This is not sales/purchase transaction import.
- Offline plaintext/encrypted backup restore for installations that cannot start. Requires an explicit installation directory, confirmation and exclusive instance lock. Existing database/WAL/SHM files are preserved; encrypted input authenticates before replacement. This is restore assistance, not automatic repair.

## POS and analytics continuation

- POS cash tender/change calculation, remaining-payment display, removable split-payment parts and balance-prefilled new parts. Tender is not posted as revenue/payment; only invoice allocations are recorded.
- F6 last quantity, F9 checkout, Escape back to cart and Alt+N next paid sale, alongside existing F2/F4/F8 shortcuts.
- Paginated POS active customer/product lookup, preserved customer selection and independent exact barcode/SKU lookup with ambiguous-code rejection.
- Customer-sales and monthly-sales analytics with credit-note-date adjustments, original COGS and gross profit. Low-stock reporting uses current minimum levels against closing-date stock across warehouses. All three use existing CSV/XLSX/PDF exports.
- Go and frontend compilation pass. Regression cases are written but execution remains deferred; no schema change is required.

## Partial dispatch continuation (schema 26)

- Dispatch invoices can select a subset of products/quantities. Uninvoiced quantities can return to the warehouse in multiple partial returns, including after other quantities have been invoiced.
- Immutable per-product allocations consume frozen base quantities and original cost. Cumulative cost allocation preserves every paise across mixed invoices/returns. Invoice credit notes do not reopen dispatch availability.
- The UI offers base-unit quantity selection, remaining balances and paginated allocation history. Drafts reserve nothing; finalization rechecks availability and rejects stale competing quantities atomically.
- Legacy whole-dispatch invoice/return links migrate into allocations. Backup verification accepts schema 26. Diagnostics reconcile dispatch allocations with the goods-dispatched asset account.
- Allocation dates cannot predate earlier allocations; stock-return chronology and period locks still apply. This earlier batch linked one invoice to one dispatch; the later consolidation continuation supersedes that limit.
- Builds and regression-test compilation pass; runtime execution, migration execution and browser checks remain deferred.

## Consolidated verification update

The schema-26 Linux batch has now been checked: frontend build/unit cases, Go race-enabled tests and vet, 27 browser workflows across initial/focused runs, and compiled-application crash/recovery smoke checks including encrypted offline CLI restore. Failures were incorrect test expectations/selectors, corrected and rerun successfully; no application-code fix was required. See TEST_REPORT.md for exact run boundaries, coverage and evidence. Historical deferred-verification notes above are superseded only for the checks covered by this run. The application remains incomplete and Windows runtime/compliance verification remains outstanding.

## Large-catalog selector continuation

- Billing/commercial documents, direct goods receipts and dispatch creation now use shared searchable, paginated party/product/warehouse selectors. Active/type filters apply before pagination.
- Selected parties and warehouses remain visible when paging/searching or reopening drafts, including records beyond the first page. Selected-record lookup is business-scoped. Warehouse search can reach records beyond the former 200-row selector limit in these workflows.
- Purchase billing suggests the configured purchase price for newly added lines. Saved line prices remain unchanged.
- Focused verification passes: backend pagination/isolation regression with race detection, server vet, frontend/Go builds and six browser workflows, including 51 customers/products and warehouse 201. This does not repeat the full consolidated suite.

## Stock and configuration selector continuation

- Stock additions, removals, damage and transfers use paginated active PRODUCT lookup and searchable source/destination warehouses. Successful stock posting clears selections; failed posting preserves them.
- Unit conversion and price-list setup use paginated active product lookup with SKU labels. Catalog selection is independent of the saved-record table filter; a selected product remains visible across searches.
- Frontend and application builds pass. Three focused browser workflows passed across initial and corrected-selector runs, including product 51, stock transfer/report reconciliation, and alternate-unit billing/POS. No full suite was repeated.

## Invoice import, dispatch consolidation and GST preparation continuation

- CSV/XLSX sales/purchase import now supports whole-file preview, editable drafts or atomic finalized posting, stable per-business reference protection, and existing configured tax/unit/adjustment rules. Finalized import uses shared interactive billing logic for stock, costing, numbering and journals. It assigns application invoice numbers; payments and receipt/dispatch source links are not imported.
- Multiple dispatches from the same customer/warehouse can contribute selected base quantities to one invoice. Cost is assigned separately to each dispatch; finalization rechecks availability. Existing single-dispatch behavior is retained. No migration is required beyond schema 26.
- User-selected GST scope: GSTR-1 and GSTR-3B **preparation exports**. Both are available in Reports and CSV/XLSX/PDF. Frozen invoice classification and dated credit notes feed the worksheets. Booked ITC candidates and unclassified inward supplies are identified separately. These are not portal schemas or legally reviewed completed returns; unsupported special cases remain explicit.
- The full standard Go suite passes with race detection after correcting refactor defects and a missing state in a new GST test fixture. Six affected browser workflows pass. See TEST_REPORT.md for evidence and run boundaries.

## Working-database encryption continuation

- Optional SQLCipher 4.19.0 native Linux executable: `release/ledgerleaf-linux-encrypted`. The standard executable rejects encryption requests instead of silently ignoring them.
- `--database-password-stdin` creates/unlocks a keyed database; startup verifies SQLCipher encryption is active. `--encrypt-database` converts a stopped plaintext installation while preserving its original DB/WAL/SHM files. Passwords are not command arguments or stored in configuration.
- Online restore into an unlocked encrypted installation encrypts the staged replacement before swapping it in. Offline recovery restore remains plaintext and requires explicit conversion afterward.
- Local backups and retained pre-conversion originals remain plaintext. Encrypted backup exports remain a separate feature. The native Linux encryption build depends on system OpenSSL/zlib; Windows encryption packaging and final security review are deferred.
- Functional SQLCipher checks cover encrypted bytes, wrong-key rejection, conversion, backup export, staged restore and server restore. Startup/restart/conversion CLI checks use disposable installations.
- Self-testing instructions: TESTING_GUIDE.md. Project-owned files and generated artifacts: FILE_MAP.md.

## Configuration and business lookup continuation

- GST setup, party balances, variant product/group assignment and opening-stock warehouses now use paginated searchable selectors. Selected records survive searching; party balances distinguishes customers from suppliers.
- Variant group lookup reaches beyond 500; business switching reaches beyond 200 and preserves the current page and selected business across searches. Server membership restrictions apply before pagination.
- Warehouse master listing now pages and searches instead of stopping at 200. Party/product IDs are displayed for invoice-import preparation.
- Both Linux executables are refreshed. Focused backend checks and grouped browser coverage are recorded in TEST_REPORT.md. No schema migration or full-suite rerun was required.

## Verification

Verification is recorded in [TEST_REPORT.md](TEST_REPORT.md). Reproduce with `scripts/test-application.sh`. Tests use real disposable SQLite databases and the compiled application, including browser workflows, concurrent writes, rollback, backup recovery and process crash/restart.

## Pending work by requested phase

1. Foundation: live integrity diagnostics, downloadable count-only reports and recovery guidance are implemented. Offline restore assistance is implemented; automatic startup repair remains pending. Authentication remains optional until the first administrator is created.
2. Master data: financial-year settings and structured variant relationships are implemented; broader master workflows remain subject to batch verification. Logos, images and unit conversions are implemented. Categories/brands/variant labels, credit limits/periods and price lists are implemented.
3. GST/invoicing: specialized place-of-supply/valuation cases, tax on advances and full compliance review. Configured quantity-based cess is implemented. Configured-rate calculation is not automatic legal classification or portal filing.
4. Inventory/purchases: direct and purchase-order receipts, partial/consolidated invoice matching, and full reversal of wholly uninvoiced receipts are implemented. Invoiced goods use purchase returns; matching stays allocated after returns. Receipt-less purchase bills still receive all their lines at finalization.
5. Accounting: additional correction workflows. Recurring expenses/categories/attachments and year-end closing entries are implemented. Date locks, multiple bank accounts and manual bank reconciliation are implemented.
6. POS/additional documents: partial dispatch invoicing/returns are implemented; multi-dispatch consolidation is implemented for the same customer and warehouse. The expanded POS shortcut and cash-change workflow is implemented. Independent whole-dispatch challans are implemented.
7. Reports: GSTR-1/GSTR-3B preparation exports are implemented for the user-selected scope; portal filing schemas, specialized scenarios and further analytics remain outside those worksheets. Customer-sales, monthly-sales and low-stock reports are implemented. The implemented HSN summary is a preparation report, not a filing-ready return. Generic financial, tax preparation, party, ageing and stock reports are available with exports.
8. Import/security: sales/purchase transaction import and optional Linux SQLCipher working-database encryption are implemented as described above. Windows encryption packaging remains deferred. Financial journal import and encrypted backup copies are implemented. Scheduled backup/retention and business-specific access are implemented.
9. Verification: actual Windows startup/workflow tests, offline clean-machine tests, 2 GB hardware benchmarks, killed-in-progress migration/power-loss tests, full accessibility testing, remaining-workflow tests, dependency vulnerability review and support plan. A prior Linux large-dataset benchmark is saved in `release/benchmark.json`.

## Known limitations

- Partial/full credit notes, GST posting and manual journal reversals are implemented. Specialized tax/compliance scenarios remain limited. The overall specification remains incomplete.
- Password mode is optional. Existing accounts retain all-business access until an administrator assigns a restricted set. Offline recovery requires local filesystem access and a stopped application; see docs/usage.md.
- Restore replaces every business and all users. Know the credentials from the selected backup. Import only stages a verified backup; explicit restore is a separate action. Windows restore replacement has not been runtime-tested.
- GST rates and classifications are user-configured; this build does not determine legal eligibility, current product rates or filing obligations. See docs/gst.md.
- Browser selection uses the OS default; no compatibility helper yet. PDF complex-script shaping and physical printer output are unverified; one A4 design only.
- POS customer/product selectors and Billing/commercial/receipt/dispatch master selectors are paginated. Stock-operation, unit-conversion and price-list selectors are also paginated. GST setup, party balances, variant products/groups and opening-stock warehouse selectors are now paginated too. Business switching supports search/pages beyond 200 while enforcing assignments. User assignment now searches and pages through all businesses, retaining selected IDs across pages. Each restricted account still supports at most 200 assignments.
- Backdated stock postings are rejected to preserve valuation chronology. The prior Linux large-dataset benchmark does not establish Windows or 2 GB hardware performance.
- Windows compatibility is **unverified**, even though binaries cross-compile with the intended legacy toolchain.

## Build and architecture

See README.md, docs/building.md and docs/compatibility.md. Go 1.20.14 is mandatory for release builds. SQLite uses pinned go-sqlite3 v1.14.22 with static CGO because the evaluated pure-Go driver lacks Windows x86. React targets Chromium 109, without bundling a browser engine. PDFs use gofpdf v1.16.2 and embedded DejaVu Sans. Data stays local; no .env files or existing business databases/services were accessed. Development dependencies were downloaded, and local temporary test databases/servers were used as authorized by the specification.

Next continuation: work through the pending list above. Source now uses schema 26. The verified executable is `release/ledgerleaf-linux`; the existing `./ledgerleaf` executable predates the latest source changes; Windows artifacts have not been refreshed in this continuation, per the request to focus on local testing rather than release packaging.

Final-stage sequencing requested by the user: Windows, physical printer, accessibility, security and compliance verification are deferred until implementation work is finished. This does not defer ordinary functional checks of new code. Remaining implementation gaps include specialized GST/advance handling, further correction workflows, automatic startup repair and remaining configuration/business selector limits.

## Everyday-use baseline — 2026-10-19

This batch prioritizes a usable Linux trial of normal trading operations over completing niche requirements. The overview links to parties, products, billing, POS, reports and backups according to the user's role. Billing now exposes server-side invoice-number/party search, sale/purchase filters and an outstanding-only view that excludes drafts, settled and fully reversed bills. Finalized bills show paginated direct-payment history with account codes; advance allocations and return refunds remain separate workflows.

No schema migration is required. Both Linux executables are refreshed. Follow TESTING_GUIDE.md for a clean installation and the purchase → stock → sale → payment → report → backup walkthrough. Use the release executable rather than the older root or Windows binaries. This is a functional trial baseline, not completion of the full specification or final production sign-off. Specialized GST/advance taxation, broader corrections, automatic repair, remaining legacy selector limits and the deferred platform/release checks remain outstanding. Batch verification results are in TEST_REPORT.md.

## Compact workflow dialogs — 2026-10-19

- Business creation/editing now opens from an explicit action in a dialog; optional contact, banking and invoice details are collapsed. The Businesses screen is a searchable paginated directory, replacing its old capped overview list.
- Party/product create/edit, warehouse creation and opening stock use reusable dialogs. Bill draft/view, payment and return dialogs keep the document list underneath. Validation errors stay in the relevant dialog.
- Shared master selectors show a compact selected value plus Browse action. The lookup dialog supports live search, paging and selection without leaving a bill. Existing selected-record preservation and business scoping remain intact.
- Tax/pricing/delivery and adjustments/notes are collapsible bill sections. Native dialog focus handling, Escape dismissal, background scroll lock and confirmation on changed-form Escape/top-right dismissal are implemented. Explicit Cancel/Close still means discard. On small screens the dialog body scrolls internally.
- No backend or schema change. Other specialist configuration screens retain their existing layouts; this does not claim that every screen has been redesigned or that missing records can be created inline from a lookup.

## Configuration dialogs continuation — 2026-10-19

GST item configuration, unit conversion, list pricing, financial-year changes, variant creation/assignment and party transactions now open from explicit actions. Manual journals, new accounts, journal reversals and recurring-expense editing use the same dialog pattern. Successful saves close the dialog and refresh records without resetting the list page/search; failed validation retains entered values. Party transaction choices display readable action names. Discarding a calendar edit preserves the saved-month summary. No backend/schema change; remaining screens such as bank reconciliation, receipts/dispatch entry and user management retain their existing layouts.

## In-app help and language continuation — 2026-10-19

Offline Help is available from the sidebar, contextual header action and login screen. Eleven guides cover the principal workflows in English, Hindi and Gujarati, with cross-language search and permitted navigation shortcuts. English remains the default; the browser remembers the selected language. Navigation and about 200 common UI labels/actions are translated. Stored business values, backend codes, money formats and PDF/export output remain unchanged. Specialized messages and some dynamic text retain English fallback; complete interface translation, native-speaker review and Windows script/font verification remain outstanding. See LOCALIZATION.md.

### 2026-10-20 — Database Excel export and import templates

Implemented administrator-only export of all database tables across all businesses into one `.xlsx` workbook, including archived rows and binary attachments represented as base64 chunks. Export uses one read transaction and completes into private temporary storage before download. `_Guide` documents tables, units, sensitive contents and limitations. This is not a SQLite restore format. Added blank downloadable product and party XLSX templates to existing validated imports; invoice and journal imports remain available. Added in-app help in all three languages.


## 2026-10-20 — User management usability

Account creation/editing and password changes now open in compact dialogs. Account search filters the directory. Restricted-business assignments use paginated search instead of the legacy first-200 list; selections remain independent of visible rows, including when editing existing accounts. The 200-assignment per-account server limit is unchanged and displayed. Administrators retain installation-wide access. No schema/backend changes.


## 2026-10-22 — Bank dialogs, expense accounts and language updates

Bank-account creation and ledger-to-statement matching now use dialogs that preserve validation errors and entered values. Ordinary and recurring expenses now accept cash or a configured bank account belonging to the selected business, using the existing payment-account validator. Their forms offer named bank accounts; schedules still require confirmation before posting. No migration is needed.

Language changes now use React's external-store subscription, and the login screen subscribes directly. The updated Hindi login and translated Help checks passed in the first current-code grouped run. Final expense/bank verification is recorded in TEST_REPORT.md.

Build distinction: latest changes are in `release/ledgerleaf-linux-dev`, built offline with installed Go 1.27.2. Previous standard/encrypted release binaries were retained because the temporary pinned Go 1.20.14/SQLCipher toolchains were unavailable. Refreshing pinned release artifacts remains pending; see building.md.


## 2026-10-22 — Goods workflow dialogs and broad UI regression

Goods-receipt creation, receipt quantity matching and receipt corrections now use the shared dialog component. Dispatch creation also uses a dialog; its table has a horizontal scroll wrapper for narrow screens. Successful submissions close the relevant form, while failed validation retains entered values. Existing list selection, paging, invoice allocation and stock rules remain unchanged. Dispatch allocation/history/consolidation still use their existing panels.

The latest development binary is refreshed; pinned standard/encrypted release rebuilds remain pending. Desktop/mobile visual checks and the regression results are recorded in TEST_REPORT.md. Help and TESTING_GUIDE.md describe the new entry actions.


Startup follow-up: the broader run reproduced an intermittent login-language failure despite the earlier targeted pass. The workspace had rendered interactive controls before authentication completed. App startup now shows a loading/error view until ready or login is required, removing that transient selector and avoiding premature workspace requests. Seven final affected workflows passed; the 41-workflow run and rerun boundaries are documented in TEST_REPORT.md.


## 2026-10-22 — Dispatch allocation dialogs

Dispatch invoicing, uninvoiced returns, history and multi-dispatch consolidation now use the shared modal component, with busy-close protection and internal table scrolling. The existing shared number parser rejects negative allocation quantities; regression coverage now checks that rejection creates no invoice draft. Stock/accounting behavior and backend APIs are unchanged. Earlier dated statements that these remain inline panels are superseded by this entry. Verification evidence is in TEST_REPORT.md; the runnable artifact remains the development executable.
