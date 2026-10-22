CREATE TABLE businesses (
 id INTEGER PRIMARY KEY,
 name TEXT NOT NULL CHECK(length(trim(name)) BETWEEN 1 AND 160),
 legal_name TEXT NOT NULL DEFAULT '',
 gstin TEXT NOT NULL DEFAULT '',
 state_code TEXT NOT NULL CHECK(length(state_code)=2),
 address TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE TABLE audit_log (
 id INTEGER PRIMARY KEY,
 business_id INTEGER REFERENCES businesses(id),
 action TEXT NOT NULL,
 entity_id INTEGER NOT NULL,
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX audit_business ON audit_log(business_id,id);
