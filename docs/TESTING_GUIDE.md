# Run and test Ledgerleaf yourself

Updated 2026-10-19. Use a separate test installation and fictional records. This is a development build; final Windows, printer, accessibility, security and compliance verification is still deferred.

## Daily-use baseline

Use the current Linux build for a hands-on trial of purchases, stock, sales/POS, payments, returns, reports and backup/restore. Niche GST scenarios and final release verification remain outside this baseline. The overview's **Everyday workflow** buttons lead to the relevant screens.

In **Billing**, use **Find documents** to search invoice numbers or party names, select sales/purchases, and check **Outstanding only** to find balances still due. Filters apply across all pages. Open a finalized bill with **View** to see its paginated **Recorded payments**, including date, mode, amount and account code. That table contains direct payments; review advance allocations in Party balances and refunds in Credit notes separately.

## Quick start on this Linux computer

Open a terminal and run:

```bash
cd /home/aagam-linux/Desktop/Projects/MyBillBook
./release/ledgerleaf-linux --data-dir ./manual-test-data
```

The program opens your default browser and prints a local URL containing `#launch=...`. If no browser opens, copy the complete URL into the browser. Keep the terminal open while testing. No Node, Go, npm, external server or internet connection is needed to run the already-built standard executable.

For manual browser launch:

```bash
./release/ledgerleaf-linux --data-dir ./manual-test-data --no-browser
```

The port and launch token change on every start. Use the new URL after restarting. The URL grants access to this installation; do not share it. Before administrator setup, the local launch session has administrator access. After setup, sign in using the application username/password as well.

Use **Exit application** in the sidebar or Ctrl+C in the terminal to stop. Closing the browser tab alone does not stop the program. Restart with the same command and directory to keep your records. To start a completely new test, use a new directory name such as `./manual-test-data-2`; there is no need to delete anything.

Do not use the older root `./ledgerleaf` executable or the existing Windows executables for testing the latest changes. The current standard Linux executable is `release/ledgerleaf-linux`. The SQLCipher variant is `release/ledgerleaf-linux-encrypted`.

## First end-to-end walkthrough

1. Create a business named **My Test Shop**, state code **27**, GST registration **Unregistered**. Leave GSTIN blank. This keeps the first arithmetic check simple. A main warehouse is created automatically.
2. In **Parties**, add one **SUPPLIER** and one **CUSTOMER**.
3. In **Products**, add a physical product, SKU `TEST-001`, base unit `PCS`, sale price ₹150, purchase price ₹100. UI amounts use rupees and normal quantities; file imports use integer paise/thousandths.
4. In **Billing**, create a **PURCHASE** draft for 10 units at ₹100 from your supplier. Use today's date for invoice and due date. Save, reopen and review, then finalize. Expected invoice total: **₹1,000**. Expected stock: **10 PCS**, value **₹1,000**.
5. Create a **SALE** for 2 units at ₹150 to your customer, using the same date or a later date. Finalize. Expected sales total: **₹300**, remaining stock **8 PCS**, stock value **₹800**, gross profit **₹100**, customer outstanding **₹300**.
6. Record a ₹100 payment. Outstanding should become ₹200. Record another ₹200 payment. Outstanding should become zero. Attempt an extra payment to confirm it is rejected.
7. Inspect **Reports**: stock, sales, purchases, profit-loss, customer-sales and trial-balance. Download CSV/XLSX/PDF. CSV and spreadsheet numeric money columns generally use **integer paise**; quantities use **integer thousandths**. The screen formats them for reading.
8. Open the finalized invoice and download its PDF. This checks generation and content, not physical printer compatibility.
9. Stop the application and restart using the same test-data directory. Your business and transactions should remain.
10. In **Backup & restore**, create a backup and run diagnostics. Add a clearly named test record, then restore the backup with the explicit confirmation shown in the UI. Restore replaces **all businesses and users**, not just the selected business. The added record should disappear, and a pre-restore recovery backup should remain.

Record the observed totals and exact error message if any step differs. Avoid backdating stock transactions before existing movements: the application rejects that to preserve valuation chronology.

## Try the latest features

### Sales and purchase import

Open **Import & export → Sales and purchase invoice import**. Download the CSV template. Use party and warehouse IDs belonging to the selected business and exact active product SKUs. The Parties, Products and Warehouses tables display record IDs. Use those IDs; they are installation-specific and must not be guessed.

The minimum layout is:

```csv
reference,kind,date,due_date,party_id,warehouse_id,sku,quantity,price
PUR-TEST-001,PURCHASE,2026-10-18,2026-10-18,1,1,TEST-001,10000,10000
SALE-TEST-001,SALE,2026-10-18,2026-10-18,2,1,TEST-001,2000,15000
```

Replace the example IDs with your supplier/customer/warehouse IDs and use dates consistent with your stock history. In this file `10000` quantity means 10 units, `10000` price means ₹100, and `15000` price means ₹150. References are unique across invoice imports within a business.

Choose **Save editable drafts** first. Click **Preview invoice import**, review each reference and total, then confirm. Preview saves nothing. Open Billing to inspect the drafts. For a later file with new references, try **Finalize and post invoices**: the entire batch posts stock, invoices, numbering and journals together, or none of it does. Put stock-receiving purchases before dependent sales. Repeating the same upload request does not duplicate invoices; reusing an imported reference under a new upload is rejected.

The full template supports line discounts, alternate units, tax mode/place of supply, tax-inclusive prices, input-credit selection, shipping address, notes, invoice discount/charge and rupee rounding. Repeat invoice-level fields identically for all lines in the same reference. No historical invoice-number preservation, payment import, or receipt/dispatch-linked import is provided. Imported finalized invoices receive the application's invoice sequence.

### Combine dispatch challans

Create two dispatches for the same customer and warehouse using available stock. In the dispatch list select their consolidation checkboxes and click **Combine selected dispatches**. Load the balances, choose base-unit quantities and create the consolidated draft. Open Billing, review prices and tax settings, then finalize.

Shared products become one line; the first selected dispatch supplies the initial price suggestion. The immutable allocation ledger tracks each source dispatch separately. Invoicing must not remove warehouse stock a second time. Drafts reserve no quantity: a stale draft can fail after another invoice or warehouse return consumes availability. Invoice returns do not reopen the original dispatch allocations.

### GST preparation exports

Use a separate **Regular GST** test business with a valid GSTIN, configure the products' effective-dated taxes, and explicitly choose place of supply on bills. Do not change registration on your initial non-GST test business merely to force old invoices into a GST report.

In **Reports**, choose `gstr-1-preparation` or `gstr-3b-preparation` and the intended reporting dates. GSTR-1 preparation groups recorded outward invoice/credit-note lines by suggested section. GSTR-3B preparation summarizes recorded supplies and booked ITC candidates. Credit notes appear in their own posting period; party registration and tax details come from frozen invoice snapshots.

These are preparation worksheets, **not GST portal upload files or completed returns**. Review rows identify missing classification. ITC candidates need reconciliation and payment/eligibility review. Sections can overlap, so summing every row is incorrect. Amendments, advances, customs/ISD, certain special supplies, interest and settlement adjustments are not fully represented. Final compliance review remains outstanding.

### Large catalogs, returns and accounting

- Stock changes, Billing, receipts, dispatches, unit conversions, price lists, GST configuration, party balances, variant assignment and opening-stock warehouses have searchable, paginated master pickers. Use Find business in the sidebar to reach businesses beyond the initial list. Try more than 50 products and verify the selected record remains selected when searching another page.
- Try a partial return from Billing; preview the quantities/refund and verify stock/accounting after confirmation.
- Try a financial-journal CSV import: preview first, then post. Duplicate references or unbalanced entries reject the entire file.
- Recurring expenses require confirmation when paid. A due schedule alone does not post a payment.
- Year-end closing has a preview. Use only a disposable business: closing establishes a permanent accounting boundary.

## Test working-database encryption

The encrypted Linux build includes SQLCipher 4.19.0 and currently relies on this machine's OpenSSL/zlib runtime libraries. Its Windows packaging and independent security review are deferred. The database password is separate from an application login password and an encrypted-backup password. Keep it securely: there is no database-password recovery bypass.

Create a **new** encrypted test installation in Bash:

```bash
cd /home/aagam-linux/Desktop/Projects/MyBillBook
read -r -s -p 'Database password (at least 12 characters): ' ledgerleaf_db_password
printf '\n'
printf '%s\n' "$ledgerleaf_db_password" | ./release/ledgerleaf-linux-encrypted --data-dir ./encrypted-manual-test --database-password-stdin
unset ledgerleaf_db_password
```

The terminal must report **Working database encryption is active**. Use the same password/command every time you restart. The password is read from stdin, not passed as a command argument. Run the walkthrough above and restart with the correct password. A wrong password should fail to open the database; it must not silently start a new empty database. The standard executable rejects encryption mode instead of silently ignoring it.

To convert an existing **stopped test installation**, supply the password the same way but add `--encrypt-database`:

```bash
read -r -s -p 'New database password: ' ledgerleaf_db_password
printf '\n'
printf '%s\n' "$ledgerleaf_db_password" | ./release/ledgerleaf-linux-encrypted --data-dir ./manual-test-data --database-password-stdin --encrypt-database
unset ledgerleaf_db_password
```

Conversion exits after creating the encrypted working database. It prints the path holding the **plaintext original**. Previous backups and local backup snapshots remain plaintext. Protect these separately; use the existing encrypted-backup download feature when exporting protected copies. Conversion does not securely erase old files. Online restore in an unlocked encrypted installation re-encrypts the replacement database. Offline restore produces plaintext; run conversion afterward before restarting with encryption.

## Files and folders you will see

| Path in a data directory | Purpose |
| --- | --- |
| `data/billbook.db` | All businesses, users, transactions, settings, document snapshots and stored images/attachments. |
| `data/billbook.db-wal`, `data/billbook.db-shm` | SQLite transaction support files while running. Do not delete or copy these independently during operation. |
| `backups/` | Verified snapshots, imported backups and preserved originals from recovery/encryption conversion. |
| `logs/application.log` | Runtime messages; rotates at the configured size. |
| `settings/instance.lock` | Prevents two application processes using one installation simultaneously. |
| `attachments/`, `exports/` | Reserved installation directories; current expense attachments/media are stored in the database. Browser downloads normally go to your browser download directory. |

See [FILE_MAP.md](FILE_MAP.md) for the purpose of every project source/configuration/documentation file, and [usage.md](usage.md) for recovery commands.

## Rebuild after source changes

For this machine, with dependencies already installed:

```bash
cd /home/aagam-linux/Desktop/Projects/MyBillBook
npm run build --prefix frontend
GOTOOLCHAIN=local GOPROXY=off GOCACHE=/tmp/billbook-go-cache /tmp/billbook-toolchain/go/bin/go build -buildvcs=false -o release/ledgerleaf-linux ./cmd/billbook
```

Stop any running instance before replacing its executable. The frontend build must run before the Go build because Go embeds the compiled web files. You do not need to run a separate frontend development server.

To build the SQLCipher variant:

```bash
GO_BIN=/tmp/billbook-toolchain/go/bin/go GOCACHE=/tmp/billbook-go-cache ./scripts/build-linux-encrypted.sh
```

That script downloads a pinned, SHA-256-checked official source archive on the first run and compiles it locally. It requires GCC/make, Tcl, pkg-config, OpenSSL and zlib development headers, Node/npm and Go. It builds for the current Linux machine, not Windows.

## Automated checks, when wanted

The complete standard-build verification creates disposable databases and local servers:

```bash
GO_BIN=/tmp/billbook-toolchain/go/bin/go GOCACHE=/tmp/billbook-go-cache ./scripts/test-application.sh
```

Chrome must be installed and the environment must allow local sockets/browser subprocesses. This command is broader and slower than a normal startup; it does not run automatically when you launch Ledgerleaf. See [TEST_REPORT.md](TEST_REPORT.md) for the exact checks already performed and remaining verification.

## Troubleshooting and reporting a problem

| Symptom | What to check |
| --- | --- |
| Old screens or missing features | Use `release/ledgerleaf-linux`, rebuild frontend then backend if necessary, and open the latest printed URL. |
| Another instance may be using this directory | Stop the previous terminal's application or select a different test directory. Do not remove the lock to bypass a live instance. |
| Unauthorized or session expired | Open the complete launch URL from the running process and sign in again if passwords are enabled. |
| Empty business list after restart | Check that `--data-dir` points to the same folder. Relative paths depend on your terminal's working directory. |
| Insufficient stock | Finalize the purchase/opening stock first, check warehouse and units, and check for earlier dispatches. |
| Date predates existing movement / locked period | Use an appropriate posting date or inspect the business period controls. |
| Supplier state / place of supply required | Set party state and invoice place of supply explicitly for taxable purchases. |
| Encrypted database will not open | Use the SQLCipher executable, the same directory and the original database password. Preserve the files before attempting recovery. |
| Database cannot start | Preserve the complete data directory and use the documented verified-backup restore workflow. Automatic record reconstruction is not implemented. |

When reporting a problem, include the executable name, page/action, expected and actual result, exact error text, and minimal reproduction using fictional data. A screenshot is useful. Do not include passwords, launch-token URLs or real customer data. Record whether the issue reproduces after restarting the same test installation.

## Compact forms and value lists

Choose **+ Add business** to open setup in a dialog. Business name and state code are required; expand **Contact, bank and invoice details** for optional fields. The Businesses screen now searches and pages through business records, with Open and Edit actions.

Parties, products, new warehouses, opening stock, bill drafts/views, payments and returns also open in dialogs, keeping the underlying list in place. Close the dialog before navigating elsewhere. Escape or the top-right close button asks before discarding changed forms; explicit Cancel/Close actions dismiss the form. Errors appear inside the dialog so details can be corrected without starting over.

Master values use a compact dropdown for quick selection. Choose **Browse…** to search a longer list, page through matching results and select a record. Your current selection remains available when searching other values. In bill entry, expand **Tax, pricing and delivery options** for supply treatment, place of supply, inclusive pricing and related fields; inspect these before GST finalization. Dialogs scroll internally on small screens.

Configuration screens now open their forms on demand: **+ Configure tax**, **+ Add conversion**, **+ Add list price**, **+ Create group**, **+ Assign variant**, **Change financial year** and **+ Party transaction**. Journal entries offers separate actions for posting a journal, creating an account and reversing a posting. Recurring-expense New/Edit opens a dialog too. Successful saves return to the list; validation errors remain inside the form. Closing financial-year settings without saving leaves the saved-month summary unchanged.

## Help and interface language

Choose **Help** from the sidebar or page header for offline instructions. Search topics in English, Hindi or Gujarati. The header opens a topic related to the current screen where available; guide shortcuts respect your account's permissions.

Choose **Language / भाषा / ભાષા** in the sidebar or login screen. English is the default; Hindi and Gujarati are available for Help, navigation and common labels. Your browser remembers the choice. Some advanced instructions, statuses and server errors remain English. Business data, API codes, numbers and PDF/export output do not change with interface language. See [LOCALIZATION.md](LOCALIZATION.md) for scope and maintenance instructions.

## Entire database workbook and Excel import templates

As the local owner or an administrator, open **Backup & restore → Download database workbook (.xlsx)**. This exports all database tables across all businesses, including archived data. `_Guide` maps the numbered worksheet names to database table names and explains stored units. Each table has a header row even when empty. IDs preserve relationships; monetary values remain integer paise and quantities remain integer thousandths. Large integers are text to avoid Excel precision loss. NULL and empty text both display as blank.

The workbook is unencrypted and includes account details and user password hashes. Store it privately. Images, attachments, very long text and XML-unsafe values are represented by `@base64:` references. Join the matching `_LargeValues` chunks in part order and base64-decode them to recover the original bytes. Excel's worksheet limits apply: an oversized export fails instead of silently omitting records. The download is prepared completely before being sent, from one consistent read transaction; temporary files are removed afterwards.

This is an inspection export, not a restore format. Use the existing SQLite backup/restore workflow to recover the whole application. Do not upload this workbook to a record importer.

To load new records, open **Import & export**, choose Products or Customers / suppliers, and download the blank Excel template. Keep the English header names, fill one record per row, save as `.xlsx`, select the file, and choose **Import new records**. Set SKU/phone/GSTIN columns to Text in Excel to preserve leading zeros. Use `PRODUCT` / `SERVICE` or `CUSTOMER` / `SUPPLIER` as appropriate. ₹100 is `10000` paise; 1 unit is `1000` thousandths. Formulas are rejected; paste their values first. Imports add records without overwriting existing ones; any invalid row rolls back the entire import. Invoice and journal imports have their own templates, previews and validation farther down the same page.


## Manage accounts without a long setup page

Open **Users & permissions → Add account**, or choose **Edit account** beside an existing username. Forms open in a dialog. Select a non-administrator role and enable **Limit access to selected businesses** to search and page through businesses. Selected IDs remain selected when you change the search or page. **Review selected business IDs** lets you remove hidden selections. Up to 200 businesses can be assigned to one restricted account, including businesses beyond the first 200 in the directory. Save commits the complete selected set. Administrators always have installation-wide access.

Use **Find account** to filter usernames. **Change your password** opens a separate dialog and requires the current password. Successful account/password changes retain the existing session-invalidation rules.


## Bank account and statement dialogs

In **Banks & periods**, choose **Add bank account** to enter the code and bank details in a dialog. Saving closes the dialog; rejected entries stay open with their values and error message. In the reconciliation table, choose **Match** beside a ledger entry to enter its statement date/reference. Matching does not create or alter a ledger posting. **Unmatch** clears only the statement association. Period locks and year-end closing keep their existing workflows.

For the latest 2026-10-22 bank-dialog and login-language changes, the available Linux **development** build is:

```sh
./release/ledgerleaf-linux-dev --data-dir ./manual-test-data
```

Use a fresh test directory. This build uses the locally installed Go 1.27.2 compiler; the older standard/encrypted release executables were preserved because their pinned build toolchains were unavailable. This is not a release or Windows compatibility claim.

Expenses and recurring-expense schedules can now use **Cash**, the default bank, or a named bank created in **Banks & periods**. Choose the account under **Payment account** or **Recurring payment account**. Each confirmed payment credits that specific account, making it available for statement matching. Accounts from another business and non-bank ledger accounts are rejected. Scheduled expenses still require explicit payment confirmation.

## Compact goods receipt and dispatch workflows

Open **Goods receipts → New goods receipt** to receive against an issued purchase order or directly from a supplier. Posting closes the dialog and refreshes the list. **Choose quantities** / **Match selected receipts** open a quantity-matching dialog; **Correct receipt** opens a reversal dialog. Existing stock, invoice and correction rules still apply.

Open **Dispatch challans → New dispatch challan** to choose the customer, warehouse and items. Successful posting returns to the dispatch list. Failed validation keeps the form open. The list scrolls horizontally when needed on small screens; dialogs scroll internally. Closing a changed form with its top-right close control or Escape asks before discarding it. Dispatch allocation/history and consolidation also use dialogs, as described below.

Dispatch **Invoice**, **Return**, **History**, and **Combine selected dispatches** actions now open dialogs. Quantity/date changes stay inside the dialog until submitted. Negative quantities are rejected explicitly; zero leaves an item for later. Top-right close/Escape asks before discarding changed input; the explicit Close action discards it. Tables scroll horizontally inside the dialog on narrow screens. Successful invoice-draft creation or return closes the dialog and refreshes the dispatch list.
