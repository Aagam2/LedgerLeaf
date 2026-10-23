# Application test report

Updated 2026-10-23. Results describe Linux verification against disposable application databases. They do not certify general production readiness or Windows runtime compatibility. Newer dated entries supersede older environment/scope notes.

## Production-hardening baseline — 2026-10-23

At implementation revision `788aa21`, `scripts/release-linux.sh` completed successfully: frontend build/unit tests, the complete standard Go suite with race detection, Go vet, all **44/44 browser workflows**, and compiled lifecycle/recovery smoke. The smoke now checks `--version` and graceful SIGTERM/restart alongside crash persistence, ledger reconciliation, administrator recovery, scheduled backups and encrypted-backup offline restore. All data was disposable; no `.env` files or unrelated services were accessed. Evidence: `release/linux-candidate-wGgf3A/Ledgerleaf-0.1.0-rc.1-linux-amd64/verification/`.

New database tests verify committed WAL data in pre-upgrade snapshots, original schema retention after snapshot failure, migration rollback/snapshot retention, and private snapshot permissions. The keyed snapshot test is skipped without SQLCipher. The baseline executable linked statically but produced glibc warnings from optional SQLite extension loading/system DNS. The subsequent candidate explicitly removes those capabilities and adds a build-tagged test proving that SQLite native extension loading is absent. Final candidate evidence is recorded separately after that run; RC1 is superseded for distribution.

## Everyday-use Linux batch — 2026-10-19

The current standard Linux executable passes the full Go suite with race detection, Go vet, TypeScript/Vite build, frontend unit files, and compiled-application smoke/recovery checks. The server package completed in 126 seconds. No existing business database or `.env` file was accessed; all runtime checks used disposable installations.

The browser recheck passed 32 of 33 workflows. The remaining master-import test used a partial label match that also selected the newer invoice-import input; its exact-label correction passed separately in `import-recheck.log`. All 33 browser workflows therefore pass across the recheck and focused rerun. The initial run had 17 selector failures: 16 from new overview shortcuts sharing navigation names and one from the import label. Tests now scope sidebar navigation explicitly. No runtime application defect was exposed by those failures. The new browser case verifies overview navigation, search/type/outstanding filters, partial payment history and exclusion of a fully settled bill. Backend coverage also verifies document/business isolation and invalid payment-history IDs.

Evidence: `release/verification-product-2026-10-19/` contains build, frontend tests, full Go race tests, vet, browser initial/recheck, focused import and smoke logs. Both Linux executables were rebuilt; SQLCipher-specific runtime checks from the preceding batch were not repeated here. Windows, physical printers, accessibility, security and tax-compliance release verification remain deferred. This establishes a local functional trial baseline, not full-specification completion or production certification.

## Consolidated Linux verification — 2026-10-18

The schema-26 source has now been exercised using disposable databases and local loopback servers. No existing business databases or `.env` files were accessed. No application source changes were required by the observed failures.

| Check | Result |
|---|---|
| TypeScript/Vite production build | Pass |
| Frontend unit cases | 8 cases across 2 test files pass |
| Go suite with race detector and coverage | 96 top-level tests exercised; initial run had one incorrect test expectation, corrected test passes with race detection |
| Go vet | Pass |
| Browser workflows | 27 workflows pass across the initial run and focused reruns/additional coverage |
| Compiled-app smoke/recovery checks | Pass, including encrypted offline CLI restore |
| Coverage | 66.1% statements from the initial consolidated run; not complete branch coverage |

The initial backend run failed only because the new low-stock test expected HTTP 201 from warehouse creation; the established endpoint returns HTTP 200. The expectation was corrected and `TestLowStockUsesClosingDateAndAllWarehouses` passed with `-race -count=1`. All other backend tests passed in the original consolidated run. This is not a claim that the entire suite was rerun after the test-only correction.

The sandbox blocked Chrome startup (`setsockopt: Operation not permitted`) before application execution. Browser and process checks were rerun with approved permissions for disposable local installations. The 26-workflow browser run passed 23 tests; three POS tests had ambiguous status-message selectors after cash-change status was added. Corrected selectors and the three focused reruns pass. The POS test now verifies cash tender/change and the F9 shortcut, and confirms only the invoice total is recorded. A further encrypted-backup browser workflow passed, bringing verified browser workflows to 27.

New browser coverage includes partial dispatch quantity selection, invoicing, warehouse return/history and reconciled stock; financial journal preview without writes followed by posting; year-close preview without closing followed by confirmation; encrypted backup download/import and wrong-password rejection. The expanded executable smoke test verifies encrypted CLI restore after deliberate corruption of a disposable installation, rejection of a wrong password without changing the original file, preservation of corrupt bytes, restored login/payment records and passing diagnostics.

Evidence is saved under `release/verification-2026-10-18/`. Initial logs retain their failures for traceability; focused successful browser logs and final smoke output sit alongside them. The coverage profile and summary remain in `release/coverage.out` and `release/coverage-summary.txt`.

The verified Linux executable is `release/ledgerleaf-linux`. The root `./ledgerleaf` executable and Windows artifacts were not refreshed. Previous “deferred” entries below describe earlier batches and are superseded for the checks explicitly covered here. Windows runtime, clean-machine/2 GB hardware, physical printers, full accessibility/security/compliance review and real power-loss verification remain outstanding.

## Environment and data safety

Linux amd64 with Go 1.20.14, Node/npm and the installed Google Chrome. All test databases are disposable SQLite fixtures. Browser/process tests run the compiled application on IPv4 loopback; the browser harness blocks external page resources. No existing business databases or `.env` files were opened.

## Purchase-receipt phase verification (2026-10-18)

This batch uses focused verification, as requested; the full regression suite and Windows cross-compilation were not repeated.

- TypeScript/Vite production build and Go vet for the changed server/database packages pass.
- Targeted receipt, receipt-matching, alternate-unit receipt, adjusted-order and populated migration tests pass with the race detector. Existing backup/upgrade checks also passed in the focused non-race run.
- New tests cover direct receipt idempotency; splitting and combining quantities; stale drafts; exact cumulative cost allocation; journal-failure rollback after allocation writes; alternate units; historical link migration; immutable records; correction stock shortage/period locks; correction valuation variance; reversed receipt rejection; and reopening purchase-order quantities.
- Existing quotation conversion and purchase-order receipt browser workflows pass. The new direct/partial/consolidated receipt workflow also passes after dropdown-label refinement, including Billing edits, finalization, correction and final stock reconciliation.
- Schema is now version 20. The prior full-suite results and coverage below do not include this batch.

## Previous full-suite verification (2026-10-17)

| Check | Result |
|---|---|
| Go tests (76 top-level tests plus subcases) | Pass |
| Go race detector / Go vet | Pass |
| Frontend unit tests | 4 pass |
| TypeScript / Vite build | Pass |
| Browser workflows | 23 pass in the final full run |
| Compiled executable crash/recovery smoke | Pass |
| Windows x86/x64 compilation | Pass with Go 1.20.14; temporary check artifacts only |
| Windows DLL imports | KERNEL32.dll and msvcrt.dll only |

The 2026-10-17 complete verification script passed before the purchase-receipt phase: 76 top-level Go tests with race detection, Go vet, four frontend tests, the production frontend build, 23 browser workflows and the compiled-application smoke checks. An initial browser run exposed a sign-out test timing race: it filled the old account form before navigation completed. The test now waits for the sign-in screen, and the final complete run passed.

The prior full-suite coverage profile reports **68.8% of statements**. This is not complete branch coverage or a correctness certification. Evidence: `release/coverage.out` and `release/coverage-summary.txt`.

The current Linux executable is `./ledgerleaf`, with the same bytes as `release/ledgerleaf-linux`. Reproduce verification with:

```sh
GO_BIN=/tmp/billbook-toolchain/go/bin/go \
GOCACHE=/tmp/billbook-go-cache ./scripts/test-application.sh
```

The script builds TypeScript/Vite, runs frontend tests, the Go suite with race detection and coverage, Go vet, 23 browser workflows and a compiled-executable crash/recovery smoke test. Chrome and loopback execution require permissions outside the default sandbox on this host. The initial sandbox browser run failed before exercising the app; rerunning outside the sandbox passed the prior 14 workflows.

## New continuation coverage

- Scheduled backups: disabled policy, valid/invalid policy, due intervals, retaining the newest automatic snapshots, preserving manual backups, failed destination writes without pruning, error reporting and recovery, automatic file verification, and startup catch-up in the compiled executable.
- Administrator recovery: unknown users and non-administrators rejected; backup failure leaves credentials unchanged; prior credentials retained in a verified snapshot; generated password works; audit record persists. Process checks reject recovery while the instance is running, reject the old password after reset, allow changing the generated password and preserve financial records.
- Business access: filtered business list; denied cross-business reads, writes, reports and PDF requests; invalid assignment rollback; assignment changes revoke sessions; empty assignments grant no business access; administrators cannot be restricted.
- Log rotation across restart, recent-log retention and rejection of writes after close.
- Browser checks for scheduled policy persistence, duplicate invoice remaining an unposted draft, restricted biller/viewer controls, partial purchase-order receipt to supplier invoice without double stock receipt, and user business assignments.
- Schema migrations through version 19, plus whole-batch migration rollback, old-data preservation, future-schema and corrupt-database rejection.

## Unit, image and analytics continuation

- Alternate-unit purchase/sale quantities post base-unit stock correctly; non-divisible per-piece prices retain exact per-box monetary totals. Repeated partial returns restore exact stock and cost. Invalid fractional base quantities fail without posting. Conversion definitions and base-unit changes are protected.
- Goods receipts in boxes match later invoices in pieces without receiving quantities twice. Inclusive GST, converted biller prices and restricted configuration writes are covered.
- Logo/product upload validation, role enforcement, cross-business isolation, frozen invoice logo references, A4/58 mm/80 mm PDF images, backup/restore of current and historical images, and removal of current pointers are covered.
- The generated logo invoice was rendered with Poppler and visually inspected; the logo preserves its aspect ratio and does not overlap invoice text. Evidence: `release/logo-invoice-preview.pdf`.
- Product-sales and HSN summaries reconcile net quantities, taxable value, tax, cost and gross profit across partial returns, including returns whose original invoice lies outside the date filter. CSV/XLSX/PDF exports and business isolation pass. Browser checks cover normalized quantities, percentage display and XLSX download.

## Invoice adjustments and diagnostics continuation

- Invoice discounts and allocated charges distribute every paise across mixed-rate lines. Tests cover tax-inclusive/exclusive values, reverse charge, eligible/ineligible purchase input credit, rounding outside inventory cost, repeated partial returns, exact refunds, receipt matching, validation rollback, computed-field tampering and biller restrictions.
- Billing form values survive editing and POS resumption, including the preexisting line discount. Browser checkout and three successive partial returns reconcile to the exact rounded payable. A4/58 mm/80 mm PDF generation passes. The adjusted A4 PDF was rendered and visually inspected; negative round-off displays correctly. Evidence: `release/adjusted-invoice-preview.pdf`.
- Schema 19 prevents older application builds from opening records whose snapshot rounding semantics they cannot safely reverse. Migration and backup tests pass.
- Live diagnostics pass on valid stock/accounting data and detect injected unbalanced journals and inventory-ledger discrepancies in a disposable database. Tests confirm no writes, no private record/token leakage, administrator-only access and JSON report download/recovery instructions in the browser.
- Windows x86/x64 cross-compilation passed again using temporary artifacts; imports remain KERNEL32.dll and msvcrt.dll. Actual Windows execution remains unverified.

## Existing workflow coverage

Backend tests cover GST/inclusive pricing/reverse charge, partial/full credit-note allocation and refunds, purchase/sale weighted-average valuation, concurrent posting, shortage and injected-write rollback, idempotency, immutable documents, manual and split payments, journals, party opening balances/advances/refunds, bank matching/period locks, credit limits/price lists, partial goods receipts/invoice matching, CSV/XLSX imports and transactional rollback, reports, PDF extraction, backups/import/restore and authentication.

The browser suite covers sale/stock/payment/PDF; purchases and partial payments; insufficient stock; duplicate SKU/edit/archive/business switching; session/theme/mobile layout; paid returns; backup create/download/import/verify/restore; stock transfers/expenses/reports; users and roles; profiles/manual journals; GST/partial credit notes; quotation conversion; POS hold/resume/split payments/thermal PDF; and CSV import/report exports. Additional workflows are listed above. API responses are not mocked.

The compiled-app smoke test checks embedded assets, launch authentication, single-instance locking, committed invoice/payment/stock persistence after forced process kill, token rotation, balanced journals and foreign keys, PDF retrieval, graceful restart, offline password recovery and scheduled backup startup.

## Prior large-data benchmark

`release/benchmark.json` records the previous continuation's Linux benchmark: 10,000 products, 5,000 customers, 2,000 suppliers, 100,000 invoices, 500,000 invoice lines and 1,000,000 stock movements. Median product search was 5.08 ms, first invoice page 3.75 ms, deep invoice page 146.03 ms, stock page 465.93 ms and profit/loss 528.42 ms. This benchmark was not rerun in this continuation. It does not measure browser memory, 2 GB hardware, Windows, cold-disk performance or physical printers.

## Verification limits

- No runtime tests on Windows 7/8.1/10/11 or Chrome 109, and no clean-machine or physical-printer testing. Compilation is not proof of Windows compatibility.
- No full accessibility audit, actual power-loss test or killed-in-progress migration test. Transaction fault injection does not emulate physical disk failure.
- No OS-level network-isolation certification. Browser resource blocking is narrower.
- Unimplemented features in IMPLEMENTATION_STATUS.md are not covered as completed functionality.
- Existing Windows files under `release/` predate this continuation. The user requested local Linux testing without release packaging.

## Current continuation — verification deferred

The latest source contains schema 25 and the master/accounting, quantity-cess, encrypted-backup, dispatch and journal-import batch described in IMPLEMENTATION_STATUS.md. Focused checks for the earlier portion were reported in the preceding conversation; this report does not claim a fresh full-suite pass for those changes.

Before the user's reminder to defer tests, this continuation ran the frontend build/unit-test command and a Go package build successfully. Subsequent changes add encrypted offline restore, year-closing previews and journal-import validation/previews. New parser regression tests are written but not yet run. Database, browser and service checks have not been executed in this continuation. Tests are deferred for the larger batch as requested. Existing executables and Windows artifacts have not been refreshed.

After completing this source batch, TypeScript/Vite and `go build -buildvcs=false ./...` passed again with network dependency resolution disabled for Go. This was compile verification only; no regression tests, database connections or application servers were run. Encrypted offline-restore regression coverage was also added but remains unexecuted.

## POS and analytics batch — compiled, runtime verification pending

Go package compilation and TypeScript/Vite production build pass for the POS cash-change, shortcut, lookup-pagination and analytics changes. No tests, databases or application servers were run during this batch. Added, unexecuted regression cases cover exact paise/change allocation, split cash versus non-cash, invalid numeric inputs, return-only reporting periods, customer/monthly report business isolation/exports, and closing-date low-stock thresholds across warehouses. Browser keyboard/scan timing and new report SQL still require the deferred runtime checks. Existing executables have not been refreshed.


## Partial dispatch batch — runtime checks deferred

Implemented schema 26, cumulative invoice/return allocations, quantity-selection UI, allocation history and dispatch diagnostics/backup compatibility. Go and frontend builds pass; server and database regression-test binaries compile without executing tests. New unexecuted cases cover split invoicing mixed with warehouse returns, repeated return requests, stale competing drafts, cumulative paise allocation, invoice-credit availability, immutable records, and populated schema-25 migration of invoice/return links. No database was opened and no application server was started during this batch. Actual migrations, rollback/fault injection, backup/restore and browser flows still require runtime verification. Existing executable artifacts have not been refreshed.


## Selector batch — focused verification

After the consolidated run, the shared master selectors were introduced in Billing, commercial documents, receipts and dispatches. Focused checks pass: `TestMasterPickerFiltersPaginationAndIsolation` with the race detector; server-package vet; TypeScript/Vite and Go builds; and six browser workflows (quotation conversion, order receipt matching, direct/partial receipts, dispatch creation/partial allocation, large-catalog selectors, and the core create/stock/bill/payment workflow).

The new browser case selects customer/product 51, retains a selected customer while paging/searching away, searches warehouse 201, saves and reopens the draft, and checks stored IDs. Backend coverage checks filtering before pagination, warehouse pagination beyond 200 and business isolation of selected-record retrieval. Evidence: `release/verification-selectors/browser.log`. Only disposable fixtures/loopback servers were used; `.env` files and existing business data were not accessed. The full suite was not repeated for this batch. `release/ledgerleaf-linux` contains the updated source; root and Windows executables remain older.

## Stock and configuration selectors — focused verification (2026-10-18)

- Frontend TypeScript/Vite build and Go application build passed; `release/ledgerleaf-linux` includes the updated embedded UI.
- Initial browser batch: stock transfer/expense/report and the new product-51 stock/unit/price workflow passed. Existing alternate-unit workflow stopped on an ambiguous test label after addition of the search field; its selector was made exact, then the complete alternate-unit billing/POS workflow passed.
- These runs used disposable test installations and local loopback servers. No full Go/browser suite was repeated for these frontend changes.
- Evidence: `release/verification-stock-selectors/browser-initial.log` and `browser-units-recheck.log`. Windows runtime and physical printers remain unverified.

## Invoice imports, dispatch consolidation, GST preparation and database encryption — 2026-10-18

This implementation batch adds shared invoice preparation/finalization helpers, atomic CSV/XLSX draft/finalized imports, multi-dispatch allocation, GSTR-1/GSTR-3B preparation reports, and optional SQLCipher working storage.

- **Full standard Go suite:** `go test -race -count=1 ./...` passes. This exercises disposable databases only. Earlier refactoring checks exposed pointer/Scan mistakes that were corrected. A new reverse-charge preparation test initially lacked the supplier's state; the fixture was corrected before the final passing suite.
- **Frontend/application builds:** TypeScript/Vite and standard/encrypted native Go builds pass. Both Linux executables have been refreshed. Windows artifacts remain older.
- **Go vet:** passes on the implementation batch.
- **Six browser workflows:** quotation conversion, partial goods receipts, invoice adjustments/POS, partial dispatch, atomic purchase/sale file import, and consolidated dispatch editing/finalization all pass. The complete browser suite was not repeated.
- **SQLCipher functional checks:** encrypted bytes and keyed reopen, wrong password rejection, plaintext snapshot export, conversion preserving the original, encryption of restore staging, and server backup/restore retaining encryption pass against SQLCipher 4.19.0. These are separate from standard-build tests, which skip SQLCipher-only cases and verify unsupported encryption fails closed.
- **CLI checks:** new encrypted installation, restart, offline conversion, preserved plaintext original, wrong-password byte preservation and standard-build rejection pass. The first CLI run found a real URI-encoding issue for passwords containing spaces; SQLite URI parsing does not translate `+` to a space. Open now uses `%20`, and regression passwords include spaces plus `&`, `=`, `?`, `#` and an apostrophe. The focused storage/recovery/server tests and CLI smoke pass after that fix.
- **Documentation:** TESTING_GUIDE.md contains startup/rebuild commands and manual workflows. FILE_MAP.md lists project-owned files; relative links in both documents were checked for missing targets. The encrypted build script passes shell syntax checking.

Evidence is in `release/verification-implementation-2026-10-18/`: `go-race.log`, `browser.log`, `sqlcipher-tests.log`, `sqlcipher-restore.log`, `sqlcipher-special-password.log`, `sqlcipher-cli-initial.log`, `sqlcipher-cli.log`, `sqlcipher-recovery-final.log`, and `vet.log`. Initial failures and later passing checks are distinguished; there is no claim that a failed initial run passed.

The full standard suite ran before the later password-URI correction and conversion rollback-error reporting adjustment; focused SQLCipher checks cover those later changes. Windows/runtime/printer/accessibility/security/compliance review remains deferred at the user's request. Working-database encryption does not encrypt existing snapshots, retained plaintext originals, browser downloads or exported reports. The native encrypted binary is not yet a portable Windows release.

## Configuration and business lookup batch — 2026-10-18

Frontend build, standard/encrypted Linux application builds and server vet pass. Focused race-enabled backend checks cover business 201, variant group 501, selected-record isolation, master filtering and restricted business assignments. Four browser workflows are covered across initial/recheck runs: large configuration catalogs/business search, business isolation, GST invoice/credit-note flow, and the core opening-stock/invoice/payment flow. The initial business-switch test exposed an unintended navigation reset; switching now retains the active page, and both affected workflows pass on recheck. No full suite was repeated.

Evidence: `release/verification-configuration-selectors/backend.log`, `browser-initial.log`, and `browser-recheck.log`. Documentation and both Linux executables reflect this batch.

## Frontend visual cleanup — 2026-10-19

Shared styling now provides a compact independently scrolling sidebar, consistent form spacing and input surfaces, grouped master selectors, clear table headers/actions, aligned checkboxes and native dark controls. Phone layouts stack forms and constrain navigation/table overflow. Desktop overview/billing, bill-entry dark mode and 390px mobile screenshots were reviewed under `release/ui-review/`; the mobile document has no horizontal page overflow. The TypeScript/Vite build and both Linux executable builds pass. Three existing browser workflows pass: theme/mobile layout, bill lookup/payment history, and record creation through stock/finalization/payment. This focused visual check does not constitute the deferred full accessibility or platform review.

## Dialog and lookup workflow batch — 2026-10-19

All **34 browser workflows pass** in `release/verification-dialogs/browser-final.log`, including first-run business creation, business profile edits, duplicate-record errors within dialogs, stock entry, billing/payment/return flows, paged master lookup, nested searchable selection, unsaved-change Escape cancellation and focus restoration. The earlier exploratory run retained four outdated interaction-selector failures while the picker UI was being finalized; the complete final run passes after updating those interactions.

TypeScript/Vite and both Linux executable builds pass. Screenshots of business setup, invoice entry, value lookup, dark mode and mobile layouts are in the same directory. The final visual-only CSS adjustment scopes optional-section ordering so the Save action follows the optional fields; the screenshot review was repeated after rebuilding. Mobile page overflow check passes. Backend behavior/schema is unchanged, so the previously passing backend suite was not rerun for this frontend batch. This is focused interaction verification, not the deferred complete accessibility audit.

## Configuration action dialogs — 2026-10-19

The full browser batch passed 33 of 35 workflows; two conversion workflows needed exact button selectors after adding the list-level action. Both corrected cases plus the new configuration-dialog case pass in the focused recheck, covering all 35 workflows across runs. New coverage verifies invalid conversion input remains in the dialog, a corrected save closes it, canceled financial-year changes leave the saved summary unchanged, variant-group creation, and recurring-expense creation. Existing GST, price-list, variant assignment, party-balance and journal workflows pass with explicit dialog opening.

TypeScript/Vite and both Linux builds pass. Configuration list/form screenshots, accounting overview and desktop/dark/mobile captures are reviewed; no mobile page overflow was observed. The screenshot fixture initially attempted a tax date on an already-finalized invoice date; it was corrected to a later date, respecting the existing tax-history rule. No application change was needed for that rejection. Evidence: `release/verification-config-dialogs/`, including initial and recheck logs. Only toolbar spacing changed after the full browser batch, with final builds, focused checks and screenshots afterward. Backend/schema behavior is unchanged; no backend-suite rerun or final accessibility certification is claimed.

## Offline Help and Hindi/Gujarati interface — 2026-10-19

The main browser batch passes all **37 workflows**, including cross-language Help search, Hindi/Gujarati switching, persistence after reload, localized login and permission-filtered Help shortcuts. A subsequent Hindi record-creation assertion exposed an ambiguous accessible label on the product-type dropdown; an explicit localized label fixes it, and the final three-case rerun passes. It verifies a Hindi-named service saves the unchanged `SERVICE` backend code, existing names/amounts stay unchanged, login/Help permissions work, and the English stock/billing/payment flow still passes. A mixed-language master-dialog title was also corrected.

TypeScript/Vite and both Linux executable builds pass. English, Hindi and Gujarati Help screenshots were reviewed; system Noto fonts render both scripts. Mobile Help uses a bounded topic list and shows no page overflow. All resources are local. Evidence is in `release/verification-help-language/`: `browser.log` (full batch), `browser-final.log` (the additional assertion's initial failure), `browser-localized-record.log` (final focused pass), build and screenshot logs/images. No full-suite rerun after the small accessible-label/title fixes is claimed. Backend/schema behavior is unchanged. Native-speaker editorial review, complete interface/error translation, PDF localization and final Windows font verification remain outside this batch.

## 2026-10-20 — Entire database Excel export

- Full Go race suite: passed (`release/verification-database-excel/go-tests.log`), including existing import rollback/round-trip coverage and new all-table export, attachment reconstruction, template and role checks.
- Focused export/template server checks: passed (`focused.log`).
- Frontend TypeScript/Vite build and frontend unit tests: passed.
- Browser checks for existing encrypted backup and translated Help: passed. The new download check required test-only navigation/select label corrections; initial failure logs retained. The final download workflow passed (`browser-download-final.log`).
- Standard and SQLCipher Linux executables rebuilt. Native Windows Excel opening and large production-size database trials remain final-stage checks. No real business database was opened.


## User-management continuation — status reviewed 2026-10-22

Frontend build and both Linux executable builds completed. In `release/verification-user-dialogs/browser.log`, account creation/viewer restrictions, UI business assignment and business-201/hidden-selection preservation passed (3 passes). The Hindi login language-switch check failed while waiting for the translated username label; the cause remains uninvestigated and this is an open verification item. `password-browser.log` separately records a passing password-change dialog error/success workflow. No full backend suite was repeated for these frontend-only changes. Do not interpret prior language-test passes as resolving this latest failure.


## 2026-10-22 — Banks, expense accounts and language subscription

Evidence: `release/verification-bank-language/`.

- Frontend TypeScript/Vite build and frontend unit checks passed.
- Focused backend tests passed for named-bank ordinary/recurring expenses, idempotency, cross-business and non-bank rejection, period locks/reconciliation, and existing recurring month-end behavior (`backend.log`). These ran with installed Go 1.27.2, without race instrumentation. The full backend suite was not repeated.
- Seven current-build browser workflows passed (`browser-final.log`): user roles, journal import/year closing, Help languages/data preservation, Hindi login, business-201 assignment retention, password error/success, and bank creation → named-bank expense → statement matching. The bank dialog screenshot was visually reviewed.
- This supersedes the previously unresolved Hindi login check. Earlier failed logs are retained: `browser.log` used an old release executable after the pinned build failed; `browser-dev.log` exposed the expense handler's default-account-only restriction, which was then implemented and verified.
- Current code was compiled into `release/ledgerleaf-linux-dev`; standard/encrypted release binaries were preserved. Their temporary pinned toolchains were unavailable. No release-toolchain, Windows, native printer or final compliance claim is made.


## 2026-10-22 — Goods dialogs, responsive review and startup follow-up

Evidence: `release/verification-goods-dialogs/`.

- Frontend build and unit checks passed; the development executable was refreshed with installed Go 1.27.2. No backend/schema changes were made in this batch, so the backend suite was not repeated.
- The complete regression browser file ran 41 workflows: 39 passed, two failed (`browser.log`). One expected the old inline password action; the test was updated for the current dialog. The other reproduced the intermittent Hindi login issue, superseding the prior targeted-pass confidence.
- Startup code previously rendered workspace controls while authentication was unresolved. It now shows a loading/error view before either login or the ready workspace. The language test also explicitly waits for the login heading and confirms the selected language.
- Final affected rerun: seven workflows passed (`browser-final.log`), covering both failures, translated Help, partial/direct receipt invoicing/corrections, dispatch allocation/returns, and responsive dialog layouts. All 41 workflows passed across the initial run and targeted rerun; this is not a second complete 41-test run after the startup change.
- Visually reviewed `receipt-desktop.png`, `receipt-mobile.png`, `dispatch-desktop.png`, and `dispatch-mobile.png`. Dialog overflow checks passed at 390px mobile width; desktop screenshots use 1280px width. No clipping or horizontal overflow was found in these views. This is focused visual QA, not the deferred full accessibility/platform review.
- Standard/encrypted release binaries remain older; use `release/ledgerleaf-linux-dev` for these changes. Documentation and in-app goods-workflow Help were updated.


## 2026-10-22 — Dispatch allocation/history/consolidation dialogs

Evidence: `release/verification-dispatch-dialogs/`.

Frontend builds and unit checks passed; the Linux development executable was rebuilt. The initial four-test run had three passes and one test expectation failure: the existing shared parser already rejects negative input with “nonnegative number,” rather than the redundant proposed error wording. The redundant validation code was removed and the test expectation corrected. This was not a newly discovered acceptance of negative quantities.

Final four-test run passed (`browser-final.log`): partial dispatch invoice/return/history with rejected negative input creating no draft, multi-dispatch consolidation without duplicate stock issue, Hindi login, and responsive receipt/dispatch dialogs. Desktop consolidation and mobile allocation screenshots were visually reviewed. Wide allocation tables scroll inside their dialog; the dialog itself does not overflow the mobile viewport. No backend/schema change or full backend rerun occurred. Pinned release rebuilds and final platform/accessibility/compliance review remain outstanding.

## 2026-10-22 — Repository baseline and operation/period dialogs

Initial Git commit `7aa7c24` captures all existing source, documentation, tests, migrations and embedded frontend assets before this batch. Generated executables, local databases, installed dependencies and Python caches are excluded. No `.env` file was read.

Changes cover ordinary expense/stock entry dialogs, period-lock editing with saved-state preservation, and a financial-year-closing dialog retaining the existing preview and permanent-close confirmation. Accounting, architecture, database and build documentation were reviewed against source and corrected where stale. No schema/backend change.

Executed checks:

- `npm run build --prefix frontend` — TypeScript and production frontend bundle pass. Vite reports its existing CJS Node API deprecation notice.
- `npm test --prefix frontend` — both existing numerical/POS payment test files pass.
- `node frontend/tests/dialogs.offline.mjs` (also available as `npm run test:ui:offline --prefix frontend`) — four browser scenarios pass with the real React components and an in-memory API stub: expense failure/retry preserves values, account choice and idempotency key; mobile stock transfer rejects negative quantities and sends exact base-unit quantities; period-lock Cancel/save and year-close date/preview invalidation/confirmation refresh the displayed summary/history; accountant accounts lack administrator period actions. Chrome required execution outside the restricted process sandbox. Browser contexts are offline and browser network requests are aborted.
- Offline Go build and `go vet ./...` — pass using installed Go **1.27.3**, `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off`, and a temporary build cache. Refreshed `release/ledgerleaf-linux-dev` embeds the final frontend bundle. The executable was compiled but not launched.
- `git diff --check` — passes for this continuation. Existing whitespace in the baseline's license notices/receipt source was retained unchanged.

Verification boundaries: no database connection, application server or external service was opened. No Go test suite, database-backed browser E2E suite, smoke test or manual live accounting workflow ran in this batch. The stubbed browser checks establish interface behavior and request payloads, not financial transaction correctness. Pinned release builds, Windows/runtime, printer, full localization, specialized tax handling and other previously deferred checks remain pending.

## 2026-10-22 — Simplicity and familiar workflows

- Frontend TypeScript/production build passes. Existing numeric/POS unit tests pass.
- Six offline browser scenarios pass using real React components with an in-memory API and blocked network requests. Existing expense/stock/period checks remain covered. Added checks retain all 29 menu destinations, expand groups through the shared E2E navigation helper, mark the current page, save an explicitly created purchase draft, update the new Hindi/Gujarati menu/purchase labels, and preserve sale-only biller actions on a 390-pixel screen.
- Reviewed desktop screenshots of the actual Navigation, Dashboard and Billing components in the offline harness. Screenshots use stubbed empty data and a minimal shell; they are not evidence of live business results or a full application audit. Checked purchase-dialog/mobile bounds in the browser.
- Full E2E discovery (`npm run test:e2e --prefix frontend -- --list`) succeeds: 42 tests across two files. Updated navigation clicks to open their group, and logo checks to expand business details. Discovery does not execute those database-backed tests.
- Rebuilt `release/ledgerleaf-linux-dev` offline with Go 1.27.3 and the refreshed frontend. The executable was not launched. `git diff --check` passes.

No `.env` file, database or service was accessed. No real posting, persistence, tax, platform or full accessibility validation was performed in this UI pass. The existing verification limitations remain in effect.

## 2026-10-22 — Daily order/invoice entry and review

- Frontend TypeScript/production build passes. All three frontend unit-test files pass, including new fractional-quantity/paise rounding, line-discount and subtotal-limit checks. An initial boundary fixture used the wrong rupee/paise magnitude; it was corrected and the limit checks rerun successfully.
- Nine offline browser scenarios pass. New coverage checks quantity focus/selection, repeated-item guidance without duplicate lines, running exact line/subtotal amounts, failed draft-save input preservation, server-returned review totals, editing/resaving the same draft ID, cancelled finalization without another request, failed finalization retry, and PDF-link readiness after finalization. Purchase-order coverage checks direct supplier/purchase-price entry, issue-response fallback and a 390-pixel layout. Viewer inspection offers no edit/finalize/draft-print actions. Existing expense, stock, period, localization and navigation checks remain covered.
- An initial order-selector check exposed its ambiguous accessible label. The control now has an explicit translated accessible name; the final run passes. Numeric boundary checking also now applies the line limit after its discount, consistent with the entered net amount.
- Desktop entry/review screenshots were visually inspected in the offline harness. Its data and returned totals are deliberately stubbed; they are not a real accounting reconciliation. Item subtotal remains visible alongside the entry actions. Review actions sit beside the calculated total.
- Full E2E discovery succeeds for 42 tests. Navigation aliases and quantity/price field selectors were updated. The database-backed tests were not executed.
- `release/ledgerleaf-linux-dev` was rebuilt offline with Go 1.27.3 and the final embedded frontend; it was not launched. `git diff --check` passes.

No database, application server or external service was accessed, and no `.env` file was read. Backend/schema code is unchanged. The harness validates interface state transitions and request payloads against in-memory responses; it does not verify real posting, stock/ledger persistence, tax calculation or PDF generation/printing. Final-stage checks and previously documented implementation gaps remain pending.

## 2026-10-22 — PNG export, PDF first

- Frontend TypeScript/production build and all four frontend unit-test files pass. New wrapping coverage includes long unbroken values, paragraph breaks, Unicode and literal markup text.
- Ten offline browser scenarios pass, including actual PNG downloads. The new export scenario verifies PDF is first/primary, image failures retain PDF and allow retry, a one-page download has a valid PNG signature and 1200 × 1697 dimensions, the saved logo is drawn, and title/status, GST/HSN/cess, adjustments, total and notes reach the canvas. A 100-item export produces multiple page links; all item names and final notes are drawn, and the final page downloads under its correct page filename. Existing daily-entry/role checks pass with the clearer Download PDF label.
- The first browser run found relative URL construction depended on the live page origin, unavailable in the opaque offline harness. Query construction now works directly from the local export path; the final run passes.
- Visually inspected actual single-page and last-page PNG files from stubbed invoice content. Pagination now keeps ordinary item blocks together. This is a separate image layout using local fonts, not a PDF rasterization or proof of production tax correctness.
- Pure Go `TestImageExportData` passes for invoice, credit-note, commercial and dispatch URL shapes: default PDF response is untouched, and image data round-trips the complete print model including logo bytes. Only this selected server test ran; it uses response recorders and no database/listener. Existing `internal/printing` tests pass, including PDF content/pagination and receipt sizes, without a database or service.
- Go vet and offline Linux development build pass with Go 1.27.3. Full E2E discovery succeeds for 42 tests; database-backed tests were not run. The development executable was rebuilt, not launched. `git diff --check` passes.

No `.env` file was read and no database, application server or external service was accessed. Live endpoint integration, Windows/browser/font coverage and physical printers remain unverified. PNG export is currently for business documents; report export formats remain CSV/XLSX/PDF.

## 2026-10-23 — Application-only database permission and consolidated readiness run

The user authorized database access for this application. All checks used fresh disposable application SQLite databases; browser checks started loopback-only application instances with `--no-browser` and blocked external browser requests. Existing business records were not opened or modified. No `.env` file was read. Database access was limited to disposable databases belonging to this application.

Executed verification:

- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE=/tmp/ledgerleaf-review-go-cache /snap/go/current/bin/go test -race -count=1 ./...` — the full **standard-build** Go suite passes. This includes accounting, backup, migrations/database, recovery and server tests. SQLCipher-only cases require the encrypted build and remain outside this run. The server package completed in 175.079 seconds.
- Initial browser run against `release/ledgerleaf-linux-dev`: **36 passed, 8 failed** in 5.4 minutes. Failures involved ambiguous stock-submit selectors, post-reload menu readiness, a legacy partial Import navigation selector, and a new assertion expecting an empty stock list rather than a product with zero stock. These were test failures; no application implementation changes were needed.
- Updated test steps to open the expense/stock/year-close dialogs, use exact submit selectors, and wait for navigation to exist before expanding its group. Updated the standalone workflow to honor `LEDGERLEAF_TEST_BINARY` and configured supported Chrome headless startup. A focused backup/navigation rerun passed.
- Final complete browser run: **44 passed in 1.9 minutes**, using `LEDGERLEAF_TEST_BINARY=/home/aagam-linux/Desktop/Projects/MyBillBook/release/ledgerleaf-linux-dev`, one worker, and the existing application fixtures. This is a full final rerun, rather than an aggregate of partial passes.
- New real daily-invoice test verifies saved-draft totals, cancelled and confirmed finalization, draft export rejection, PDF bytes, actual PNG file bytes/dimensions, partial payment, remaining stock and cross-business export rejection. New purchase-order test verifies saved totals, issue, real PDF/PNG downloads, zero stock movements and zero financial journals.
- `git diff --check` passes. The application implementation and embedded bundle were unchanged in this readiness pass; no release build was claimed or produced.

Local ignored evidence: `release/readiness-browser.log` (initial run), `release/readiness-focused.log`, `release/readiness-browser-final.log`, `release/readiness-final-results/`, `release/readiness-invoice.png` and `release/readiness-purchase-order.png`.

Conclusion: the implemented Linux workflows now have database-backed regression evidence, but the application is **not complete or production-ready**. Dedicated customer sales orders, specialized tax/advance handling, further correction/recovery work and full localization remain incomplete. Pinned release rebuilding, Windows runtime/encryption packaging, clean-machine/offline installation, accessibility/security/compliance reviews, migration interruption and hardware checks remain pending. See READINESS.md.
