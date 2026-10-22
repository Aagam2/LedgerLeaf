CREATE TABLE media_assets (
 id INTEGER PRIMARY KEY,
 business_id INTEGER NOT NULL REFERENCES businesses(id),
 format TEXT NOT NULL CHECK(format IN ('png','jpeg')),
 content BLOB NOT NULL,
 UNIQUE(business_id,id)
);
CREATE TRIGGER media_immutable BEFORE UPDATE ON media_assets BEGIN SELECT RAISE(ABORT,'saved images are immutable'); END;
CREATE TRIGGER media_no_delete BEFORE DELETE ON media_assets BEGIN SELECT RAISE(ABORT,'saved images are immutable'); END;
ALTER TABLE businesses ADD COLUMN logo_id INTEGER REFERENCES media_assets(id);
ALTER TABLE products ADD COLUMN image_id INTEGER REFERENCES media_assets(id);
CREATE TRIGGER business_logo_scope BEFORE UPDATE OF logo_id ON businesses
WHEN NEW.logo_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM media_assets WHERE id=NEW.logo_id AND business_id=NEW.id)
BEGIN SELECT RAISE(ABORT,'logo belongs to another business'); END;
CREATE TRIGGER product_image_scope BEFORE UPDATE OF image_id ON products
WHEN NEW.image_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM media_assets WHERE id=NEW.image_id AND business_id=NEW.business_id)
BEGIN SELECT RAISE(ABORT,'image belongs to another business'); END;
