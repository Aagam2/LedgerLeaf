CREATE TABLE document_reversals (
 id INTEGER PRIMARY KEY,
 business_id INTEGER NOT NULL,
 document_id INTEGER NOT NULL,
 date TEXT NOT NULL,
 reason TEXT NOT NULL,
 refund INTEGER NOT NULL CHECK(refund>=0),
 refund_mode TEXT NOT NULL CHECK(refund_mode IN ('CASH','BANK')),
 reference TEXT NOT NULL,
 UNIQUE(business_id,document_id),
 UNIQUE(business_id,reference),
 FOREIGN KEY(business_id,document_id) REFERENCES documents(business_id,id)
);
CREATE TRIGGER reversals_no_update BEFORE UPDATE ON document_reversals BEGIN SELECT RAISE(ABORT,'reversals are immutable'); END;
CREATE TRIGGER reversals_no_delete BEFORE DELETE ON document_reversals BEGIN SELECT RAISE(ABORT,'reversals are immutable'); END;
