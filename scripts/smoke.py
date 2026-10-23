"""Exercise the compiled application using disposable SQLite data and loopback only.
Python is a development test dependency, never a runtime dependency of BillBook.
"""
import http.cookiejar
import json
import os
import pathlib
import queue
import sqlite3
import subprocess
import tempfile
import threading
import urllib.error
import urllib.request

BINARY = pathlib.Path(os.environ.get('LEDGERLEAF_TEST_BINARY') or pathlib.Path(__file__).resolve().parents[1] / 'release' / 'ledgerleaf-linux').resolve()


def start(data):
    process = subprocess.Popen(
        [str(BINARY), '--no-browser', '--data-dir', data],
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
    )
    output = queue.Queue()

    def read_output():
        for line in process.stdout:
            if '#launch=' in line:
                output.put(line.strip())
                return
        output.put(None)

    threading.Thread(target=read_output, daemon=True).start()
    try:
        launch = output.get(timeout=15)
        if launch is None:
            raise RuntimeError(process.stderr.read())
    except BaseException:
        process.kill()
        process.wait(timeout=10)
        raise
    base, token = launch.split('/#launch=')
    client = urllib.request.build_opener(
        urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())
    )

    def request(path, body=None):
        req = urllib.request.Request(
            base + path,
            data=None if body is None else json.dumps(body).encode(),
            headers={'Origin': base, 'Content-Type': 'application/json', 'X-Ledgerleaf': '1'},
        )
        with client.open(req, timeout=10) as response:
            return response.read()

    request('/api/session', {'token': token})
    assert b'Ledgerleaf' in request('/')
    assert b'third-party notices' in request('/third-party-notices.txt')
    return process, request, token


def main():
    info = subprocess.run([str(BINARY), '--version'], capture_output=True, text=True, timeout=10)
    assert info.returncode == 0 and info.stdout.startswith('Ledgerleaf '), info.stderr
    with tempfile.TemporaryDirectory(prefix='ledgerleaf-smoke-') as data:
        process = None
        try:
            process, request, old_token = start(data)
            def post(path, body):
                return json.loads(request('/api/' + path, body))

            business = post('businesses', {'name': 'Smoke business', 'state_code': '27'})
            assert business['id'] == 1
            post('parties?business_id=1', {'name': 'Buyer', 'kind': 'CUSTOMER'})
            post('products?business_id=1', {
                'name': 'Tea', 'sku': 'TEA', 'kind': 'PRODUCT', 'unit': 'PCS', 'sale_price': 10000,
            })
            post('opening-stock?business_id=1', {
                'product_id': 1, 'warehouse_id': 1, 'quantity': 10000, 'value': 60000,
                'date': '2026-10-15', 'key': 'smoke-opening-key',
            })
            bill = post('documents?business_id=1', {
                'kind': 'SALE', 'party_id': 1, 'warehouse_id': 1,
                'date': '2026-10-15', 'due_date': '2026-10-20', 'key': 'smoke-invoice-key',
                'lines': [{'product_id': 1, 'quantity': 2000, 'price': 10000}],
            })
            post('finalize?business_id=1', {'id': bill['id']})
            post('payments?business_id=1', {
                'document_id': bill['id'], 'amount': 5000, 'date': '2026-10-15',
                'mode': 'CASH', 'key': 'smoke-payment-key',
            })
            duplicate = subprocess.run(
                [str(BINARY), '--no-browser', '--data-dir', data],
                capture_output=True, text=True, timeout=10,
            )
            assert duplicate.returncode != 0 and 'another instance' in duplicate.stderr

            # Deliberately bypass graceful shutdown after committed financial operations.
            process.kill()
            process.wait(timeout=10)
            process, request, new_token = start(data)
            assert new_token != old_token
            try:
                request('/api/session', {'token': old_token})
                raise AssertionError('Old launch token still accepted')
            except urllib.error.HTTPError as error:
                assert error.code == 401
            rows = json.loads(request('/api/documents?business_id=1'))
            assert len(rows) == 1
            assert rows[0]['total'] == 20000 and rows[0]['paid'] == 5000
            stock = json.loads(request('/api/stock?business_id=1'))
            assert stock[0]['quantity'] == 8000 and stock[0]['value'] == 48000
            assert request('/api/document-pdf?business_id=1&id=1').startswith(b'%PDF-')
            request('/api/exit', {})
            assert process.wait(timeout=20) == 0

            # Read the isolated fixture database after shutdown and reconcile ledgers.
            with sqlite3.connect(pathlib.Path(data) / 'data' / 'billbook.db') as db:
                assert db.execute('PRAGMA integrity_check').fetchone() == ('ok',)
                assert db.execute('PRAGMA foreign_key_check').fetchall() == []
                assert db.execute('SELECT SUM(debit-credit) FROM journal_lines').fetchone() == (0,)
                assert db.execute("SELECT SUM(debit-credit) FROM journal_lines WHERE account='1100'").fetchone() == (15000,)

            process, request, _ = start(data)
            assert json.loads(request('/api/businesses'))[0]['name'] == 'Smoke business'
            # Service managers terminate Unix processes with SIGTERM. This must
            # drain requests and release the instance lock just like UI Exit.
            if os.name != 'nt':
                process.terminate()
                assert process.wait(timeout=20) == 0
                process, request, _ = start(data)
                assert json.loads(request('/api/documents?business_id=1'))[0]['paid'] == 5000
            request('/api/auth/users', {'username': 'owner', 'password': 'Original-Password-123!', 'role': 'ADMIN', 'active': 1})
            locked = subprocess.run([str(BINARY), '--data-dir', data, '--recover-admin', 'owner'], capture_output=True, text=True, timeout=10)
            assert locked.returncode != 0 and 'another instance' in locked.stderr
            request('/api/exit', {})
            assert process.wait(timeout=20) == 0
            recovered = subprocess.run([str(BINARY), '--data-dir', data, '--recover-admin', 'owner'], capture_output=True, text=True, timeout=20)
            assert recovered.returncode == 0, recovered.stderr
            temporary_password = next(line.split(': ', 1)[1] for line in recovered.stdout.splitlines() if line.startswith('Temporary password: '))
            process, request, _ = start(data)
            try:
                request('/api/auth/login', {'username': 'owner', 'password': 'Original-Password-123!'})
                raise AssertionError('Old password accepted after recovery')
            except urllib.error.HTTPError as error:
                assert error.code == 401
            request('/api/auth/login', {'username': 'owner', 'password': temporary_password})
            assert json.loads(request('/api/documents?business_id=1'))[0]['paid'] == 5000
            request('/api/auth/password', {'current': temporary_password, 'password': 'Changed-Password-123!'})
            request('/api/backup-policy', {'enabled': True, 'interval_hours': 24, 'retain_count': 2})
            request('/api/exit', {})
            assert process.wait(timeout=20) == 0
            # A due policy runs at the next launch, even before the first login.
            process, request, _ = start(data)
            request('/api/auth/login', {'username': 'owner', 'password': 'Changed-Password-123!'})
            policy = json.loads(request('/api/backup-policy'))
            assert policy['last_success'] and not policy['last_error']
            assert any(b['name'].startswith('Auto-') for b in json.loads(request('/api/backups')))
            snapshot = json.loads(request('/api/backups', {}))['name']
            backup_password = 'Smoke-Encrypted-Backup-123!'
            encrypted = pathlib.Path(data) / 'recovery.llbenc'
            encrypted.write_bytes(request('/api/backup-encrypt', {'name': snapshot, 'password': backup_password}))
            request('/api/exit', {})
            assert process.wait(timeout=20) == 0
            database_path = pathlib.Path(data) / 'data' / 'billbook.db'
            original = database_path.read_bytes()
            restore_command = [str(BINARY), '--data-dir', data, '--restore-backup', str(encrypted), '--restore-encrypted', '--confirm-restore']
            wrong = subprocess.run(restore_command, input='Incorrect-Password-123!\n', capture_output=True, text=True, timeout=20)
            assert wrong.returncode != 0 and 'incorrect password' in wrong.stderr
            assert database_path.read_bytes() == original
            # Exercise the CLI when the installed database cannot be opened.
            database_path.write_bytes(b'corrupt disposable smoke fixture')
            restored = subprocess.run(restore_command, input=backup_password+'\n', capture_output=True, text=True, timeout=20)
            assert restored.returncode == 0, restored.stderr
            preserved = list((pathlib.Path(data) / 'backups').glob('pre-offline-restore-*/billbook.db'))
            assert any(p.read_bytes() == b'corrupt disposable smoke fixture' for p in preserved)
            process, request, _ = start(data)
            request('/api/auth/login', {'username': 'owner', 'password': 'Changed-Password-123!'})
            assert json.loads(request('/api/documents?business_id=1'))[0]['paid'] == 5000
            assert json.loads(request('/api/diagnostics', {}))['status'] == 'pass'
            request('/api/exit', {})
            assert process.wait(timeout=20) == 0
        finally:
            if process and process.poll() is None:
                process.kill()
                process.wait(timeout=10)
    print('PASS: embedded UI/resources, local session, full sale/partial payment, duplicate instance rejection, crash recovery, token rotation, PDF, SQLite integrity, balanced ledgers, graceful restart, offline administrator recovery, scheduled backup on launch, encrypted offline CLI restore with wrong-password rejection and original-file preservation')


if __name__ == '__main__':
    main()
