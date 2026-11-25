CREATE TABLE IF NOT EXISTS ai_image_jobs (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    seller_id BIGINT NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    source_object_key TEXT NOT NULL UNIQUE,
    cutout_object_key TEXT NOT NULL DEFAULT '',
    source_content_type TEXT NOT NULL,
    source_byte_size BIGINT NOT NULL CHECK (source_byte_size > 0 AND source_byte_size <= 26214400),
    source_width INTEGER NOT NULL DEFAULT 0 CHECK (source_width >= 0),
    source_height INTEGER NOT NULL DEFAULT 0 CHECK (source_height >= 0),
    dominant_colors JSONB NOT NULL DEFAULT '[]'::JSONB,
    aspect_ratio TEXT NOT NULL DEFAULT 'square' CHECK (aspect_ratio IN ('square', 'portrait', 'landscape')),
    status TEXT NOT NULL DEFAULT 'uploaded' CHECK (status IN ('uploaded', 'processing', 'completed', 'failed', 'pending_approval', 'approved', 'rejected')),
    engine_used TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS ai_image_jobs_product_idx
    ON ai_image_jobs (product_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ai_image_jobs_seller_idx
    ON ai_image_jobs (seller_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS ai_image_variations (
    id BIGSERIAL PRIMARY KEY,
    job_id BIGINT NOT NULL REFERENCES ai_image_jobs(id) ON DELETE CASCADE,
    preset_slug TEXT NOT NULL,
    preset_label TEXT NOT NULL,
    category_recommended BOOLEAN NOT NULL DEFAULT FALSE,
    object_key TEXT NOT NULL UNIQUE,
    content_type TEXT NOT NULL CHECK (content_type IN ('image/png', 'image/jpeg', 'image/webp')),
    byte_size BIGINT NOT NULL CHECK (byte_size > 0 AND byte_size <= 26214400),
    width INTEGER NOT NULL CHECK (width > 0),
    height INTEGER NOT NULL CHECK (height > 0),
    offset_x DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (offset_x >= -0.45 AND offset_x <= 0.45),
    offset_y DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (offset_y >= -0.45 AND offset_y <= 0.45),
    scale DOUBLE PRECISION NOT NULL DEFAULT 0.80 CHECK (scale >= 0.30 AND scale <= 1.40),
    fit_mode TEXT NOT NULL DEFAULT 'contain' CHECK (fit_mode IN ('contain', 'cover', 'fill')),
    shadow_strength DOUBLE PRECISION NOT NULL DEFAULT 0.35 CHECK (shadow_strength >= 0 AND shadow_strength <= 1),
    status TEXT NOT NULL DEFAULT 'pending_approval' CHECK (status IN ('pending_approval', 'approved', 'rejected')),
    published_media_id BIGINT REFERENCES product_media(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS ai_image_variations_job_idx
    ON ai_image_variations (job_id, id);
