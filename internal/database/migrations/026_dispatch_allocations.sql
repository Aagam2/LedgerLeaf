CREATE TABLE dispatch_allocations (
 business_id INTEGER NOT NULL,dispatch_id INTEGER NOT NULL,product_id INTEGER NOT NULL,
 reference TEXT NOT NULL,date TEXT NOT NULL,document_id INTEGER,
 quantity INTEGER NOT NULL CHECK(quantity>0),value INTEGER NOT NULL CHECK(value>=0),
 PRIMARY KEY(business_id,dispatch_id,product_id,reference),
 FOREIGN KEY(business_id,dispatch_id) REFERENCES dispatches(business_id,id),
 FOREIGN KEY(business_id,product_id) REFERENCES products(business_id,id),
 FOREIGN KEY(business_id,document_id) REFERENCES documents(business_id,id)
);
INSERT INTO dispatch_allocations
 SELECT d.business_id,d.id,json_extract(l.value,'$.product_id'),
 CASE WHEN d.document_id IS NOT NULL THEN 'document:'||d.document_id ELSE 'legacy-dispatch-return:'||d.id END,
 COALESCE((SELECT date FROM documents WHERE business_id=d.business_id AND id=d.document_id),d.returned_date),d.document_id,
 json_extract(l.value,'$.quantity')*COALESCE(NULLIF(json_extract(l.value,'$.unit_factor'),0),1000)/1000,
 json_extract(l.value,'$.cost')
 FROM dispatches d,json_each(d.snapshot,'$.lines') l WHERE d.document_id IS NOT NULL OR d.returned_date IS NOT NULL;
CREATE INDEX dispatch_allocation_document ON dispatch_allocations(business_id,document_id);
CREATE TRIGGER dispatch_allocation_no_update BEFORE UPDATE ON dispatch_allocations BEGIN SELECT RAISE(ABORT,'dispatch allocations are immutable'); END;
CREATE TRIGGER dispatch_allocation_no_delete BEFORE DELETE ON dispatch_allocations BEGIN SELECT RAISE(ABORT,'dispatch allocations are immutable'); END;
