package postgres

import (
	"context"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *PaymentRepository) ClaimRefundSubmission(ctx context.Context, id int64) (bool, error) {
	result, err := r.pool.Exec(ctx, `UPDATE payment_refunds SET submission_state='submitting', submission_started_at=NOW(), next_reconcile_at=NOW()+INTERVAL '5 minutes' WHERE id=$1 AND status='pending' AND submission_state='queued' AND provider_refund_id IS NULL`, id)
	return result.RowsAffected() == 1, err
}

func (r *PaymentRepository) ClaimRefundReconciliation(ctx context.Context, limit int) ([]ports.PaymentRefundRecord, error) {
	rows, err := r.pool.Query(ctx, `WITH due AS (
	  SELECT id FROM payment_refunds WHERE status='pending'
	    AND submission_state IN ('submitting','uncertain','pending') AND next_reconcile_at<=NOW()
	  ORDER BY next_reconcile_at,id LIMIT $1 FOR UPDATE SKIP LOCKED
	), claimed AS (
	  UPDATE payment_refunds p SET next_reconcile_at=NOW()+LEAST(60,5*power(2,LEAST(reconcile_attempts,4)))*INTERVAL '1 minute', reconcile_attempts=reconcile_attempts+1
	  FROM due WHERE p.id=due.id RETURNING p.*
	)
	SELECT p.id,p.order_id,o.order_number,o.status,pa.status,
	  COALESCE(pa.provider_payment_id,''),COALESCE(p.provider_refund_id,''),p.status,p.amount_cents,p.currency,p.reason,p.request_key,p.submission_state
	FROM claimed p JOIN orders o ON o.id=p.order_id
	JOIN LATERAL (SELECT status,provider_payment_id FROM payment_attempts WHERE order_id=o.id ORDER BY id DESC LIMIT 1) pa ON TRUE`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []ports.PaymentRefundRecord
	for rows.Next() {
		var record ports.PaymentRefundRecord
		if err := rows.Scan(&record.ID, &record.OrderID, &record.OrderNumber, &record.OrderStatus, &record.PaymentStatus, &record.ProviderPaymentID, &record.ProviderRefundID, &record.Status, &record.AmountCents, &record.Currency, &record.Reason, &record.RequestKey, &record.SubmissionState); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
