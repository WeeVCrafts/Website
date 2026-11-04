package handlers

// Provider failures are individual attempts. Keep legacy/expired aggregate
// failures visible only when no individual failed attempt represents them.
const failedPaymentActivitySQL = `(
 SELECT 'provider:'||p.provider_payment_id AS id,p.updated_at AS created_at,p.status,pa.order_id,p.amount_cents,p.provider
 FROM provider_payment_attempts p JOIN payment_attempts pa ON pa.id=p.payment_attempt_id WHERE p.status='failed'
 UNION ALL
 SELECT 'aggregate:'||pa.id::text,pa.created_at,pa.status,pa.order_id,pa.amount_cents,pa.provider
 FROM payment_attempts pa WHERE pa.status='failed' AND NOT EXISTS(SELECT 1 FROM provider_payment_attempts p WHERE p.payment_attempt_id=pa.id AND p.status='failed')
)`
