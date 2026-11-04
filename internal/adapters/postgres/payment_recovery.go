package postgres

import (
	"context"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *PaymentRepository) PaymentOrderPayable(ctx context.Context, userID int64, number string) (bool, error) {
	var payable bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM orders o JOIN payment_attempts pa ON pa.order_id=o.id JOIN inventory_reservations ir ON ir.order_id=o.id WHERE o.user_id=$1 AND o.order_number=$2 AND o.status='pending_payment' AND NOT o.payment_review_required AND pa.status IN ('created','pending') AND ir.status='active' AND ir.expires_at>NOW())`, userID, number).Scan(&payable)
	return payable, err
}

func (r *PaymentRepository) ClaimPaymentReconciliation(ctx context.Context, limit int) ([]ports.PaymentOrderRecord, error) {
	rows, err := r.pool.Query(ctx, `WITH due AS (
 SELECT id FROM payment_attempts WHERE provider='razorpay' AND provider_order_id IS NOT NULL AND status IN ('pending','authorized','failed') AND next_reconcile_at<=NOW()
 ORDER BY next_reconcile_at,id LIMIT $1 FOR UPDATE SKIP LOCKED
 ), claimed AS (
 UPDATE payment_attempts pa SET next_reconcile_at=NOW()+LEAST(60,power(2,LEAST(reconcile_attempts,6)))*INTERVAL '1 minute',reconcile_attempts=reconcile_attempts+1
 FROM due WHERE pa.id=due.id RETURNING pa.*)
 SELECT o.id,o.order_number,o.status,p.provider,p.provider_order_id,p.status,p.amount_cents,p.currency FROM claimed p JOIN orders o ON o.id=p.order_id`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []ports.PaymentOrderRecord{}
	for rows.Next() {
		var record ports.PaymentOrderRecord
		if err := rows.Scan(&record.OrderID, &record.OrderNumber, &record.OrderStatus, &record.Provider, &record.ProviderOrderID, &record.PaymentStatus, &record.AmountCents, &record.Currency); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
func (r *PaymentRepository) ClaimOrderPaymentReconciliation(ctx context.Context, userID int64, number string) (bool, error) {
	result, err := r.pool.Exec(ctx, `UPDATE payment_attempts pa SET next_reconcile_at=NOW()+INTERVAL '1 minute' FROM orders o WHERE o.id=pa.order_id AND o.user_id=$1 AND o.order_number=$2 AND pa.provider='razorpay' AND pa.provider_order_id IS NOT NULL AND pa.status IN ('pending','authorized','failed') AND pa.next_reconcile_at<=NOW()`, userID, number)
	return result.RowsAffected() == 1, err
}
