CREATE TABLE commercial_documents (
 id INTEGER PRIMARY KEY,business_id INTEGER NOT NULL REFERENCES businesses(id),
 document_type TEXT NOT NULL CHECK(document_type IN ('ESTIMATE','QUOTATION','PROFORMA','PURCHASE_ORDER','DELIVERY_CHALLAN')),
 kind TEXT NOT NULL CHECK(kind IN ('SALE','PURCHASE')),
 status TEXT NOT NULL DEFAULT 'DRAFT' CHECK(status IN ('DRAFT','ISSUED')),
 number TEXT, date TEXT NOT NULL,party_id INTEGER NOT NULL,total INTEGER NOT NULL,snapshot TEXT NOT NULL,
 converted_document_id INTEGER,
 UNIQUE(business_id,id),UNIQUE(business_id,number),
 FOREIGN KEY(business_id,party_id) REFERENCES parties(business_id,id),
 FOREIGN KEY(business_id,converted_document_id) REFERENCES documents(business_id,id)
);
CREATE INDEX commercial_document_list ON commercial_documents(business_id,document_type,date,id);
CREATE TRIGGER commercial_issued_delete BEFORE DELETE ON commercial_documents WHEN OLD.status='ISSUED' BEGIN SELECT RAISE(ABORT,'issued document is immutable'); END;
CREATE TRIGGER commercial_issued_update BEFORE UPDATE ON commercial_documents WHEN OLD.status='ISSUED' AND (NEW.snapshot!=OLD.snapshot OR NEW.total!=OLD.total OR NEW.status!=OLD.status OR NEW.kind!=OLD.kind OR NEW.number!=OLD.number OR NEW.party_id!=OLD.party_id OR NEW.business_id!=OLD.business_id OR NEW.date!=OLD.date OR NEW.document_type!=OLD.document_type OR OLD.converted_document_id IS NOT NULL) BEGIN SELECT RAISE(ABORT,'issued document is immutable'); END;
