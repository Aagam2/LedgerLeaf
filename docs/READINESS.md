# Application readiness

Updated 2026-10-23. **The application is not complete or production-ready.** The current deliverable is a Linux development build with substantial working billing, inventory and accounting functionality.

## Implemented core workflows

Customers/suppliers and products; sale and purchase invoices; saved-draft review and confirmed finalization; purchase orders, quotations and proformas; receipts and dispatches; stock movements; payments and returns; accounting and reports; PDF-first downloads with secondary PNG pages; users/business access; backup and restore.

The 2026-10-23 standard Go suite passed with race detection, and the final complete browser run passed **44 of 44 workflows** against fresh SQLite databases. This includes real invoice/order PDF and PNG exports. See TEST_REPORT.md for executed checks and their limits. Passing tests verify the implemented behavior; they do not complete missing features or establish release readiness on other platforms.

## Remaining feature work

- A dedicated customer sales-order workflow is not implemented. Purchase orders and quotations are available; an invoice draft or quotation is not a separate sales order.
- Specialized GST/place-of-supply/valuation and advance-tax handling remain incomplete. GST preparation exports are not portal-ready return filing.
- Further accounting correction workflows, automatic startup repair and browser compatibility assistance remain pending. Diagnostics and offline recovery tools already exist.
- Full interface localization and PDF complex-script support remain incomplete.

## Remaining release checks

Pinned-toolchain release rebuilding and Windows runtime/encryption packaging; clean-machine/offline installation; accessibility, security and compliance review; migration interruption/power-loss testing; low-memory hardware checks; and support/release preparation. Existing Linux development checks do not substitute for these. Physical-printer checks are separate from the implemented PDF/PNG file-export workflows.

## Database access scope

On 2026-10-23 the user authorized access to this application's database only. Verification uses fresh disposable application databases and loopback test instances. Existing business records are left untouched. `.env` files must not be read; access to unrelated databases or services is not authorized.

The next implementation priority should be to settle and implement the customer sales-order workflow, then finish the remaining agreed feature scope before final release verification.
