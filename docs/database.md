# Database

Seven embedded migrations define business/audit records, master data/accounting/stock, immutable-ledger triggers, billing/payments, document reversals, users/actor audit and business profiles. `PRAGMA user_version` is updated in the same transaction as each pending migration batch. Opening a newer schema fails rather than downgrading it.

All connections enable foreign keys, WAL journaling, FULL synchronous writes, a 5-second busy timeout, and immediate write transactions. One connection per process avoids divergent per-connection settings and bounds concurrent writers. Database filenames are URL-escaped. Parameterized SQL is used for input.

Money is integer paise; quantities are integer thousandths of a unit. Individual invoices and opening-stock values are capped at 1,000,000,000,000 paise. Per-line quantity is capped at 1,000,000,000 thousandths. Go uses int64 and big.Int intermediates; frontend parsing uses decimal strings and validates safe integers.

Stock balances derive from immutable stock movements, not editable counters. Accounts, journals, stock movements and payments are scoped by business. Foreign keys include business ID where a child references a business-owned record. Unique business/SKU, invoice number, posting reference and idempotency keys prevent duplicates.

Finalized documents store a JSON snapshot of parties, business, product descriptions, units, prices, quantities, discounts, costs and totals. Their records cannot be updated/deleted. Journal lines, stock movements, payments and audit events cannot be updated/deleted. Balanced manual journal APIs protect inventory/receivable/payable control accounts. Full document returns and manual/expense journal reversals append corrections rather than modifying history.

A purchase bill without a matched receipt receives all lines and posts the payable at finalization. Partial purchase-order receipts post stock and goods-received-not-invoiced; a matched supplier invoice clears that liability without receiving stock twice. Document date cannot predate prior movements for affected stock; this prevents corrupting chronological weighted-average valuation.

Backups use VACUUM INTO to create a standalone snapshot, with SQLite integrity, foreign-key, balanced-journal, schema and stock checks. Import stages and verifies the uploaded file before listing it. Restore takes an exclusive API lock, stages/verifies the source, saves a recovery backup, checkpoints/closes SQLite, replaces the file and reopens/migrates it. A raw copy of a live .db without its WAL is not a verified backup. Migration and transaction tests use disposable local databases and never existing business data.

## Continuation migrations 15–16

Migration 15 adds the singleton backup policy with schedule interval, automatic snapshot retention, last-success time and last error. It is installation-wide and included in backups. Migration 16 adds optional user business restrictions and foreign-key-backed membership assignments. Existing users default to unrestricted access; administrator accounts retain installation-wide access. Assignment changes are transactional with the user update and audit entry.

## Continuation migrations 17–18

Migration 17 adds immutable product unit factors, scaled to thousandths of a base unit. Bill lines retain the selected unit, quantity and price plus a frozen base unit and factor. Older snapshots without a factor use 1:1 conversion. Stock movements always remain in base units.

Migration 18 stores immutable image assets in SQLite, with business-scoped current logo/product pointers. Invoices store a logo asset ID rather than duplicating image bytes in every snapshot. Pointer replacements preserve historical image bytes; backups include current and historical assets.

## Continuation migration 19

Migration 19 adds account 5300 (Invoice rounding) to existing businesses. Its schema bump also guards the new JSON snapshot semantics: older builds must reject these databases rather than reverse invoices without their rounding allocation. Invoice snapshots preserve fixed document discount/charges, computed per-line allocations and optional payable rounding. Rounded purchase payables do not alter inventory value; rounding uses a separate journal account. Credit notes allocate original payable totals cumulatively so complete returns clear every paise.

Administrator diagnostics use the exclusive API request lock and a bounded context to report structural and accounting checks without writes. Reports contain aggregate issue counts and no source records.

## Continuation migration 20

Goods receipts now allow a null order reference. The table is rebuilt transactionally with historical IDs, snapshot bytes, invoice links and immutability triggers preserved. `receipt_matches` records immutable base-unit quantities and receipt-cost allocations per receipt, product and invoice; existing whole-receipt invoice links are backfilled. `receipt_reversals` records immutable dated corrections with reasons. Backups validate both new tables.

Receipt availability is checked inside the invoice transaction. Cumulative receipt-value allocation preserves the exact original cost across successive partial invoices. Invoice finalization writes the matches and financial journal together; quantities are not posted twice. Receipt reversal removes current weighted-average stock cost and clears the original uninvoiced liability, using stock adjustments for any difference. Drafts do not reserve quantities and become invalid if receipts are consumed or reversed.
