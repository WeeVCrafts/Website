ALTER TABLE payment_refunds
    ADD COLUMN submission_state TEXT NOT NULL DEFAULT 'queued'
      CHECK (submission_state IN ('queued','submitting','uncertain','pending','processed','failed')),
    ADD COLUMN submission_started_at TIMESTAMPTZ,
    ADD COLUMN next_reconcile_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN reconcile_attempts INTEGER NOT NULL DEFAULT 0;

-- Legacy missing references are ambiguous: never automatically submit again.
UPDATE payment_refunds SET submission_state = CASE
  WHEN status='processed' THEN 'processed'
  WHEN provider_refund_id IS NOT NULL THEN 'pending'
  ELSE 'uncertain' END;
UPDATE payment_refunds SET status='pending' WHERE submission_state='uncertain' AND status='failed';
CREATE INDEX payment_refunds_recovery_due_idx ON payment_refunds(next_reconcile_at,id)
  WHERE submission_state IN ('submitting','uncertain','pending');
