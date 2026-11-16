-- Restored credit gets its own expiry. Extending the source grant would also
-- revive any unrelated balance that expired without being spent.
ALTER TABLE store_credit_grants ADD COLUMN source_grant_id bigint REFERENCES store_credit_grants(id);
ALTER TABLE store_credit_grants ADD COLUMN restoration_key text UNIQUE;
