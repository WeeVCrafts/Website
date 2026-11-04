-- A provider order can contain several independent payment attempts. Keep the
-- existing payment_attempts row as the payable-order aggregate for compatibility.
CREATE TABLE provider_payment_attempts (
    provider TEXT NOT NULL,
    provider_payment_id TEXT NOT NULL,
    payment_attempt_id BIGINT NOT NULL REFERENCES payment_attempts(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('pending','authorized','captured','failed','refunded')),
    amount_cents BIGINT NOT NULL CHECK (amount_cents>0),
    currency TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(provider,provider_payment_id)
);
INSERT INTO provider_payment_attempts(provider,provider_payment_id,payment_attempt_id,status,amount_cents,currency)
SELECT provider,provider_payment_id,id,CASE WHEN status='created' THEN 'pending' ELSE status END,amount_cents,currency
FROM payment_attempts WHERE provider_payment_id IS NOT NULL AND provider_payment_id<>'' AND amount_cents>0;

ALTER TABLE orders ADD COLUMN payment_review_required BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE payment_attempts ADD COLUMN next_reconcile_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN reconcile_attempts INTEGER NOT NULL DEFAULT 0;
CREATE INDEX payment_attempts_reconciliation_due ON payment_attempts(next_reconcile_at,id)
    WHERE provider='razorpay' AND status IN ('pending','authorized','failed');

CREATE TABLE payment_review_cases (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    payment_attempt_id BIGINT NOT NULL REFERENCES payment_attempts(id) ON DELETE RESTRICT,
    provider_payment_id TEXT NOT NULL UNIQUE,
    amount_cents BIGINT NOT NULL CHECK(amount_cents>0),
    currency TEXT NOT NULL,
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','approved','refund_pending','refunded')),
    resolution_reason TEXT NOT NULL DEFAULT '',
    resolved_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    refund_submission_state TEXT NOT NULL DEFAULT 'queued' CHECK(refund_submission_state IN ('queued','submitting','uncertain','pending','processed','failed')),
    refund_attempt INTEGER NOT NULL DEFAULT 1 CHECK(refund_attempt>0),
    provider_refund_id TEXT UNIQUE,
    next_reconcile_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reconcile_attempts INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX payment_review_cases_queue ON payment_review_cases(status,created_at,id);

-- Keep any previously recorded capture outside the fulfilment lifecycle on
-- hold too. This does not invent a payment or change stock/earnings history.
INSERT INTO payment_review_cases(order_id,payment_attempt_id,provider_payment_id,amount_cents,currency,reason)
SELECT o.id,pa.id,pa.provider_payment_id,pa.amount_cents,pa.currency,'Existing captured payment outside the payable order lifecycle'
FROM orders o JOIN payment_attempts pa ON pa.order_id=o.id
WHERE pa.status='captured' AND pa.provider_payment_id IS NOT NULL AND pa.provider_payment_id<>''
  AND pa.amount_cents>0 AND o.status IN ('pending_payment','payment_failed','cancelled')
ON CONFLICT(provider_payment_id) DO NOTHING;
UPDATE orders o SET payment_review_required=TRUE WHERE EXISTS(SELECT 1 FROM payment_review_cases c WHERE c.order_id=o.id AND c.status='open');
INSERT INTO notifications(user_id,kind,title,body)
SELECT DISTINCT u.id,'payment_review','Captured payments need review','Open Finance payment reviews to resolve previously captured payments held outside the payable order lifecycle.'
FROM users u JOIN user_roles ur ON ur.user_id=u.id
WHERE u.status='active' AND ur.role_slug IN ('admin','super_admin') AND EXISTS(SELECT 1 FROM payment_review_cases);
INSERT INTO audit_logs(action,resource_type,resource_id,request_id,metadata)
SELECT 'payment.review_backfill','payment_review','migration-0054','migration-0054',jsonb_build_object('case_count',COUNT(*)) FROM payment_review_cases HAVING COUNT(*)>0;
