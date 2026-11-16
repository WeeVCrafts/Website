ALTER TABLE marketplace_claims ADD COLUMN review_due_at timestamptz,ADD COLUMN pickup_due_at timestamptz,ADD COLUMN resolution_due_at timestamptz;
CREATE INDEX marketplace_claim_review_due ON marketplace_claims(review_due_at) WHERE status='requested';
