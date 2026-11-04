package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

// Caller holds the order and aggregate payment locks. Attempt failures never
// close the payable order or release its reservation; expiry owns that decision.
func applyProviderPayment(ctx context.Context, tx pgx.Tx, orderID int64, event ports.PaymentWebhook) error {
	var attemptID int64
	var orderStatus, aggregateStatus, paymentID, provider string
	if err := tx.QueryRow(ctx, `SELECT pa.id,o.status,pa.status,COALESCE(pa.provider_payment_id,''),pa.provider FROM orders o JOIN payment_attempts pa ON pa.order_id=o.id WHERE o.id=$1 AND pa.provider_order_id=$2 FOR UPDATE OF o,pa`, orderID, event.ProviderOrderID).Scan(&attemptID, &orderStatus, &aggregateStatus, &paymentID, &provider); err != nil {
		return err
	}
	status := "pending"
	switch event.EventType {
	case "payment.failed":
		status = "failed"
	case "payment.authorized":
		status = "authorized"
	case "payment.captured", "order.paid":
		status = "captured"
	}
	if event.ProviderPaymentID == "" {
		return ports.ErrPaymentMismatch
	}
	result, err := tx.Exec(ctx, `INSERT INTO provider_payment_attempts(provider,provider_payment_id,payment_attempt_id,status,amount_cents,currency) VALUES($1,$2,$3,$4,$5,$6)
 ON CONFLICT(provider,provider_payment_id) DO UPDATE SET status=CASE
 WHEN provider_payment_attempts.status IN ('captured','refunded') THEN provider_payment_attempts.status
 WHEN EXCLUDED.status='captured' THEN 'captured'
 WHEN provider_payment_attempts.status='authorized' AND EXCLUDED.status='pending' THEN 'authorized'
 ELSE EXCLUDED.status END,updated_at=NOW()
 WHERE provider_payment_attempts.payment_attempt_id=EXCLUDED.payment_attempt_id AND provider_payment_attempts.amount_cents=EXCLUDED.amount_cents AND provider_payment_attempts.currency=EXCLUDED.currency`, provider, event.ProviderPaymentID, attemptID, status, event.AmountCents, event.Currency)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ports.ErrPaymentMismatch
	}
	if status != "captured" {
		if status == "authorized" {
			_, err = tx.Exec(ctx, `UPDATE payment_attempts SET status='authorized',updated_at=NOW() WHERE id=$1 AND status IN ('created','pending')`, attemptID)
		}
		return err
	}
	if (aggregateStatus == "captured" || aggregateStatus == "refunded") && paymentID == event.ProviderPaymentID {
		return nil
	}
	if aggregateStatus == "captured" || aggregateStatus == "refunded" {
		return openPaymentReview(ctx, tx, orderID, attemptID, event, "additional captured provider payment")
	}
	// Record receipt of money even if stock expired or the order was cancelled.
	if _, err = tx.Exec(ctx, `UPDATE payment_attempts SET status='captured',provider_payment_id=$2,updated_at=NOW() WHERE id=$1`, attemptID, event.ProviderPaymentID); err != nil {
		return err
	}
	var reservationID int64
	err = tx.QueryRow(ctx, `SELECT id FROM inventory_reservations WHERE order_id=$1 AND status='active' AND expires_at>NOW() FOR UPDATE`, orderID).Scan(&reservationID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if orderStatus != "pending_payment" || errors.Is(err, pgx.ErrNoRows) {
		if _, err = releaseReservation(ctx, tx, orderID); err != nil {
			return err
		}
		return openPaymentReview(ctx, tx, orderID, attemptID, event, "capture arrived after the payable stock reservation ended")
	}
	return commitCapturedOrder(ctx, tx, orderID, reservationID, orderStatus)
}

func commitCapturedOrder(ctx context.Context, tx pgx.Tx, orderID, reservationID int64, from string) error {
	result, err := tx.Exec(ctx, `UPDATE orders SET status='paid',payment_review_required=FALSE,updated_at=NOW() WHERE id=$1 AND status=$2`, orderID, from)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ports.ErrPaymentState
	}
	var expected int64
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM inventory_reservation_items WHERE reservation_id=$1`, reservationID).Scan(&expected); err != nil {
		return err
	}
	result, err = tx.Exec(ctx, `UPDATE inventory_stock i SET reserved_quantity=reserved_quantity-ri.quantity,updated_at=NOW() FROM inventory_reservation_items ri WHERE ri.reservation_id=$1 AND i.variant_id=ri.variant_id AND i.reserved_quantity>=ri.quantity`, reservationID)
	if err != nil {
		return err
	}
	if expected < 1 || result.RowsAffected() != expected {
		return ports.ErrInsufficientStock
	}
	result, err = tx.Exec(ctx, `UPDATE inventory_reservations SET status='committed',updated_at=NOW() WHERE id=$1 AND status='active'`, reservationID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ports.ErrPaymentState
	}
	if _, err = tx.Exec(ctx, `INSERT INTO order_status_history(order_id,from_status,to_status,reason) VALUES($1,$2,'paid','verified provider capture')`, orderID, from); err != nil {
		return err
	}
	if err = recordSellerSalesLedger(ctx, tx, orderID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox_events(event_type,aggregate_type,aggregate_id,payload) SELECT 'OrderPaid','Order',order_number,jsonb_build_object('order_number',order_number) FROM orders WHERE id=$1 AND status='paid' ON CONFLICT(event_type,aggregate_id) WHERE event_type='OrderPaid' DO NOTHING`, orderID)
	return err
}

func openPaymentReview(ctx context.Context, tx pgx.Tx, orderID, attemptID int64, event ports.PaymentWebhook, reason string) error {
	// Serialize hold changes with seller-ledger/settlement eligibility decisions.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(4856203)`); err != nil {
		return err
	}
	var id int64
	err := tx.QueryRow(ctx, `INSERT INTO payment_review_cases(order_id,payment_attempt_id,provider_payment_id,amount_cents,currency,reason) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(provider_payment_id) DO NOTHING RETURNING id`, orderID, attemptID, event.ProviderPaymentID, event.AmountCents, event.Currency, reason).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE orders SET payment_review_required=TRUE,updated_at=NOW() WHERE id=$1`, orderID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO notifications(user_id,kind,title,body) SELECT DISTINCT u.id,'payment_review','Captured payment needs review','Order '||o.order_number||' has a captured payment held for finance review. Fulfilment and payout remain blocked.' FROM users u JOIN user_roles ur ON ur.user_id=u.id CROSS JOIN orders o WHERE u.status='active' AND ur.role_slug IN ('admin','super_admin') AND o.id=$1`, orderID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_logs(action,resource_type,resource_id,request_id,metadata) VALUES('payment.review_opened','order',$1,$2,jsonb_build_object('case_id',$3::bigint,'reason',$4::text))`, fmt.Sprint(orderID), ports.RequestID(ctx), id, reason)
	return err
}
