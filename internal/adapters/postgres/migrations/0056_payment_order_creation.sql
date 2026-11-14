-- Keep the submission claim when an HTTP response or the local attachment is lost.
ALTER TABLE payment_attempts
    ADD COLUMN creation_started_at TIMESTAMPTZ,
    ADD COLUMN creation_next_lookup_at TIMESTAMPTZ,
    ADD COLUMN creation_review_reason TEXT NOT NULL DEFAULT '';

CREATE INDEX payment_attempts_creation_due ON payment_attempts(creation_next_lookup_at, id)
    WHERE provider = 'razorpay' AND provider_order_id IS NULL
      AND creation_started_at IS NOT NULL AND creation_review_reason = '';

CREATE INDEX payment_attempts_creation_review ON payment_attempts(creation_started_at, id)
    WHERE provider = 'razorpay' AND creation_review_reason <> '';
