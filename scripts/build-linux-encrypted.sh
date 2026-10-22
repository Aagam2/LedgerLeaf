#!/usr/bin/env bash
# Native Linux SQLCipher build. Requires Go 1.20.14, GCC/make, Tcl, pkg-config,
# OpenSSL development headers and zlib development headers. No database is opened.
set -euo pipefail
cd "$(dirname "$0")/.."
GO_BIN=${GO_BIN:-go}
cipher_version=4.19.0
cipher_sha=7075f96cbabe45b4ecfc2e6b1745a625f856f695b0827a5506ce9ed85b906aa0
cipher_cache=${SQLCIPHER_BUILD_DIR:-/tmp/ledgerleaf-sqlcipher-build-4.19.0}
mkdir -p "$cipher_cache" release
archive="$cipher_cache/source.tar.gz"
if [[ ! -f "$archive" ]]; then
 curl --fail --location --proto '=https' "https://github.com/sqlcipher/sqlcipher/archive/refs/tags/v${cipher_version}.tar.gz" -o "$archive"
fi
printf '%s  %s\n' "$cipher_sha" "$archive" | sha256sum --check
mkdir -p "$cipher_cache/source"
tar -xzf "$archive" -C "$cipher_cache/source" --strip-components=1
(
 cd "$cipher_cache/source"
 ./configure --with-tempstore=yes --disable-shared --enable-static CFLAGS='-O2 -DSQLITE_HAS_CODEC -DSQLITE_EXTRA_INIT=sqlcipher_extra_init -DSQLITE_EXTRA_SHUTDOWN=sqlcipher_extra_shutdown' LDFLAGS='-lcrypto'
 make -j2 libsqlite3.a
)
npm run build --prefix frontend
CGO_ENABLED=1 CGO_CFLAGS="-I$cipher_cache/source" CGO_LDFLAGS="-L$cipher_cache/source -lcrypto -lm -lz" "$GO_BIN" build -tags libsqlite3 -buildvcs=false -o release/ledgerleaf-linux-encrypted ./cmd/billbook
cp "$cipher_cache/source/LICENSE.md" release/SQLCipher-LICENSE.md
printf 'Built release/ledgerleaf-linux-encrypted (SQLCipher %s). OpenSSL/zlib runtime libraries are required.\n' "$cipher_version"
