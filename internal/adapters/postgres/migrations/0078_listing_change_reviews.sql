CREATE TABLE listing_change_reviews (
    product_id bigint PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision>0),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','changes_requested')),
    cover_base text,
    cover_proposed text,
    review_note text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((cover_base IS NULL) = (cover_proposed IS NULL))
);

CREATE INDEX listing_change_reviews_pending
    ON listing_change_reviews(updated_at,product_id) WHERE status='pending';

CREATE TABLE listing_price_changes (
    product_id bigint NOT NULL REFERENCES listing_change_reviews(product_id) ON DELETE CASCADE,
    variant_id bigint NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    base_price bigint NOT NULL,
    base_compare_at bigint,
    proposed_price bigint NOT NULL CHECK (proposed_price>0),
    proposed_compare_at bigint CHECK (proposed_compare_at>=proposed_price),
    new_variant boolean NOT NULL DEFAULT false,
    PRIMARY KEY (product_id,variant_id)
);

CREATE TABLE listing_media_changes (
    product_id bigint NOT NULL REFERENCES listing_change_reviews(product_id) ON DELETE CASCADE,
    media_id bigint NOT NULL REFERENCES product_media(id) ON DELETE CASCADE,
    publish boolean NOT NULL,
    PRIMARY KEY (product_id,media_id)
);

ALTER TABLE product_variants DROP CONSTRAINT product_variants_status_check,
    ADD CONSTRAINT product_variants_status_check CHECK (status IN ('active','archived','pending_review'));
