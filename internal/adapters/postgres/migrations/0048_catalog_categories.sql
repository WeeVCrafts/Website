ALTER TABLE categories
    ADD COLUMN image_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN image_object_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN display_order INTEGER NOT NULL DEFAULT 1000 CHECK(display_order >= 0),
    ADD COLUMN show_in_navigation BOOLEAN NOT NULL DEFAULT false;

INSERT INTO categories(slug,name,image_url,display_order,show_in_navigation) VALUES
 ('arts','Arts','/assets/images/customer/madhubani-tree.webp',10,true),
 ('home','Home & Living','/assets/images/customer/black-cushion.webp',20,true),
 ('textiles','Textiles','/assets/images/customer/saree-maroon.webp',30,true),
 ('gifts','Gifts','/assets/images/customer/wooden-box.webp',40,true),
 ('pottery-ceramics','Pottery & Ceramics','/assets/images/customer/ceramic-mugs.webp',50,true),
 ('handloom-sarees','Handloom Sarees','/assets/images/customer/saree-blue.webp',60,true),
 ('brass-metalwork','Brass & Metalwork','/assets/images/customer/brass-elephant.webp',70,true)
ON CONFLICT(slug) DO UPDATE SET image_url=EXCLUDED.image_url,
 display_order=EXCLUDED.display_order,show_in_navigation=true;

CREATE TABLE product_categories (
 product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
 category_slug TEXT NOT NULL REFERENCES categories(slug),
 PRIMARY KEY(product_id,category_slug)
);
CREATE INDEX product_categories_category_idx ON product_categories(category_slug,product_id);
INSERT INTO product_categories SELECT id,category_slug FROM products;

-- All writers, including older clients, retain their primary membership.
CREATE FUNCTION sync_primary_category() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO product_categories(product_id,category_slug) VALUES(NEW.id,NEW.category_slug)
 ON CONFLICT DO NOTHING;
 RETURN NEW;
END $$;
CREATE TRIGGER products_primary_category AFTER INSERT OR UPDATE OF category_slug ON products
 FOR EACH ROW EXECUTE FUNCTION sync_primary_category();

CREATE FUNCTION protect_primary_category() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM products WHERE id=OLD.product_id AND category_slug=OLD.category_slug)
 AND NOT EXISTS(SELECT 1 FROM product_categories WHERE product_id=OLD.product_id AND category_slug=OLD.category_slug) THEN
  RAISE EXCEPTION 'Choose a replacement primary category before removing this membership' USING ERRCODE='23514';
 END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER product_categories_primary AFTER DELETE OR UPDATE ON product_categories
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION protect_primary_category();

-- A revision in authoritative storage invalidates cached catalog projections
-- across processes, without relying on a Redis prefix deletion.
CREATE TABLE catalog_revision(singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK(singleton),revision BIGINT NOT NULL DEFAULT 1);
INSERT INTO catalog_revision DEFAULT VALUES;
CREATE FUNCTION bump_catalog_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE catalog_revision SET revision=revision+1 WHERE singleton;
 RETURN NULL;
END $$;
CREATE TRIGGER categories_revision AFTER INSERT OR UPDATE OR DELETE ON categories FOR EACH STATEMENT EXECUTE FUNCTION bump_catalog_revision();
CREATE TRIGGER memberships_revision AFTER INSERT OR UPDATE OR DELETE ON product_categories FOR EACH STATEMENT EXECUTE FUNCTION bump_catalog_revision();
CREATE TRIGGER products_revision AFTER INSERT OR UPDATE OR DELETE ON products FOR EACH STATEMENT EXECUTE FUNCTION bump_catalog_revision();

-- Import retries resume only drafts created by this importer; unrelated SKUs
-- are never taken over or overwritten.
CREATE TABLE merchant_catalog_imports (
 seller_id BIGINT NOT NULL REFERENCES sellers(id),
 sku TEXT NOT NULL,
 product_id BIGINT NOT NULL UNIQUE REFERENCES products(id) ON DELETE CASCADE,
 PRIMARY KEY(seller_id,sku)
);
