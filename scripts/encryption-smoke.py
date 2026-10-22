#!/usr/bin/env python3
"""Linux CLI smoke check; only disposable installations and local HTTP are used."""
import http.cookiejar
import json
import pathlib
import re
import select
import signal
import subprocess
import tempfile
import time
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[1]
PASSWORD = "test-only passphrase &=?#'"


def start(executable, folder, encrypted=False):
    command = [str(ROOT / "release" / executable), "--data-dir", str(folder), "--no-browser"]
    if encrypted:
        command.append("--database-password-stdin")
    process = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, bufsize=1)
    if encrypted:
        process.stdin.write(PASSWORD + "\n")
        process.stdin.flush()
    deadline = time.monotonic() + 15
    try:
        # Read bytes to avoid TextIO buffering hiding an already-read URL line.
        output = b""
        while time.monotonic() < deadline:
            readable, _, _ = select.select([process.stdout], [], [], 0.2)
            if readable:
                import os
                chunk = os.read(process.stdout.fileno(), 4096)
                output += chunk
                match = re.search(rb"(http://127\.0\.0\.1:\d+)/#launch=([a-f0-9]+)", output)
                if match:
                    origin, token = (part.decode() for part in match.groups())
                    opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
                    def call(path, body=None):
                        request = urllib.request.Request(origin + "/api/" + path, data=None if body is None else json.dumps(body).encode(), headers={"Origin": origin, "X-Ledgerleaf": "1", "Content-Type": "application/json"})
                        with opener.open(request, timeout=15) as response:
                            return json.load(response)
                    call("session", {"token": token})
                    return process, call
            if process.poll() is not None:
                raise RuntimeError("Application failed: " + process.stderr.read())
        raise RuntimeError("Startup timed out")
    except Exception:
        stop(process)
        raise


def stop(process):
    if process.poll() is None:
        process.send_signal(signal.SIGINT)
        try:
            process.wait(timeout=15)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
    for stream in (process.stdin, process.stdout, process.stderr):
        stream.close()


with tempfile.TemporaryDirectory(prefix="ledgerleaf-encryption-smoke-") as temporary:
    for convert in (False, True):
        folder = pathlib.Path(temporary) / ("converted" if convert else "new")
        process, call = start("ledgerleaf-linux" if convert else "ledgerleaf-linux-encrypted", folder, not convert)
        try:
            call("businesses", {"name": "Cipher smoke marker", "state_code": "27"})
        finally:
            stop(process)
        if convert:
            result = subprocess.run([str(ROOT / "release/ledgerleaf-linux-encrypted"), "--data-dir", str(folder), "--encrypt-database", "--database-password-stdin"], input=PASSWORD + "\n", text=True, capture_output=True, timeout=30)
            assert result.returncode == 0, result.stderr
            assert list((folder / "backups").glob("pre-encryption-*/billbook.db"))
        database = folder / "data" / "billbook.db"
        before = database.read_bytes()
        assert not before.startswith(b"SQLite format 3") and b"Cipher smoke marker" not in before
        bad = subprocess.run([str(ROOT / "release/ledgerleaf-linux-encrypted"), "--data-dir", str(folder), "--database-password-stdin", "--no-browser"], input="wrong-password-12345\n", text=True, capture_output=True, timeout=15)
        assert bad.returncode != 0 and database.read_bytes() == before
        process, call = start("ledgerleaf-linux-encrypted", folder, True)
        try:
            assert call("businesses")[0]["name"] == "Cipher smoke marker"
        finally:
            stop(process)
    unsupported = pathlib.Path(temporary) / "unsupported"
    bad = subprocess.run([str(ROOT / "release/ledgerleaf-linux"), "--data-dir", str(unsupported), "--database-password-stdin"], input=PASSWORD + "\n", text=True, capture_output=True, timeout=15)
    assert bad.returncode != 0 and not (unsupported / "data" / "billbook.db").exists()
print("PASS: encrypted startup/restart, wrong-password preservation, conversion, plaintext-original retention, and standard-build rejection")
