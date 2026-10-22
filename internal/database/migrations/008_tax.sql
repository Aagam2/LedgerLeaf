ALTER TABLE businesses ADD COLUMN tax_scheme TEXT NOT NULL DEFAULT 'UNREGISTERED' CHECK(tax_scheme IN ('UNREGISTERED','REGULAR','COMPOSITION'));
ALTER TABLE parties ADD COLUMN gstin TEXT NOT NULL DEFAULT '';
ALTER TABLE parties ADD COLUMN state_code TEXT NOT NULL DEFAULT '';
CREATE TABLE product_taxes (
 id INTEGER PRIMARY KEY, business_id INTEGER NOT NULL, product_id INTEGER NOT NULL,
 effective_date TEXT NOT NULL, hsn TEXT NOT NULL, treatment TEXT NOT NULL CHECK(treatment IN ('TAXABLE','NIL','EXEMPT','NON_GST')),
 rate INTEGER NOT NULL CHECK(rate BETWEEN 0 AND 10000 AND rate%2=0), cess INTEGER NOT NULL CHECK(cess BETWEEN 0 AND 10000),
 FOREIGN KEY(business_id,product_id) REFERENCES products(business_id,id),
 UNIQUE(business_id,product_id,effective_date)
);
CREATE INDEX product_tax_lookup ON product_taxes(business_id,product_id,effective_date DESC);
CREATE TRIGGER immutable_tax_update BEFORE UPDATE ON product_taxes BEGIN SELECT RAISE(ABORT,'tax configurations are immutable; add a new effective date'); END;
CREATE TRIGGER immutable_tax_delete BEFORE DELETE ON product_taxes BEGIN SELECT RAISE(ABORT,'tax configurations are immutable'); END;
