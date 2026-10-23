# Running Ledgerleaf

This package is for a single computer running 64-bit Linux with a current Chromium-based browser. No Go, Node, Python, database server or internet connection is needed at runtime. Windows and older-browser support require separate qualification. Check READINESS.md before deploying for real business records.

## Install and start

Extract the archive into a folder owned by your normal desktop account. Run `sha256sum -c SHA256SUMS` inside it. Keep the executable and data on a local disk; avoid shared network folders and live cloud-sync folders for the working database.

Run `./ledgerleaf`. It opens the browser and stores data in your user configuration directory under `BillBook`. For an explicit location, use `./ledgerleaf --data-dir /absolute/path/to/LedgerleafData`. Keep that path the same after upgrades. `./ledgerleaf --version` identifies the build without opening any database.

Keep the terminal open. Closing a browser tab leaves the application running. Use **Exit application**, Ctrl+C or SIGTERM for a normal shutdown. Only one process can use a data directory at a time. If the browser does not open, paste the printed launch link into your browser. Treat that link as a password; do not share it or expose the application through a proxy or public port.

## First-use checklist

1. Create your business and check its address, tax scheme, currency amounts and invoice details.
2. Create an administrator in **Users & permissions**, then sign in. Before users exist, possession of the local launch link grants owner access. Give each additional operator their own account and the appropriate business access.
3. Add customers, suppliers and products. Enter opening stock before selling stocked products.
4. In **Backup & restore**, enable daily automatic backups and choose retention. Automatic backups are currently opt-in. Also save a verified backup outside this computer; snapshots on the same disk do not protect against disk loss.
5. Create a small trial invoice, use **Save & review**, confirm its amount and finalize it. Check the **Download PDF** output. PNG export is secondary and saves one image per page for long documents.
6. Check a payment, stock balance and report before importing historical records. Finalized records use explicit returns/reversals rather than destructive edits.

## Backup and restore

Backups include every business and user in the installation. Store them securely. The standard working database and local snapshots are plaintext; protect the computer with OS account permissions and disk encryption. Password-encrypted backup downloads are available. They do not encrypt the working installation.

Verify downloaded backups and rehearse a restore into a separate trial installation before depending on them. Restoring replaces all businesses and users, including their passwords. Retain an independent copy of the original installation before recovery.

If startup fails, stop the application and preserve the **whole** data directory, including any `-wal` and `-shm` files. Do not delete files to force startup. Use a known-good verified snapshot:

```sh
./ledgerleaf --data-dir /absolute/path/to/LedgerleafData \
  --restore-backup /absolute/path/to/snapshot.db --confirm-restore
```

For an encrypted downloaded backup, add `--restore-encrypted` and supply its password as one line on standard input. Never put passwords in command-line arguments. Offline recovery retains original database files under `backups/pre-offline-restore-*`. Very old schema snapshots may require their matching older application; do not bypass backup validation.

## Upgrade and support

Stop Ledgerleaf and save an external verified backup. Replace the executable while keeping the same data directory. Start it and check recent invoices, payments, stock, reports and a PDF. Do not run an older executable against an upgraded schema.

Startup checks SQLite structural integrity before migration. Before upgrading an existing versioned schema, it writes a private standalone snapshot under `data/pre-upgrade/schema-OLD-to-NEW-*/billbook.db`. Committed WAL transactions are included. Snapshot failure stops the upgrade; failed migrations roll back and retain the snapshot. Working-database encryption is preserved in keyed builds. These safety snapshots are not pruned automatically and are separate from regular backups. Linux flushes the snapshot and its directory entries before migration; Windows directory durability remains unverified. A structural check is not an accounting audit or a guarantee against hardware power loss.

For support, record `--version`, the OS/browser versions, the failed action and relevant entries from `logs/application.log`. Review logs before sharing them. Share business data or backup files only deliberately with someone authorized to see all businesses and users. Logs rotate at 5 MiB with three older copies.
