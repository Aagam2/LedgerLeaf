CREATE TABLE product_units (
 business_id INTEGER NOT NULL,
 product_id INTEGER NOT NULL,
 name TEXT NOT NULL COLLATE NOCASE,
 factor INTEGER NOT NULL CHECK(factor BETWEEN 1 AND 1000000000),
 PRIMARY KEY(business_id,product_id,name),
 FOREIGN KEY(business_id,product_id) REFERENCES products(business_id,id)
);
CREATE TRIGGER product_units_immutable BEFORE UPDATE ON product_units BEGIN SELECT RAISE(ABORT,'unit conversions are immutable'); END;
CREATE TRIGGER product_units_no_delete BEFORE DELETE ON product_units BEGIN SELECT RAISE(ABORT,'unit conversions are immutable'); END;
