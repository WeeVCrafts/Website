ALTER TABLE product_media ADD COLUMN derivatives_checked_at timestamptz;
CREATE TABLE product_image_derivatives (
    media_id bigint NOT NULL REFERENCES product_media(id) ON DELETE CASCADE,
    width integer NOT NULL CHECK(width IN (320,640,960)),
    source_key text NOT NULL,
    source_type text NOT NULL,
    source_bytes bigint NOT NULL,
    object_key text NOT NULL,
    content_type text NOT NULL CHECK(content_type IN ('image/jpeg','image/png','image/webp')),
    byte_size bigint NOT NULL CHECK(byte_size>0 AND byte_size<=1048576),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(media_id,width)
);
