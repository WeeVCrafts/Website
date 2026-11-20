ALTER TABLE marketplace_setup_submissions DROP CONSTRAINT marketplace_setup_submissions_status_check;
ALTER TABLE marketplace_setup_submissions ADD CHECK(status IN ('draft','submitted','changes_requested','approved','superseded','attested'));

ALTER TABLE seller_invoice_profiles
    ALTER COLUMN verified_by DROP NOT NULL,
    ALTER COLUMN verified_at DROP NOT NULL,
    ADD COLUMN attested_by bigint REFERENCES users(id),
    ADD COLUMN attested_at timestamptz,
    ADD CHECK ((verified_by IS NOT NULL AND verified_at IS NOT NULL AND attested_by IS NULL AND attested_at IS NULL)
        OR (verified_by IS NULL AND verified_at IS NULL AND attested_by IS NOT NULL AND attested_at IS NOT NULL));

-- An operator can verify a previously attested profile, including during
-- a rolling deployment when an older application issues the update.
CREATE FUNCTION replace_setup_attestation_on_verification() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.verified_by IS NOT NULL AND NEW.verified_at IS NOT NULL THEN
        NEW.attested_by := NULL;
        NEW.attested_at := NULL;
    END IF;
    RETURN NEW;
END $$;

ALTER TABLE variant_tax_profiles
    ALTER COLUMN verified_by DROP NOT NULL,
    ALTER COLUMN verified_at DROP NOT NULL,
    ADD COLUMN attested_by bigint REFERENCES users(id),
    ADD COLUMN attested_at timestamptz,
    ADD CHECK ((verified_by IS NOT NULL AND verified_at IS NOT NULL AND attested_by IS NULL AND attested_at IS NULL)
        OR (verified_by IS NULL AND verified_at IS NULL AND attested_by IS NOT NULL AND attested_at IS NOT NULL));

CREATE TRIGGER seller_invoice_profile_verification BEFORE UPDATE OF verified_by,verified_at ON seller_invoice_profiles
FOR EACH ROW EXECUTE FUNCTION replace_setup_attestation_on_verification();
CREATE TRIGGER variant_tax_profile_verification BEFORE UPDATE OF verified_by,verified_at ON variant_tax_profiles
FOR EACH ROW EXECUTE FUNCTION replace_setup_attestation_on_verification();
