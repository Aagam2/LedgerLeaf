CREATE TABLE parties (
 id INTEGER PRIMARY KEY,
 business_id INTEGER NOT NULL REFERENCES businesses(id),
 kind TEXT NOT NULL CHECK(kind IN ('CUSTOMER','SUPPLIER')),
 name TEXT NOT NULL CHECK(length(trim(name)) BETWEEN 1 AND 160),
 phone TEXT NOT NULL DEFAULT '',
 email TEXT NOT NULL DEFAULT '',
 address TEXT NOT NULL DEFAULT '',
 active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1)),
 UNIQUE(business_id,id)
);
CREATE INDEX parties_search ON parties(business_id,kind,name);
CREATE TABLE products (
 id INTEGER PRIMARY KEY,
 business_id INTEGER NOT NULL REFERENCES businesses(id),
 name TEXT NOT NULL CHECK(length(trim(name)) BETWEEN 1 AND 160),
 sku TEXT NOT NULL,
 barcode TEXT NOT NULL DEFAULT '',
 unit TEXT NOT NULL DEFAULT 'PCS',
 kind TEXT NOT NULL CHECK(kind IN ('PRODUCT','SERVICE')),
 sale_price INTEGER NOT NULL CHECK(sale_price>=0),
 minimum_stock INTEGER NOT NULL DEFAULT 0 CHECK(minimum_stock>=0),
 active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1)),
 UNIQUE(business_id,sku),
 UNIQUE(business_id,id)
);
CREATE INDEX products_search ON products(business_id,name);
CREATE TABLE warehouses (
 id INTEGER PRIMARY KEY,
 business_id INTEGER NOT NULL REFERENCES businesses(id),
 name TEXT NOT NULL,
 UNIQUE(business_id,name),
 UNIQUE(business_id,id)
);
CREATE TABLE accounts (
 business_id INTEGER NOT NULL REFERENCES businesses(id),
 code TEXT NOT NULL,
 name TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('ASSET','LIABILITY','EQUITY','INCOME','EXPENSE')),
 PRIMARY KEY(business_id,code)
);
CREATE TABLE journals (
 id INTEGER PRIMARY KEY,
 business_id INTEGER NOT NULL REFERENCES businesses(id),
 date TEXT NOT NULL,
 reference TEXT NOT NULL,
 description TEXT NOT NULL,
 UNIQUE(business_id,reference),
 UNIQUE(business_id,id)
);
CREATE TABLE journal_lines (
 id INTEGER PRIMARY KEY,
 business_id INTEGER NOT NULL,
 journal_id INTEGER NOT NULL,
 account TEXT NOT NULL,
 debit INTEGER NOT NULL DEFAULT 0 CHECK(debit>=0),
 credit INTEGER NOT NULL DEFAULT 0 CHECK(credit>=0),
 CHECK((debit>0 AND credit=0) OR (credit>0 AND debit=0)),
 FOREIGN KEY(business_id,journal_id) REFERENCES journals(business_id,id),
 FOREIGN KEY(business_id,account) REFERENCES accounts(business_id,code)
);
CREATE INDEX journal_account ON journal_lines(business_id,account,journal_id);
CREATE TABLE stock_movements (
 id INTEGER PRIMARY KEY,
 business_id INTEGER NOT NULL,
 product_id INTEGER NOT NULL,
 warehouse_id INTEGER NOT NULL,
 quantity INTEGER NOT NULL CHECK(quantity!=0),
 value INTEGER NOT NULL,
 kind TEXT NOT NULL,
 reference TEXT NOT NULL,
 date TEXT NOT NULL,
 remarks TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
 FOREIGN KEY(business_id,product_id) REFERENCES products(business_id,id),
 FOREIGN KEY(business_id,warehouse_id) REFERENCES warehouses(business_id,id),
 UNIQUE(business_id,reference,product_id,warehouse_id)
);
CREATE INDEX stock_balance ON stock_movements(business_id,product_id,warehouse_id);
CREATE TABLE requests (
 business_id INTEGER NOT NULL REFERENCES businesses(id),
 key TEXT NOT NULL,
 payload TEXT NOT NULL,
 PRIMARY KEY(business_id,key)
);
INSERT INTO warehouses(business_id,name) SELECT id,'Main warehouse' FROM businesses;
