CREATE TABLE documents (
 id INTEGER PRIMARY KEY,
 business_id INTEGER NOT NULL REFERENCES businesses(id),
 kind TEXT NOT NULL CHECK(kind IN ('SALE','PURCHASE')),
 status TEXT NOT NULL DEFAULT 'DRAFT' CHECK(status IN ('DRAFT','FINALIZED','CANCELLED')),
 party_id INTEGER NOT NULL,
 warehouse_id INTEGER NOT NULL,
 date TEXT NOT NULL,
 due_date TEXT NOT NULL,
 number TEXT,
 total INTEGER NOT NULL CHECK(total>=0),
 snapshot TEXT NOT NULL,
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
 UNIQUE(business_id,id),
 UNIQUE(business_id,number),
 FOREIGN KEY(business_id,party_id) REFERENCES parties(business_id,id),
 FOREIGN KEY(business_id,warehouse_id) REFERENCES warehouses(business_id,id)
);
CREATE INDEX document_list ON documents(business_id,kind,date,id);
CREATE TABLE counters (
 business_id INTEGER NOT NULL REFERENCES businesses(id),
 series TEXT NOT NULL,
 next INTEGER NOT NULL,
 PRIMARY KEY(business_id,series)
);
CREATE TABLE payments (
 id INTEGER PRIMARY KEY,
 business_id INTEGER NOT NULL,
 document_id INTEGER NOT NULL,
 amount INTEGER NOT NULL CHECK(amount>0),
 date TEXT NOT NULL,
 mode TEXT NOT NULL CHECK(mode IN ('CASH','UPI','BANK_TRANSFER','CHEQUE','CARD','OTHER')),
 reference TEXT NOT NULL,
 UNIQUE(business_id,reference),
 FOREIGN KEY(business_id,document_id) REFERENCES documents(business_id,id)
);
CREATE INDEX payment_document ON payments(business_id,document_id);
CREATE TRIGGER documents_finalized_update BEFORE UPDATE ON documents WHEN OLD.status!='DRAFT' BEGIN SELECT RAISE(ABORT,'finalized documents are immutable'); END;
CREATE TRIGGER documents_finalized_delete BEFORE DELETE ON documents WHEN OLD.status!='DRAFT' BEGIN SELECT RAISE(ABORT,'finalized documents are immutable'); END;
CREATE TRIGGER payments_no_update BEFORE UPDATE ON payments BEGIN SELECT RAISE(ABORT,'payments are immutable'); END;
CREATE TRIGGER payments_no_delete BEFORE DELETE ON payments BEGIN SELECT RAISE(ABORT,'payments are immutable'); END;
