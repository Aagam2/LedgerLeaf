-- Rebuild receipts to allow independent receipts. Historical IDs and snapshots stay intact.
DROP TRIGGER receipts_no_delete;
DROP TRIGGER receipts_no_update;
DROP INDEX receipts_order;
ALTER TABLE goods_receipts RENAME TO goods_receipts_old;
CREATE TABLE goods_receipts(id INTEGER PRIMARY KEY,business_id INTEGER NOT NULL,order_id INTEGER,party_id INTEGER NOT NULL,warehouse_id INTEGER NOT NULL,date TEXT NOT NULL,reference TEXT NOT NULL,snapshot TEXT NOT NULL,value INTEGER NOT NULL CHECK(value>=0),document_id INTEGER,UNIQUE(business_id,id),UNIQUE(business_id,reference),UNIQUE(business_id,document_id),FOREIGN KEY(business_id,order_id) REFERENCES commercial_documents(business_id,id),FOREIGN KEY(business_id,party_id) REFERENCES parties(business_id,id),FOREIGN KEY(business_id,warehouse_id) REFERENCES warehouses(business_id,id),FOREIGN KEY(business_id,document_id) REFERENCES documents(business_id,id));
INSERT INTO goods_receipts SELECT * FROM goods_receipts_old;
DROP TABLE goods_receipts_old;
CREATE INDEX receipts_order ON goods_receipts(business_id,order_id);
CREATE TRIGGER receipts_no_delete BEFORE DELETE ON goods_receipts BEGIN SELECT RAISE(ABORT,'receipts are immutable'); END;
CREATE TRIGGER receipts_no_update BEFORE UPDATE ON goods_receipts WHEN NEW.business_id!=OLD.business_id OR NEW.order_id IS NOT OLD.order_id OR NEW.party_id!=OLD.party_id OR NEW.warehouse_id!=OLD.warehouse_id OR NEW.date!=OLD.date OR NEW.reference!=OLD.reference OR NEW.snapshot!=OLD.snapshot OR NEW.value!=OLD.value OR OLD.document_id IS NOT NULL BEGIN SELECT RAISE(ABORT,'receipts are immutable'); END;
CREATE TABLE receipt_matches(business_id INTEGER NOT NULL,receipt_id INTEGER NOT NULL,document_id INTEGER NOT NULL,product_id INTEGER NOT NULL,quantity INTEGER NOT NULL CHECK(quantity>0),value INTEGER NOT NULL CHECK(value>=0),PRIMARY KEY(business_id,receipt_id,document_id,product_id),FOREIGN KEY(business_id,receipt_id) REFERENCES goods_receipts(business_id,id),FOREIGN KEY(business_id,document_id) REFERENCES documents(business_id,id),FOREIGN KEY(business_id,product_id) REFERENCES products(business_id,id));
INSERT INTO receipt_matches SELECT g.business_id,g.id,g.document_id,json_extract(l.value,'$.product_id'),json_extract(l.value,'$.quantity')*COALESCE(NULLIF(json_extract(l.value,'$.unit_factor'),0),1000)/1000,json_extract(l.value,'$.amount') FROM goods_receipts g,json_each(g.snapshot,'$.lines') l WHERE g.document_id IS NOT NULL;
CREATE INDEX receipt_match_document ON receipt_matches(business_id,document_id);
CREATE TRIGGER receipt_matches_no_update BEFORE UPDATE ON receipt_matches BEGIN SELECT RAISE(ABORT,'receipt matches are immutable'); END;
CREATE TRIGGER receipt_matches_no_delete BEFORE DELETE ON receipt_matches BEGIN SELECT RAISE(ABORT,'receipt matches are immutable'); END;
CREATE TABLE receipt_reversals(business_id INTEGER NOT NULL,receipt_id INTEGER NOT NULL,date TEXT NOT NULL,reason TEXT NOT NULL,reference TEXT NOT NULL,PRIMARY KEY(business_id,receipt_id),UNIQUE(business_id,reference),FOREIGN KEY(business_id,receipt_id) REFERENCES goods_receipts(business_id,id));
CREATE TRIGGER receipt_reversals_no_update BEFORE UPDATE ON receipt_reversals BEGIN SELECT RAISE(ABORT,'receipt reversals are immutable'); END;
CREATE TRIGGER receipt_reversals_no_delete BEFORE DELETE ON receipt_reversals BEGIN SELECT RAISE(ABORT,'receipt reversals are immutable'); END;
