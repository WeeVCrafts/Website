ALTER TABLE categories ADD COLUMN description TEXT NOT NULL DEFAULT '';
-- Releases are immutable snapshots; the single head row serializes publication.
CREATE TABLE storefront_releases (
 id BIGSERIAL PRIMARY KEY,
 config JSONB NOT NULL,
 published_by BIGINT REFERENCES users(id),
 published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 note TEXT NOT NULL DEFAULT ''
);
CREATE TABLE storefront_head (
 singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
 release_id BIGINT NOT NULL REFERENCES storefront_releases(id)
);
CREATE TABLE storefront_drafts (
 id BIGSERIAL PRIMARY KEY,
 version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
 base_release BIGINT NOT NULL REFERENCES storefront_releases(id),
 config JSONB NOT NULL,
 state TEXT NOT NULL DEFAULT 'draft' CHECK (state IN ('draft','submitted','published')),
 created_by BIGINT NOT NULL REFERENCES users(id),
 updated_by BIGINT NOT NULL REFERENCES users(id),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 review_note TEXT NOT NULL DEFAULT '',
 published_release BIGINT REFERENCES storefront_releases(id)
);
CREATE INDEX storefront_drafts_updated ON storefront_drafts(updated_at DESC,id DESC);
CREATE TABLE storefront_media (
 id TEXT PRIMARY KEY,
 object_key TEXT NOT NULL UNIQUE,
 content_type TEXT NOT NULL,
 byte_size BIGINT NOT NULL CHECK (byte_size > 0),
 alt TEXT NOT NULL,
 focal_x INTEGER NOT NULL DEFAULT 50 CHECK (focal_x BETWEEN 0 AND 100),
 focal_y INTEGER NOT NULL DEFAULT 50 CHECK (focal_y BETWEEN 0 AND 100),
 variants JSONB NOT NULL DEFAULT '{}'::jsonb,
 created_by BIGINT NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE FUNCTION storefront_immutable_release() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Published storefront releases are immutable'; END $$;
CREATE TRIGGER storefront_release_immutable BEFORE UPDATE OR DELETE ON storefront_releases FOR EACH ROW EXECUTE FUNCTION storefront_immutable_release();
ALTER TABLE orders ADD COLUMN policy_revision BIGINT REFERENCES storefront_releases(id), ADD COLUMN policy_snapshot JSONB;

ALTER TABLE orders ADD COLUMN delivered_at TIMESTAMPTZ;
CREATE FUNCTION storefront_delivery_date() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.status='delivered' AND NEW.delivered_at IS NULL THEN NEW.delivered_at=now(); END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER storefront_delivery_date BEFORE UPDATE OF status ON orders FOR EACH ROW EXECUTE FUNCTION storefront_delivery_date();
