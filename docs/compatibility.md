# Compatibility investigation

Current Linux candidate: Go 1.27.3, Linux amd64, statically linked with native SQLite extension loading and the system DNS resolver excluded. RC3 passed the full automated workflow/recovery suites and extracted-package checks on this host. It is not Windows runtime evidence; see [READINESS.md](READINESS.md).

Legacy Windows toolchain: Go 1.20.14. The historical Windows 7/8 compatibility decision uses Go 1.20 (https://go.dev/doc/go1.20); it does not pin the current Linux release to that old compiler. All legacy Windows runtime and browser qualification below remains pending.

SQLite: github.com/mattn/go-sqlite3 v1.14.22, requiring Go 1.19 (https://raw.githubusercontent.com/mattn/go-sqlite3/v1.14.22/go.mod). SQLite's amalgamation is compiled into each executable. CGO and MinGW are build-time requirements only. The cached modernc.org/sqlite v1.28.0 pure-Go driver has Windows amd64 and arm64 implementations but no Windows 386 implementation, so it cannot meet the complete target matrix. Verify the PE import table after every release.

Frontend: React/React DOM 18.3.1, TypeScript 4.9.5, Vite 5.4.21, Tailwind CSS 3.4.17. Exact dependency graph in frontend/package-lock.json. Node is a build-time dependency only. Target browser is Chromium 109 or newer; IE is unsupported. Chrome 109 is the final Chrome release for Windows 7/8/8.1 (https://support.google.com/chrome/a/answer/7100626?hl=en). No polyfills are needed for APIs currently used at this target. The executable does not bundle a browser; a suitable existing browser is required.

These older OS, browser and toolchain releases are unsupported and carry security maintenance risks. Loopback binding reduces network exposure but is not an OS security substitute. No claim of current security support or Windows runtime compatibility is made from compilation alone.

| Target | Compilation | Startup / workflows |
|---|---|---|
| Linux amd64 (development host) | Passed | Passed: startup, persistence, API and Chrome E2E |
| Windows 7 SP1 x86 / x64 | Cross-compiled | Unverified: no Windows VM available |
| Windows 8.1 x86 / x64 | Cross-compiled | Unverified: no Windows VM available |
| Windows 10 x64 | Cross-compiled | Unverified: no Windows VM available |
| Windows 11 x64 | Cross-compiled | Unverified: no Windows VM available |

The 2 GB RAM target, clean-machine/offline operation on Windows, physical printing, Chrome 109 behavior and large datasets remain unverified. Do not distribute as a production release until these gates pass.

PDF dependency: gofpdf v1.16.2, compiled successfully with Go 1.20.14. Font resources are embedded. Current PE imports are KERNEL32.dll and msvcrt.dll only; final build evidence is under release/imports-*.txt. These imports do not prove API-level compatibility with Windows 7.
