package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *PaymentRepository) processRefundWebhook(ctx context.Context, event ports.PaymentWebhook, payload []byte) (bool, error) {
	var seen bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM payment_webhook_events WHERE event_id=$1 AND processed_at IS NOT NULL)`, event.EventID).Scan(&seen); err != nil || seen {
		return false, err
	}
	var id int64
	err := r.pool.QueryRow(ctx, `SELECT pr.id FROM payment_refunds pr
	JOIN LATERAL (SELECT provider_payment_id FROM payment_attempts WHERE order_id=pr.order_id ORDER BY id DESC LIMIT 1) pa ON TRUE
	WHERE pa.provider_payment_id=$1 AND pr.amount_cents=$2 AND pr.currency=$3
	AND (pr.provider_refund_id=$4 OR (pr.provider_refund_id IS NULL AND 'refund-'||pr.id::text=$5))`, event.ProviderPaymentID, event.AmountCents, event.Currency, event.ProviderRefundID, event.RefundReceipt).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = r.CompleteRefund(ctx, id, ports.PaymentRefund{ProviderRefundID: event.ProviderRefundID, ProviderPaymentID: event.ProviderPaymentID, AmountCents: event.AmountCents, Currency: event.Currency, Status: event.Status})
	if err != nil {
		return false, err
	}
	// Completion is transactional and replay-safe. If recording the delivery
	// fails, a retry repeats completion without duplicating financial entries.
	result, err := r.pool.Exec(ctx, `INSERT INTO payment_webhook_events(event_id,event_type,payload,processed_at) VALUES($1,$2,$3,NOW()) ON CONFLICT(event_id) DO NOTHING`, event.EventID, event.EventType, payload)
	return result.RowsAffected() == 1, err
}
