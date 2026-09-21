ALTER TABLE consent_records
    DROP CONSTRAINT IF EXISTS consent_records_purpose_check;

ALTER TABLE consent_records
    ADD CONSTRAINT consent_records_purpose_check
    CHECK (purpose IN ('marketing', 'analytics', 'necessary', 'terms'));
