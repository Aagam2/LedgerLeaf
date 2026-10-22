CREATE TABLE backup_policy (
 id INTEGER PRIMARY KEY CHECK(id=1),
 enabled INTEGER NOT NULL DEFAULT 0 CHECK(enabled IN (0,1)),
 interval_hours INTEGER NOT NULL DEFAULT 24 CHECK(interval_hours BETWEEN 1 AND 720),
 retain_count INTEGER NOT NULL DEFAULT 7 CHECK(retain_count BETWEEN 1 AND 100),
 last_success TEXT NOT NULL DEFAULT '',
 last_error TEXT NOT NULL DEFAULT ''
);
INSERT INTO backup_policy(id) VALUES(1);
