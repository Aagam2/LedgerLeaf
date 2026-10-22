CREATE TABLE credit_notes (
 id INTEGER PRIMARY KEY, business_id INTEGER NOT NULL, document_id INTEGER NOT NULL,
 number TEXT NOT NULL, date TEXT NOT NULL, reason TEXT NOT NULL,
 total INTEGER NOT NULL CHECK(total>=0), refund INTEGER NOT NULL CHECK(refund>=0),
 refund_mode TEXT NOT NULL CHECK(refund_mode IN ('CASH','BANK')), reference TEXT NOT NULL,
 snapshot TEXT NOT NULL,
 UNIQUE(business_id,number), UNIQUE(business_id,reference),
 FOREIGN KEY(business_id,document_id) REFERENCES documents(business_id,id)
);
CREATE INDEX credit_note_document ON credit_notes(business_id,document_id,date);
INSERT INTO credit_notes(business_id,document_id,number,date,reason,total,refund,refund_mode,reference,snapshot)
SELECT v.business_id,v.document_id,'CN-OLD-'||v.id,v.date,v.reason,d.total,v.refund,v.refund_mode,v.reference,d.snapshot FROM document_reversals v JOIN documents d ON d.business_id=v.business_id AND d.id=v.document_id;
CREATE TRIGGER credit_notes_no_update BEFORE UPDATE ON credit_notes BEGIN SELECT RAISE(ABORT,'credit notes are immutable'); END;
CREATE TRIGGER credit_notes_no_delete BEFORE DELETE ON credit_notes BEGIN SELECT RAISE(ABORT,'credit notes are immutable'); END;
CREATE VIEW document_settlements AS SELECT d.business_id,d.id,
 COALESCE((SELECT SUM(amount) FROM payments p WHERE p.business_id=d.business_id AND p.document_id=d.id),0) AS paid,
 COALESCE((SELECT SUM(total) FROM credit_notes c WHERE c.business_id=d.business_id AND c.document_id=d.id),0) AS returned,
 COALESCE((SELECT SUM(refund) FROM credit_notes c WHERE c.business_id=d.business_id AND c.document_id=d.id),0) AS refunded
FROM documents d;
