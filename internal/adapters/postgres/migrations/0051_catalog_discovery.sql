CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;
ALTER TABLE products ADD COLUMN search_tags TEXT[] NOT NULL DEFAULT '{}',
 ADD COLUMN featured_pinned BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE products ADD CONSTRAINT product_search_tag_count CHECK(cardinality(search_tags)<=20);

-- Shared, transactionally maintained search document for all customer searches.
CREATE TABLE product_search_documents (
 product_id BIGINT PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
 name TEXT NOT NULL, skus TEXT[] NOT NULL, search_text TEXT NOT NULL, document TSVECTOR NOT NULL
);
CREATE INDEX product_search_document_idx ON product_search_documents USING gin(document);
CREATE INDEX product_search_text_idx ON product_search_documents USING gin(search_text public.gin_trgm_ops);
CREATE TABLE product_search_terms (
 product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
 term TEXT NOT NULL,
 PRIMARY KEY(product_id,term)
);
CREATE INDEX product_search_terms_typo_idx ON product_search_terms USING gin(term public.gin_trgm_ops);
CREATE INDEX product_search_terms_prefix_idx ON product_search_terms(term text_pattern_ops);
CREATE FUNCTION refresh_product_search(wanted_product BIGINT) RETURNS void LANGUAGE sql AS $$
 INSERT INTO product_search_documents(product_id,name,skus,search_text,document)
 SELECT p.id,lower(p.name),v.skus,
 lower(concat_ws(' ',p.name,array_to_string(p.search_tags,' '),c.names,b.name,p.description,array_to_string(v.skus,' '))),
 setweight(to_tsvector('simple',p.name||' '||array_to_string(p.search_tags,' ')),'A') ||
 setweight(to_tsvector('simple',coalesce(c.names,'')||' '||b.name||' '||array_to_string(v.skus,' ')),'B') ||
 setweight(to_tsvector('simple',p.description),'C')
 FROM products p JOIN brands b ON b.slug=p.brand_slug
 CROSS JOIN LATERAL (SELECT coalesce(array_agg(lower(sku)),'{}'::text[]) skus FROM product_variants WHERE product_id=p.id AND status='active') v
 CROSS JOIN LATERAL (SELECT string_agg(cat.name,' ') names FROM product_categories pc JOIN categories cat ON cat.slug=pc.category_slug WHERE pc.product_id=p.id) c
 WHERE p.id=wanted_product
 ON CONFLICT(product_id) DO UPDATE SET name=excluded.name,skus=excluded.skus,search_text=excluded.search_text,document=excluded.document;
 DELETE FROM product_search_terms WHERE product_id=wanted_product;
 INSERT INTO product_search_terms SELECT wanted_product,unnest(tsvector_to_array(document)) FROM product_search_documents WHERE product_id=wanted_product;
$$;
CREATE FUNCTION refresh_catalog_search_trigger() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE product BIGINT;
BEGIN
 IF TG_TABLE_NAME='products' THEN
  PERFORM refresh_product_search(NEW.id);
 ELSIF TG_TABLE_NAME IN ('product_variants','product_categories') THEN
  IF TG_OP<>'INSERT' THEN PERFORM refresh_product_search(OLD.product_id); END IF;
  IF TG_OP<>'DELETE' THEN PERFORM refresh_product_search(NEW.product_id); END IF;
 ELSE
  FOR product IN SELECT id FROM products LOOP PERFORM refresh_product_search(product); END LOOP;
 END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER product_search_refresh AFTER INSERT OR UPDATE ON products FOR EACH ROW EXECUTE FUNCTION refresh_catalog_search_trigger();
CREATE TRIGGER variant_search_refresh AFTER INSERT OR UPDATE OR DELETE ON product_variants FOR EACH ROW EXECUTE FUNCTION refresh_catalog_search_trigger();
CREATE TRIGGER membership_search_refresh AFTER INSERT OR UPDATE OR DELETE ON product_categories FOR EACH ROW EXECUTE FUNCTION refresh_catalog_search_trigger();
CREATE TRIGGER category_search_refresh AFTER UPDATE OF name ON categories FOR EACH STATEMENT EXECUTE FUNCTION refresh_catalog_search_trigger();
CREATE TRIGGER brand_search_refresh AFTER UPDATE OF name ON brands FOR EACH STATEMENT EXECUTE FUNCTION refresh_catalog_search_trigger();
SELECT refresh_product_search(id) FROM products;

-- Indexed candidate retrieval uses a broad fixed threshold locally; the final
-- score applies the stricter matching rule. No pool/session setting leaks.
CREATE FUNCTION catalog_search_candidates(q TEXT) RETURNS TABLE(product_id BIGINT)
LANGUAGE sql STABLE SET pg_trgm.similarity_threshold=0.29 AS $$
 WITH tokens AS (SELECT DISTINCT regexp_split_to_table(lower(btrim(q)),'\s+') token)
 SELECT terms.product_id FROM tokens
 CROSS JOIN LATERAL (
  SELECT st.product_id FROM product_search_terms st
  WHERE st.term LIKE replace(replace(replace(tokens.token,E'\\',E'\\\\'),'%',E'\\%'),'_',E'\\_')||'%'
  OR (tokens.token ~ '^[a-z]{4,}$' AND st.term OPERATOR(public.%) tokens.token)
 ) terms
 GROUP BY terms.product_id HAVING count(DISTINCT tokens.token)=(SELECT count(*) FROM tokens);
$$;

-- Exact identifiers beat text matches. All query terms must match; fuzzy
-- matching is bounded to words of at least four characters.
CREATE FUNCTION catalog_search_score(d product_search_documents, q TEXT) RETURNS real LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
 SELECT CASE
 WHEN btrim(q)='' THEN 1
 WHEN lower(btrim(q))=ANY(d.skus) THEN 1000
 WHEN lower(btrim(q))=d.name THEN 900
 WHEN d.document @@ plainto_tsquery('simple',q) THEN 100+ts_rank_cd(d.document,plainto_tsquery('simple',q))
 WHEN NOT EXISTS (
  SELECT 1 FROM regexp_split_to_table(lower(btrim(q)),'\s+') token
  WHERE NOT EXISTS (SELECT 1 FROM unnest(tsvector_to_array(d.document)) word
   WHERE starts_with(word,token) OR (token ~ '^[a-z]{4,}$' AND public.similarity(token,word)>=0.3))
 ) THEN 1+public.word_similarity(lower(q),d.search_text)
 ELSE 0 END::real;
$$;

-- Snapshot paid units once per order, irrespective of payment retries. Refunds
-- currently apply to whole orders; exclude orders with a processed refund.
CREATE TABLE featured_popularity (product_id BIGINT PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE, paid_units BIGINT NOT NULL, refreshed_at TIMESTAMPTZ NOT NULL);
CREATE FUNCTION refresh_featured_popularity() RETURNS void LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(78122351);
 IF EXISTS(SELECT 1 FROM featured_popularity WHERE refreshed_at>now()-interval '15 minutes') THEN RETURN; END IF;
 DELETE FROM featured_popularity;
 INSERT INTO featured_popularity
 SELECT p.id,coalesce(sum(sales.quantity),0),now() FROM products p
 LEFT JOIN (
  SELECT pv.product_id,oi.quantity FROM order_items oi JOIN product_variants pv ON pv.id=oi.variant_id
  JOIN orders o ON o.id=oi.order_id
  WHERE o.status IN ('paid','processing','shipped','delivered')
  AND EXISTS(SELECT 1 FROM payment_attempts pa WHERE pa.order_id=o.id AND pa.status='captured' AND pa.updated_at>=now()-interval '30 days')
  AND NOT EXISTS(SELECT 1 FROM payment_refunds rf WHERE rf.order_id=o.id AND rf.status='processed')
 ) sales ON sales.product_id=p.id GROUP BY p.id;
END $$;

CREATE TRIGGER variant_catalog_revision AFTER INSERT OR UPDATE OR DELETE ON product_variants FOR EACH STATEMENT EXECUTE FUNCTION bump_catalog_revision();

CREATE TRIGGER inventory_catalog_revision AFTER INSERT OR UPDATE OR DELETE ON inventory_stock FOR EACH STATEMENT EXECUTE FUNCTION bump_catalog_revision();

CREATE TRIGGER seller_catalog_revision AFTER INSERT OR UPDATE OR DELETE ON sellers FOR EACH STATEMENT EXECUTE FUNCTION bump_catalog_revision();

CREATE TRIGGER brand_catalog_revision AFTER INSERT OR UPDATE OR DELETE ON brands FOR EACH STATEMENT EXECUTE FUNCTION bump_catalog_revision();
