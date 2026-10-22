"""Reproducible large-data benchmark. Uses only disposable SQLite data.
Seeding bypasses API validation to construct volume, then measurements use the real app.
Python is a developer tool, not part of distribution. Not a Windows/2 GB certification.
"""
import json
import pathlib
import platform
import signal
import sqlite3
import statistics
import tempfile
import time
from smoke import start

ROOT = pathlib.Path(__file__).resolve().parents[1]

def main():
    with tempfile.TemporaryDirectory(prefix='ledgerleaf-benchmark-') as data:
        process, request, _ = start(data)
        request('/api/businesses', {'name': 'Benchmark only', 'state_code': '27'})
        process.send_signal(signal.SIGINT)
        process.wait(timeout=15)
        path = next((pathlib.Path(data) / 'data').glob('*.db'))
        began = time.perf_counter()
        db = sqlite3.connect(path)
        db.execute('PRAGMA foreign_keys=ON')
        db.execute('PRAGMA journal_mode=WAL')
        with db:
            db.executemany('INSERT INTO parties(id,business_id,kind,name) VALUES(?,1,?,?)',
                           ((i, 'CUSTOMER' if i <= 5000 else 'SUPPLIER', f'Party {i:05d}') for i in range(1, 7001)))
            db.executemany("INSERT INTO products(id,business_id,name,sku,kind,unit,sale_price) VALUES(?,1,?,?,'PRODUCT','PCS',1500)",
                           ((i, f'Product {i:05d}', f'SKU{i:05d}') for i in range(1, 10001)))
            db.execute("INSERT INTO journals(id,business_id,date,reference,description) VALUES(1,1,'2026-01-01','benchmark-opening','Benchmark initial stock')")
            db.executemany('INSERT INTO journal_lines(business_id,journal_id,account,debit,credit) VALUES(1,1,?,?,?)',
                           [('1200', 1000000000, 0), ('3000', 0, 1000000000)])
            db.executemany("INSERT INTO stock_movements(business_id,product_id,warehouse_id,quantity,value,kind,reference,date) VALUES(1,?,1,2000,2000,'ADD',?,'2026-01-01')",
                           ((i % 10000 + 1, f'benchmark-add:{i}') for i in range(500000)))
            for base in range(0, 100000, 1000):
                documents, journals, ledger, movements = [], [], [], []
                for n in range(base + 1, base + 1001):
                    lines = []
                    for j in range(5):
                        product = ((n - 1) * 5 + j) % 10000 + 1
                        lines.append({'product_id': product, 'name': f'Product {product:05d}', 'sku': f'SKU{product:05d}', 'unit': 'PCS', 'kind': 'PRODUCT', 'quantity': 1000, 'price': 1500, 'discount': 0, 'amount': 1500, 'cost': 1000})
                        movements.append((product, f'document:{n}'))
                    party = (n - 1) % 5000 + 1
                    snapshot = {'id': n, 'kind': 'SALE', 'status': 'FINALIZED', 'party_id': party, 'warehouse_id': 1, 'date': '2026-10-15', 'due_date': '2026-10-30', 'number': f'B/{n:08d}', 'total': 7500, 'business_name': 'Benchmark only', 'party_name': f'Party {party:05d}', 'lines': lines}
                    documents.append((n, party, snapshot['number'], json.dumps(snapshot, separators=(',', ':'))))
                    journals.append((n + 1, f'document:{n}', snapshot['number']))
                    ledger.extend([(n + 1, '1100', 7500, 0), (n + 1, '4000', 0, 7500), (n + 1, '5000', 5000, 0), (n + 1, '1200', 0, 5000)])
                db.executemany("INSERT INTO documents(id,business_id,kind,status,party_id,warehouse_id,date,due_date,number,total,snapshot) VALUES(?,1,'SALE','FINALIZED',?,1,'2026-10-15','2026-10-30',?,7500,?)", documents)
                db.executemany("INSERT INTO journals(id,business_id,date,reference,description) VALUES(?,1,'2026-10-15',?,?)", journals)
                db.executemany('INSERT INTO journal_lines(business_id,journal_id,account,debit,credit) VALUES(1,?,?,?,?)', ledger)
                db.executemany("INSERT INTO stock_movements(business_id,product_id,warehouse_id,quantity,value,kind,reference,date) VALUES(1,?,1,-1000,-1000,'SALE',?,'2026-10-15')", movements)
        assert db.execute('PRAGMA integrity_check').fetchone()[0] == 'ok'
        assert not db.execute('PRAGMA foreign_key_check').fetchall()
        assert db.execute('SELECT SUM(debit-credit) FROM journal_lines').fetchone()[0] == 0
        assert db.execute('SELECT SUM(value) FROM stock_movements').fetchone()[0] == 500000000
        assert db.execute("SELECT SUM(debit-credit) FROM journal_lines WHERE account='1200'").fetchone()[0] == 500000000
        db.execute('ANALYZE')
        db.execute('PRAGMA wal_checkpoint(TRUNCATE)')
        db.close()
        seeded = time.perf_counter() - began
        began = time.perf_counter()
        process, request, _ = start(data)
        startup = time.perf_counter() - began
        try:
            timings = {}
            routes = {'product_search': 'products?business_id=1&q=SKU09999', 'customer_search': 'parties?business_id=1&q=Party%2004999', 'invoice_page': 'documents?business_id=1', 'invoice_deep_page': 'documents?business_id=1&page=2000', 'stock_page': 'stock?business_id=1', 'trial_balance': 'reports?business_id=1&kind=trial-balance', 'profit_loss': 'reports?business_id=1&kind=profit-loss', 'dashboard': 'dashboard?business_id=1', 'monthly_revenue': 'dashboard?business_id=1&series=monthly'}
            for label, route in routes.items():
                samples = []
                for _ in range(3):
                    began = time.perf_counter()
                    response = json.loads(request('/api/' + route))
                    assert isinstance(response, list)
                    samples.append(round((time.perf_counter() - began) * 1000, 2))
                timings[label] = {'samples_ms': samples, 'median_ms': statistics.median(samples)}
                print(label, timings[label], flush=True)
            status = pathlib.Path(f'/proc/{process.pid}/status').read_text()
            memory = {line.split(':')[0]: line.split(':')[1].strip() for line in status.splitlines() if line.startswith(('VmRSS:', 'VmHWM:'))}
            result = {'platform': platform.platform(), 'counts': {'products': 10000, 'customers': 5000, 'suppliers': 2000, 'invoices': 100000, 'invoice_lines': 500000, 'stock_movements': 1000000}, 'database_bytes': path.stat().st_size, 'seed_and_integrity_seconds': round(seeded, 2), 'startup_seconds': round(startup, 3), 'backend_memory': memory, 'queries': timings, 'limits': 'Linux host, warm-cache query samples; no browser memory, Windows, printer, 2 GB hardware or cold-disk certification.'}
            (ROOT / 'release' / 'benchmark.json').write_text(json.dumps(result, indent=2) + '\n')
            print('PASS: full requested data volume, integrity, ledger/stock reconciliation and real API measurements', flush=True)
        finally:
            process.send_signal(signal.SIGINT)
            process.wait(timeout=15)

if __name__ == '__main__':
    main()
