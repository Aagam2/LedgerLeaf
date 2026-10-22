# Using the development build

1. Launch the executable and open its launch link in a compatible browser. Create a non-GST business using a two-digit state code. Every business starts empty, with a main warehouse and chart of accounts.
2. Add customers/suppliers under Parties. Add products/services under Products. SKU is unique within a business; quantities support three decimal places and prices two. Use Edit to update or archive a record. Product type cannot change; units cannot change after stock movement.
3. Use Inventory → Opening stock to enter initial quantity and **total cost**. This creates stock history and opening inventory/equity entries. Opening stock is accepted once per product/warehouse and is not editable.
4. Use Billing → New bill. Choose sale or purchase, party, warehouse, dates and lines. Search selects from the first 50 matching records. For purchase bills enter actual purchase cost, overriding the product's default selling price. Discounts are absolute per-line amounts. Save the draft; drafts can be edited.
5. Finalize only after checking the saved details. Sales need sufficient stock. Purchases receive stock immediately. Finalization allocates a sequential number per business, document type and Indian April–March financial year, posts inventory/accounting and freezes the snapshot. Use Return / cancel for a full reversal, with date/reason and the actual refund account. Partial returns remain unavailable. Original documents stay immutable; returned views and PDFs are marked.
6. Record partial or full payments against finalized bills. All methods are manual records. View finalized bills to download an A4 PDF and print it using a PDF viewer. Print output has not been tested on physical printers; complex-script typography remains unverified.
7. Use Stock changes for additions, removals, damage and warehouse transfers. Use Expenses for paid expenses; use Journal entries for balanced manual entries/custom accounts or reversing a manual journal/expense by its journal ID. Inventory, receivable and payable control accounts are protected from manual journals.
8. Use Reports for date-filtered financial statements, account books, stock and audit history, and CSV export. Switch businesses using the sidebar selector. Edit business to change contact/bank/terms; saved bills retain their original snapshot.
9. In Backup & restore, create and verify a snapshot, then download it to another device. Import accepts a downloaded standalone .db file up to 512 MB without changing live data. Restore all businesses replaces the entire installation database and users, retains a verified recovery snapshot and signs users out. Credentials from the backup apply after restore.
10. In Users & permissions, create the first administrator to enable password login. Add accountant, biller or viewer users as needed. Roles are installation-wide; billers can create sales at configured prices and record customer payments, while viewers cannot write business data. Keep the administrator password secure; no recovery tool exists yet.
11. Use Exit application to close the backend safely.

Use temporary/sample records while evaluating this build. No demonstration records are inserted automatically. If a request reports an error, correct the input and retry; transaction failures roll back, and posting operations are protected against duplicates. The full specification remains unfinished; this build is not suitable as the sole record of real business finances.

## Automatic backups

Administrators can open **Backup & restore → Automatic backups**, enable the schedule, choose an interval in hours and a retained-copy count, and save. The application checks once per minute while open and catches up at startup. It does not run when Ledgerleaf is stopped. All businesses and accounts are included. The policy is stored in SQLite and restored with a database backup.

Automatic files start with `Auto-Ledgerleaf-`. Retention deletes only older automatic files, and only after a new snapshot verifies successfully. Manual, imported and pre-restore snapshots are never automatically pruned. The screen reports the last successful automatic backup and any error when loaded. Copy important backups to another device: local snapshots cannot protect against loss of the disk.

## Offline administrator password recovery

Stop Ledgerleaf first. On this Linux development machine, run:

```sh
./ledgerleaf --data-dir ./local-test-data --recover-admin your-admin-username
```

On Windows, use the equivalent executable and actual installation data directory. The command requires an existing administrator and an explicit data directory. It refuses to run while another instance owns the directory. It verifies a backup before changing the password, then prints a newly generated password in the terminal. Start Ledgerleaf, sign in using that password and change it under **Users & permissions**. The password is not written to application logs. The pre-reset backup contains the old credentials, so restoring it also restores those credentials.

## Business assignments and read-only access

Administrators can limit a non-administrator to selected businesses in **Users & permissions**. A checked restriction with no selections grants access to no businesses. Unchecked means all businesses, including ones created later. Administrators always have installation-wide access. Changes revoke the edited user's sessions; they must sign in again. Business checks apply to API reads, downloads and writes as well as the selector.

## Duplicate a document

In Billing or Orders & quotations, **Duplicate** opens a new editable draft. Review the date, due date, customer, prices and taxes before saving. The copy does not reuse a source order or goods receipt, and saving does not post financial entries. A copied purchase bill without a goods receipt receives its own stock when finalized.

Application logs rotate at 5 MB; `application.log.1` through `.3` retain the three previous files.

## Alternate units

Open **Unit conversions** and select a product. For a product stocked in PCS, create BOX with factor 12 to mean one box contains twelve pieces. For KG, a G factor of 0.001 means one gram is one thousandth of a kilogram. Conversions cannot be changed or deleted once created; use a different name for a new pack size. There can be up to 50 alternate units per product. A product's base unit cannot change after conversions or stock movements exist.

Billing and POS offer a unit selector. Quantities and unit prices are expressed in that selected unit. Changing the selector resets the price to the configured selling/purchase price multiplied by the factor; review it, especially for purchases. Named sales price lists use the same conversion. Discounts remain whole-line amounts.

Stock records stay in the base unit. Invoice/return/receipt quantities must convert exactly to three decimal places in that base unit; excessively small fractional amounts are rejected instead of rounded. Returns use the original invoice unit. Purchase-order receipts use the order unit, and matched purchase invoices can use another unit if the base quantities match.

## Logos and product images

Administrators can upload or remove a business logo in the Overview business profile. Administrators and accountants can upload/remove product images when editing a saved product. Use PNG or JPEG up to 512 KB and 2048 × 2048 pixels. Unsupported print encodings are rejected for logos.

Images are stored in SQLite. Backups include them. Replacing/removing a current logo leaves historical images available to saved documents; resaving a draft adopts the current business logo. A4 and thermal invoice PDFs include logos. Historical images are intentionally retained, so repeated replacements increase database size.

## Product and HSN summaries

In Reports, choose **product sales** for net sales, cost and gross profit, or **hsn summary** for grouped tax preparation totals. Returns reduce values on the credit-note date, including when the original invoice is outside the selected period. Quantities are shown in the frozen base units. Product gross profit excludes operating expenses. HSN summaries retain separate sale/purchase, supply-treatment and rate groups; they do not replace legal classification, standardized unit-code review or prescribed GST returns.

## Invoice discount, charges and rounding

In Billing or Orders & quotations, enter **Invoice discount ₹** and **Allocated charges ₹**. These are fixed document amounts, after item-level discounts. The discount is allocated proportionally across the items; charges are then allocated across their remaining values. Both follow each item's configured tax treatment and the inclusive/exclusive price setting. Use this for costs sharing the supplied items' tax treatment; it does not classify separately taxable services. An invoice discount cannot exceed the item subtotal, and charges cannot be added to a fully discounted subtotal.

Check **Round total to nearest rupee** to round the payable total, half up. The difference posts separately to account 5300 (Invoice rounding); it does not change taxable value or inventory cost. Saved drafts, duplicates and commercial conversions retain the options. POS preserves saved line and invoice discounts/charges/rounding when resuming a draft. Billers cannot introduce invoice discounts or charges.

Line amounts already include the allocated adjustments. PDFs show the document-level summary and round-off. Partial returns allocate the original amounts cumulatively, including the payable rounding; returning all quantities restores the exact invoice total. Rounding a very small invoice to zero is rejected.

## Integrity diagnostics

An administrator can open **Backup & restore → Run integrity checks**. It checks every business and briefly holds other API requests while running, with a 30-second timeout. Results cover database integrity, foreign keys, journals, stock balances, inventory-ledger reconciliation and document/credit-note/receipt snapshot totals. Download the JSON report for investigation; it contains counts and results, not business names, customer records, credentials or paths. A successful report does not establish tax compliance or physical stock correctness.

If a check fails, stop transactions, exit the application and preserve the complete data directory, including WAL/SHM files, before recovery. Verify a known-good backup and restore through the existing workflow; restoration replaces all businesses and user accounts. Repeat diagnostics after restore. These checks never repair or rewrite records automatically.

## Direct receipts, invoice matching and receipt corrections

In **Goods receipts**, choose **Without a purchase order**, select an active supplier and warehouse, add products and enter estimated unit costs excluding tax. Alternate units are supported. Posting receives stock and credits Goods received, invoice pending (2050); it does not yet create a supplier payable.

Select receipt checkboxes and choose **Match selected receipts**, or **Choose quantities** for one receipt. Enter quantities in the displayed base stock units. Set a quantity to zero to leave it uninvoiced. An invoice can combine multiple receipts from the same supplier and warehouse, and a receipt can be split across several invoices. Suggested prices are estimates rounded to the supported unit-price precision: review prices, taxes and input credit in Billing before finalization. Matching invoices must cover exactly their allocated base quantities; all items must be receipt-backed.

Drafts do not reserve quantities. Another finalized invoice or a receipt reversal can invalidate an older draft; reload receipts and create a new matching draft. Finalization atomically stores allocations, clears the corresponding receipt liability, records the supplier payable and any cost variance in Stock adjustments (5100). It does not receive stock again. Invoice IDs and partial/full status appear in the receipts list. Existing whole-receipt drafts keep their original behavior.

To correct a wholly uninvoiced receipt, choose **Correct receipt**, supply the correction date and reason, then confirm **Reverse goods receipt**. This removes the full quantity at current weighted-average cost, reverses the original receipt liability and records any valuation difference. It requires enough stock and cannot predate later stock movements or fall in a locked period. The original receipt remains immutable. Its purchase-order quantities become available again; enter a replacement receipt with corrected details if needed.

Receipts already partly or fully invoiced cannot use this correction action. Use the purchase return workflow in Billing for invoiced goods. Returns do not reopen receipt allocations for another invoice.

## Financial years, variants and year closing

Use **Catalog settings** to choose the financial-year starting month before posting journals or issuing numbered documents. The default is April. Variant groups link separately stocked product SKUs using attributes such as `color=blue` and `size=large`; grouping does not merge stock or prices.

In **Banks & periods**, select the last day of the configured financial year and choose **Preview year closing**. Review the proposed transfers to retained earnings (3100), verify reports and create a backup. **Close financial year** recalculates the balances and permanently locks postings through that date. The preview does not reserve balances. An empty year can be closed without a transfer journal. Closing entries remain visible in the ledger and balance sheet but do not distort profit-and-loss reporting.

## Recurring expenses and attachments

In **Expenses**, create a recurring schedule with a category, cash/bank account, amount, next date and interval. Each due payment requires confirmation; reminders never record payments automatically. Posting advances the schedule while retaining the intended day through shorter months. Attach PDF, PNG or JPEG receipts to posted expenses (up to 5 MB each). Category reports appear under Reports.

## Financial journal imports

Under **Import & export**, download the journal CSV template and fill one account line per row. Dates use YYYY-MM-DD; debit and credit are integer paise, with exactly one positive side and zero on the other. Lines sharing a reference must have the same date and description and balance exactly. Files support up to 20 MB and 10,000 rows, with 1,000 journals and 100 lines per journal.

Choose **Validate financial import**, review the paginated reference/date/description/debit/credit preview, then choose **Post financial import**. Validation saves no records. Posting rechecks everything and rolls back the whole file if any reference fails. Cash, bank, opening equity, expenses and custom accounts are supported; operational control accounts require their normal workflows. Corrections use journal reversals. Importing sales or purchases through these journals is not supported.

## Independent dispatch challans

In **Dispatch challans**, select a customer, warehouse and products in base units. Posting moves stock value to Goods dispatched, invoice pending (1250). Print the challan or create its Billing draft, review prices/taxes and finalize. Finalization recognizes the sale without issuing stock again. Select invoice or return quantities in base units. Partial invoicing and returns are supported; quantities already invoiced require the Billing return workflow. Review supply-specific document and tax timing requirements independently.

## Encrypted backup copies and offline restore

Under **Backup & restore**, select a verified snapshot and enter a 12–256-byte password to download an encrypted `.llbenc` copy. Keep the password separately; it cannot be recovered. Decrypt/import verifies a copy before making it available for the separate restore action. Working data and local unencrypted snapshots remain plaintext.

If the application cannot start, stop it and preserve the complete installation directory. Restore a known-good plaintext snapshot using:

```sh
./ledgerleaf --data-dir /path/to/installation --restore-backup /path/to/snapshot.db --confirm-restore
```

For an encrypted copy, add `--restore-encrypted`. The password is read as one newline-terminated line on standard input, never as a command-line argument. In Bash, read it without echoing or placing it in shell history:

```sh
read -r -s -p 'Backup password: ' restore_password
printf '\n'
printf '%s\n' "$restore_password" | ./ledgerleaf --data-dir /path/to/installation --restore-backup /path/to/snapshot.llbenc --restore-encrypted --confirm-restore
unset restore_password
```

Use the equivalent executable on Windows and a secure standard-input mechanism. Do not put passwords directly in command text. Recovery requires an existing installation and refuses to proceed while another instance owns its lock. It authenticates/decrypts and verifies the snapshot before replacing the database, retaining the original database/WAL/SHM files under `backups/pre-offline-restore-*`. A temporary plaintext staging file is removed on normal completion or failure; after an abrupt process or power interruption, preserve the directory for investigation, including hidden staging files. Deleting temporary files is not a secure disk erase.

Restore replaces every business and user; sign in with credentials from the selected snapshot. Start the application and run integrity diagnostics afterward. Offline restore does not repair corrupt records or combine backups with newer transactions.

## POS shortcuts, lookup and cash change

POS supports F2 for scan/search, F4 to hold, F6 to edit the last line's quantity, F8 to review, F9 to finalize/pay, Escape to return from review to the cart, and Alt+N for the next sale after successful payment. Payment is available only from the checkout review. A finalized sale with a failed payment must be settled by retrying or through Billing; do not create a replacement sale.

Product and customer results can be paged. Customer lookup filters active customers before pagination and preserves the selected customer when browsing other pages. Pressing Enter in barcode/SKU search looks up an exact active code independently of the visible search page. Ambiguous codes require manual product selection.

At checkout, edit the split-payment allocations and remove unwanted parts. Adding a part fills the remaining balance when it is positive. Allocations must be positive and sum to the invoice total. For cash payments, optionally enter **Cash tendered ₹** to display change or a shortfall; leaving it blank means exact cash. Only the allocated cash amount posts to the ledger, not the tendered cash or change. Tender/change is a checkout aid and is not stored on the receipt.

## Customer, monthly-sales and low-stock reports

**Customer sales** ranks customers by net sales, with invoice/credit-note counts, original cost of sales and gross profit. **Monthly sales** groups the same figures by posting month. Credit notes reduce their own period, including when the original invoice predates the range. Net sales exclude tax and invoice round-off; gross profit excludes operating expenses. Customer names come from the current master record, and inactive months are omitted.

**Low stock** shows active stock products below a configured positive minimum at the selected closing date. Quantities are base units summed across all warehouses; goods already dispatched are not available warehouse stock. The threshold uses today's product minimum rather than a historical threshold. Products without a positive minimum are omitted. All three reports support paginated viewing and CSV/XLSX/PDF export.


## Partial dispatch invoicing and warehouse returns

Open **Invoice dispatch** or **Return dispatch** to review original and remaining quantities. Set unwanted items to zero and choose a date. The selection uses frozen base stock units, even when the original dispatch used alternate units. Invoice drafts suggest base-unit prices rounded to paise; review prices and taxes in Billing before finalization.

Each invoice can consume part of one dispatch; later invoices or warehouse returns can consume the balance. Drafts do not reserve stock or dispatch quantities. A competing finalized invoice or return can make a saved draft stale; reload the dispatch and edit/recreate the draft. No additional stock issue occurs at invoice finalization.

Warehouse returns restore only uninvoiced quantities at their allocated original dispatch cost. All quantities and cost are allocated cumulatively so the final part receives any remaining rounding paise. Dates cannot precede an earlier allocation, the dispatch date, or (for warehouse returns) later stock movements. Locked periods are rejected.

**History** shows posted invoice and warehouse-return allocations, their quantities and costs. Returning an invoiced item through Billing restores stock using the invoice's cost but does not reopen that dispatch quantity for invoicing again. Existing whole-dispatch links are retained in history during the schema-26 upgrade. Back up before upgrading; older application versions cannot open schema 26.


## Searching larger catalogs

Billing, commercial documents, direct goods receipts and dispatch creation provide a search field and Previous/Next controls for party, product and warehouse selection. Each page contains up to 50 matching active records of the required type. Warehouse lookup in these workflows can reach records beyond the old 200-row limit.

Changing the search or page does not clear an already selected customer, supplier or warehouse. The selected record remains an option even when it is outside the current results; reopening a draft resolves its saved IDs independently of the first page. Archived saved selections are marked inactive and still require correction before posting when the workflow requires an active master.

New purchase-bill lines suggest the configured purchase price; review it before saving. Existing draft line prices are preserved. Search/pagination controls do not submit the enclosing transaction form.

### Stock and configuration catalog lookup

Stock changes, unit conversions and price lists support product search and pages of 50 active records. Stock changes show physical products only and provide independent source/destination warehouse search. Unit-conversion and price-list record-table filters are separate from product selection. Selected products remain selected when searching another page; successful stock movements and new unit conversions reset their entry forms.

## Latest import, consolidation and encrypted-storage workflows

For hands-on startup instructions and expected test results, see [TESTING_GUIDE.md](TESTING_GUIDE.md). Sales/purchase CSV/XLSX imports can preview the complete batch and either save drafts or finalize atomically. Dispatch list checkboxes combine selected quantities into one invoice for the same customer and warehouse. Reports now includes GSTR-1/GSTR-3B preparation worksheets. The optional SQLCipher Linux build uses `--database-password-stdin`; the standard build rejects this mode. Online restore retains encryption in an unlocked keyed installation; local snapshots and pre-conversion originals remain plaintext.

For implementation navigation, see [FILE_MAP.md](FILE_MAP.md).

### Configuration lookup and business search

Use Find business in the sidebar to locate businesses beyond the initial list; switching retains the current page. Search/paging does not clear the selected business. GST setup, party balances, variant products/groups and opening-stock warehouses now support independent paginated search. Variant groups can be found beyond group 500, and the Warehouse master table is paginated. Parties and Products display record IDs for imports; warehouse IDs are also shown in their table.

## Faster daily entry

**Save & review** saves the current bill/order as a draft and displays its calculated total without returning to the list. Use **Edit draft** to correct it, then **Finalize** or **Issue** and confirm before printing. **Save draft** still saves for later. The running item subtotal uses entered prices and does not replace the backend's tax/adjustment calculation. **Orders & quotations → + Purchase order** starts directly with the supplier and purchase prices.

## PDF and image downloads

Use **Download PDF** as the primary export for invoices, orders/quotations, credit notes and dispatch challans. **Export image (PNG)** produces shareable page images from the same saved print content. A short document downloads one PNG; a long one presents individual page links. PDF and PNG have separate layouts. These are file exports; physical printing is optional through a viewer. POS keeps its receipt PDFs and adds PDF/PNG controls for the A4 invoice.
