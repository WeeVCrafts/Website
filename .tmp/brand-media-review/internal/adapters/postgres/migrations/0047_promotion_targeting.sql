-- Make promotion targeting explicit so an empty selected-product campaign
-- never falls through to the all-products rule.
ALTER TABLE promotions
    ADD COLUMN IF NOT EXISTS targeting_mode TEXT NOT NULL DEFAULT 'all';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'promotions_targeting_mode_check'
          AND conrelid = 'promotions'::regclass
    ) THEN
        ALTER TABLE promotions
            ADD CONSTRAINT promotions_targeting_mode_check
            CHECK (targeting_mode IN ('all', 'selected'));
    END IF;
END $$;

-- Existing mappings are the source of truth for the initial backfill.
UPDATE promotions p
SET targeting_mode = CASE
    WHEN EXISTS (SELECT 1 FROM promotion_products pp WHERE pp.promotion_id = p.id)
        THEN 'selected'
    ELSE 'all'
END;

-- Codes are stored uppercase by the application. Abort with an actionable
-- report if an older database contains mixed-case duplicates.
DO $$
DECLARE duplicates TEXT;
BEGIN
    SELECT string_agg(code, ', ' ORDER BY code)
    INTO duplicates
    FROM (
        SELECT MIN(code) AS code
        FROM coupons
        GROUP BY LOWER(code)
        HAVING COUNT(*) > 1
    ) duplicate_codes;
    IF duplicates IS NOT NULL THEN
        RAISE EXCEPTION 'Promotion migration blocked: duplicate coupon codes [%]. Rename the duplicates and retry.', duplicates;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS coupons_code_lower_key
    ON coupons (LOWER(code));
