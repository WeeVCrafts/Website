-- Public, seller-authored content. Existing makers have no fabricated biography.
ALTER TABLE sellers
    ADD COLUMN public_location TEXT NOT NULL DEFAULT '' CHECK (length(public_location) <= 160),
    ADD COLUMN public_story TEXT NOT NULL DEFAULT '' CHECK (length(public_story) <= 4000),
    ADD COLUMN public_technique TEXT NOT NULL DEFAULT '' CHECK (length(public_technique) <= 500),
    ADD COLUMN public_portrait TEXT NOT NULL DEFAULT '' CHECK (length(public_portrait) <= 500);
