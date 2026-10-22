ALTER TABLE products ADD COLUMN category TEXT NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN brand TEXT NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN variant TEXT NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN purchase_price INTEGER NOT NULL DEFAULT 0 CHECK(purchase_price>=0);
ALTER TABLE products ADD COLUMN mrp INTEGER NOT NULL DEFAULT 0 CHECK(mrp>=0);
ALTER TABLE parties ADD COLUMN credit_limit INTEGER NOT NULL DEFAULT 0 CHECK(credit_limit>=0);
ALTER TABLE parties ADD COLUMN credit_days INTEGER NOT NULL DEFAULT 0 CHECK(credit_days BETWEEN 0 AND 3650);
CREATE TABLE price_lists(id INTEGER PRIMARY KEY,business_id INTEGER NOT NULL,product_id INTEGER NOT NULL,name TEXT NOT NULL,price INTEGER NOT NULL CHECK(price>=0),UNIQUE(business_id,product_id,name),FOREIGN KEY(business_id,product_id) REFERENCES products(business_id,id));
CREATE INDEX product_category_search ON products(business_id,category,name);
