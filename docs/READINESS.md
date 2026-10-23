# Application readiness

Updated 2026-10-23. **Production hardening targets the everyday billing workflow.** The deliverable is a Linux release candidate; final production approval still depends on the release checks below. Completing every feature in the original specification is outside this release's scope.

## Implemented core workflows

Customers/suppliers and products; sale and purchase invoices; saved-draft review and confirmed finalization; purchase orders, quotations and proformas; receipts and dispatches; stock movements; payments and returns; accounting and reports; PDF-first downloads with secondary PNG pages; users/business access; backup and restore.

The 2026-10-23 standard Go suite passed with race detection, and the final complete browser run passed **44 of 44 workflows** against fresh SQLite databases. This includes real invoice/order PDF and PNG exports. See TEST_REPORT.md for executed checks and their limits. Passing tests verify the implemented behavior; they do not complete missing features or establish release readiness on other platforms.

## Deferred features (not release blockers)

- A dedicated customer sales-order workflow is not implemented. Purchase orders and quotations are available; an invoice draft or quotation is not a separate sales order.
- Specialized GST/place-of-supply/valuation and advance-tax handling remain incomplete. GST preparation exports are not portal-ready return filing.
- Further accounting correction workflows and browser compatibility assistance remain pending. Diagnostics and offline recovery tools already exist. Startup checks structural integrity and stops on damage; automatic destructive repair is deliberately excluded.
- Full interface localization and PDF complex-script support remain incomplete.

## Remaining release checks

The Linux candidate build uses installed Go 1.27.3 and installed, locked frontend dependencies. `scripts/release-linux.sh` builds a static executable, runs the full automated verification against that executable, and packages version/revision metadata, checksums, verification logs and an operator guide. It refuses uncommitted source, stale embedded assets and failed tests. It downloads no dependencies and does not read `.env` files.

Before general production distribution: clean-machine/offline acceptance; independent security and dependency review; keyboard/accessibility review; target-hardware performance; and realistic power-loss/filesystem testing. Automated crash recovery is tested but cannot simulate all hardware/storage failure modes. Review invoice/tax behavior for the business's actual requirements; GST preparation is not return filing.

Windows release builds remain separately blocked on legacy pinned-toolchain qualification, runtime/browser acceptance and encryption packaging. Linux verification does not establish Windows readiness. Physical-printer checks are separate from the supported PDF/PNG file exports.

## Data-safety hardening

Startup runs SQLite `quick_check` before migration. Existing versioned schemas receive a private standalone snapshot containing committed WAL data before any schema upgrade; snapshot failure cancels the upgrade. Migration failures retain the snapshot and roll back the migration batch. Keyed builds preserve snapshot encryption. Plain SQLite exports now enforce private permissions and fsync, matching encrypted exports. Unix shutdown handles SIGTERM as well as Ctrl+C, allowing service managers to drain requests and release the instance lock. `--version` identifies a binary without opening a database.

See [the operator guide](OPERATIONS.md) for installation, administrator setup, backups, updates and recovery. Daily automatic backups remain opt-in; enabling them and saving an external copy are first-use requirements.

## Database access scope

On 2026-10-23 the user authorized access to this application's database only. Verification uses fresh disposable application databases and loopback test instances. Existing business records are left untouched. `.env` files must not be read; access to unrelated databases or services is not authorized.

Release priority is safe invoices, purchase orders, payments, stock, reports, backups and PDF-first/PNG exports with simple daily entry. Specialized features above do not hold up this scope.
