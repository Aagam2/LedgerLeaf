# Architecture

Go 1.20.14 starts one net/http server on an automatically selected IPv4 loopback port. It embeds compiled React assets, SQL migrations and the PDF font. SQLite is compiled into the executable using CGO. No online service is contacted at runtime.

The browser receives a 256-bit random capability in the launch URL fragment. The UI removes the fragment and exchanges the capability for an HttpOnly, SameSite=Strict session cookie. Each API request checks the exact Host; writes also require the exact Origin and a custom header. Fragments are not sent to the HTTP server. The capability changes on restart. Before the first administrator is created, this grants local-owner access. After setup, an additional 12-hour user-session cookie is required; bcrypt cost 12 protects passwords and the server enforces ADMIN, ACCOUNTANT, BILLER and VIEWER roles. User deactivation/password changes revoke sessions; roles and active state are checked against SQLite on each request. Login attempts are throttled after five failures. Users are installation-wide. Local files remain protected by OS permissions; the standard build stores plaintext SQLite. An optional SQLCipher Linux build supports password-keyed working databases; local snapshots and preserved originals remain plaintext. See TESTING_GUIDE.md.

An OS-held file handle enforces a single application instance per data directory. Windows uses an exclusive CreateFile handle; Unix uses flock. Locks release on process exit, so no stale lock-file deletion is needed. The data path is normalized to an absolute path. SQLite independently serializes writers; a five-second busy timeout bounds waits.

Modules: `database` handles transactional migrations; `server` implements validation and business transactions; `accounting` checks balanced posting; `money` rounds integer rational arithmetic; `printing` renders PDF from stored snapshots; `instance` handles startup locks; `web` embeds static frontend assets.

Closing the browser does not stop the executable. The authenticated Exit command and Ctrl+C trigger a graceful HTTP drain, followed by closing SQLite, log file and instance lock. Logs do not contain launch tokens or business payloads. Log rotation and crash recovery UX remain pending.

The UI hides unavailable operational modules and explicitly describes the current non-GST limitation. It does not substitute mock dashboard values. Multiple business isolation is enforced by scoped queries and composite foreign keys for related business records.
