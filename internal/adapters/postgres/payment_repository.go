package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wecratfs/commerce/internal/ports"
)

type PaymentRepository struct{ pool *pgxpool.Pool }

func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool}
}

func (r *PaymentRepository) GetPaymentOrder(ctx context.Context, userID int64, orderNumber string) (ports.PaymentOrderRecord, error) {
	var record ports.PaymentOrderRecord
	err := r.pool.QueryRow(ctx, `
		SELECT o.id, o.order_number, o.status, pa.provider,
		       COALESCE(pa.provider_order_id, ''), pa.status,
		       pa.amount_cents, pa.currency
		FROM orders o
		JOIN payment_attempts pa ON pa.order_id = o.id
		WHERE o.user_id = $1 AND o.order_number = $2
		ORDER BY pa.id DESC
		LIMIT 1`, userID, orderNumber).Scan(
		&record.OrderID,
		&record.OrderNumber,
		&record.OrderStatus,
		&record.Provider,
		&record.ProviderOrderID,
		&record.PaymentStatus,
		&record.AmountCents,
		&record.Currency,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.PaymentOrderRecord{}, ports.ErrPaymentNotFound
	}
	return record, err
}

func (r *PaymentRepository) AttachProviderOrder(ctx context.Context, userID int64, orderNumber string, providerOrder ports.PaymentOrder) (ports.PaymentOrderRecord, error) {
	return r.attachProviderOrder(ctx, userID, orderNumber, 0, providerOrder)
}

func (r *PaymentRepository) attachProviderOrder(ctx context.Context, userID int64, number string, attemptID int64, providerOrder ports.PaymentOrder) (ports.PaymentOrderRecord, error) {
	var record ports.PaymentOrderRecord
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT id,order_number,status FROM orders
			WHERE user_id=$1 AND order_number=$2 FOR UPDATE`, userID, number).
			Scan(&record.OrderID, &record.OrderNumber, &record.OrderStatus); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ports.ErrPaymentNotFound
			}
			return err
		}
		var latestID int64
		var started, review, payable bool
		if err := tx.QueryRow(ctx, `SELECT pa.id,pa.provider,COALESCE(pa.provider_order_id,''),pa.status,pa.amount_cents,pa.currency,
			pa.creation_started_at IS NOT NULL,pa.creation_review_reason<>'',`+paymentOrderPayableSQL+`
			FROM orders o JOIN payment_attempts pa ON pa.order_id=o.id WHERE o.id=$1
			ORDER BY pa.id DESC LIMIT 1 FOR UPDATE OF pa`, record.OrderID).
			Scan(&latestID, &record.Provider, &record.ProviderOrderID, &record.PaymentStatus, &record.AmountCents, &record.Currency,
				&started, &review, &payable); err != nil {
			return err
		}
		if record.Provider != "razorpay" || review || (attemptID == 0 && (started || !payable)) ||
			(attemptID != 0 && (attemptID != latestID || !started)) {
			return ports.ErrPaymentState
		}
		if record.ProviderOrderID != "" {
			if attemptID != 0 && record.ProviderOrderID != providerOrder.ProviderOrderID {
				return ports.ErrPaymentMismatch
			}
			return nil
		}
		if (record.PaymentStatus != "created" && (attemptID == 0 || record.PaymentStatus != "failed")) || providerOrder.ProviderOrderID == "" ||
			record.AmountCents != providerOrder.AmountCents || record.Currency != providerOrder.Currency {
			return ports.ErrPaymentMismatch
		}
		// Associate a recovered order even after stock expiry or cancellation.
		// Signed callbacks still need the reference to record a late capture.
		if _, err := tx.Exec(ctx, `UPDATE payment_attempts SET provider_order_id=$2,status='pending',
			creation_next_lookup_at=NULL,updated_at=NOW(),next_reconcile_at=NOW() WHERE id=$1`, latestID, providerOrder.ProviderOrderID); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ports.ErrPaymentMismatch
			}
			return err
		}
		record.ProviderOrderID, record.PaymentStatus = providerOrder.ProviderOrderID, "pending"
		return nil
	})
	return record, err
}

func (r *PaymentRepository) ProcessWebhook(ctx context.Context, webhook ports.PaymentWebhook, payload []byte) (bool, error) {
	if webhook.EventType == "refund.created" || webhook.EventType == "refund.processed" || webhook.EventType == "refund.failed" {
		return r.processRefundWebhook(ctx, webhook, payload)
	}
	returnValue := false
	err := WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		var inserted bool
		if err := tx.QueryRow(transactionContext, `INSERT INTO payment_webhook_events (event_id, event_type, payload) VALUES ($1, $2, $3) ON CONFLICT (event_id) DO NOTHING RETURNING TRUE`, webhook.EventID, webhook.EventType, payload).Scan(&inserted); errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
		returnValue = inserted
		if webhook.ProviderOrderID == "" {
			_, err := tx.Exec(transactionContext, `UPDATE payment_webhook_events SET processed_at = NOW() WHERE event_id = $1`, webhook.EventID)
			return err
		}
		var orderID, expectedAmount int64
		var expectedCurrency string
		if err := tx.QueryRow(transactionContext, `SELECT o.id, pa.amount_cents, pa.currency FROM orders o JOIN payment_attempts pa ON pa.order_id=o.id WHERE pa.provider_order_id = $1 FOR UPDATE OF o, pa`, webhook.ProviderOrderID).Scan(&orderID, &expectedAmount, &expectedCurrency); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				_, updateErr := tx.Exec(transactionContext, `UPDATE payment_webhook_events SET processed_at = NOW() WHERE event_id = $1`, webhook.EventID)
				return updateErr
			}
			return err
		}
		if webhook.AmountCents != expectedAmount || webhook.Currency != expectedCurrency {
			metadata, _ := json.Marshal(map[string]any{
				"event_type": webhook.EventType, "provider_order_id": webhook.ProviderOrderID,
				"expected_amount_cents": expectedAmount, "received_amount_cents": webhook.AmountCents,
				"expected_currency": expectedCurrency, "received_currency": webhook.Currency,
			})
			if _, err := tx.Exec(transactionContext, `INSERT INTO audit_logs (action, resource_type, resource_id, request_id, metadata) VALUES ('payment.webhook_rejected_mismatch', 'order', $1, $2, $3)`, fmt.Sprint(orderID), ports.RequestID(transactionContext), metadata); err != nil {
				return err
			}
			_, err := tx.Exec(transactionContext, `UPDATE payment_webhook_events SET processed_at = NOW() WHERE event_id = $1`, webhook.EventID)
			return err
		}
		if err := applyProviderPayment(transactionContext, tx, orderID, webhook); err != nil {
			return err
		}
		metadata, _ := json.Marshal(map[string]string{"event_type": webhook.EventType, "provider_order_id": webhook.ProviderOrderID})
		if _, err := tx.Exec(transactionContext, `INSERT INTO audit_logs (action, resource_type, resource_id, request_id, metadata) VALUES ('payment.webhook_processed', 'order', $1, $2, $3)`, fmt.Sprint(orderID), ports.RequestID(transactionContext), metadata); err != nil {
			return err
		}
		_, err := tx.Exec(transactionContext, `UPDATE payment_webhook_events SET processed_at = NOW() WHERE event_id = $1`, webhook.EventID)
		return err
	})
	return returnValue, err
}

func releaseReservation(ctx context.Context, tx pgx.Tx, orderID int64) (bool, error) {
	var reservationID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM inventory_reservations WHERE order_id = $1 AND status = 'active' FOR UPDATE`, orderID).Scan(&reservationID); errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return releaseReservationWithStatus(ctx, tx, reservationID, "released")
}

func releaseReservationWithStatus(ctx context.Context, tx pgx.Tx, reservationID int64, finalStatus string) (bool, error) {
	rows, err := tx.Query(ctx, `SELECT variant_id, quantity FROM inventory_reservation_items WHERE reservation_id = $1`, reservationID)
	if err != nil {
		return false, err
	}
	type reservationItem struct {
		variantID int64
		quantity  int
	}
	items := make([]reservationItem, 0, 4)
	for rows.Next() {
		var variantID int64
		var quantity int
		if err := rows.Scan(&variantID, &quantity); err != nil {
			rows.Close()
			return false, err
		}
		items = append(items, reservationItem{variantID: variantID, quantity: quantity})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	rows.Close()
	for _, item := range items {
		if _, err := tx.Exec(ctx, `UPDATE inventory_stock SET available_quantity = available_quantity + $2, reserved_quantity = reserved_quantity - $2, updated_at = NOW() WHERE variant_id = $1`, item.variantID, item.quantity); err != nil {
			return false, err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE inventory_reservations SET status = $2, updated_at = NOW() WHERE id = $1 AND status = 'active'`, reservationID, finalStatus)
	return true, err
}
