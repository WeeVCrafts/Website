package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *MarketplaceRepository) RefundRecord(ctx context.Context, id int64) (ports.PaymentRefundRecord, bool, error) {
	var record ports.PaymentRefundRecord
	var submit bool
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var order int64
		if err := tx.QueryRow(ctx, `SELECT o.id FROM orders o JOIN marketplace_refunds mr ON mr.order_id=o.id WHERE mr.id=$1 FOR UPDATE OF o`, id).Scan(&order); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `SELECT mr.id,mr.order_id,o.order_number,mr.amount_cents+mr.shipping_cents-mr.credit_cents,o.currency,COALESCE(pa.provider_payment_id,''),COALESCE(mr.provider_refund_id,''),mr.status FROM marketplace_refunds mr JOIN orders o ON o.id=mr.order_id JOIN LATERAL(SELECT provider_payment_id,status FROM payment_attempts WHERE order_id=o.id ORDER BY id DESC LIMIT 1) pa ON true WHERE mr.id=$1 FOR UPDATE OF mr`, id).Scan(&record.ID, &record.OrderID, &record.OrderNumber, &record.AmountCents, &record.Currency, &record.ProviderPaymentID, &record.ProviderRefundID, &record.Status)
		if err != nil {
			return err
		}
		var attempt int
		if err = tx.QueryRow(ctx, `SELECT submission_attempt FROM marketplace_refunds WHERE id=$1`, id).Scan(&attempt); err != nil {
			return err
		}
		record.Receipt = fmt.Sprintf("mp-refund-%d", id)
		if attempt > 1 {
			record.Receipt = fmt.Sprintf("mp-refund-%d-a%d", id, attempt)
		}
		if record.Status == "processed" {
			return nil
		}
		if record.Status == "requested" || record.Status == "rejected" || (record.ProviderPaymentID == "" && record.AmountCents > 0) {
			return ports.ErrRefundState
		}
		submit = record.Status == "approved"
		if submit {
			_, err = tx.Exec(ctx, `UPDATE marketplace_refunds SET status='submitting',updated_at=now() WHERE id=$1`, id)
		}
		return err
	})
	return record, submit, err
}

func (r *MarketplaceRepository) SavePartialRefund(ctx context.Context, id int64, refund ports.PaymentRefund) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var order, seller, item, amount, shipping, credit int64
		var quantity int
		var currency, paymentID, previous, providerID string
		if err := tx.QueryRow(ctx, `SELECT o.id FROM orders o JOIN marketplace_refunds mr ON mr.order_id=o.id WHERE mr.id=$1 FOR UPDATE OF o`, id).Scan(&order); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(4856203)`); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `SELECT mr.seller_id,COALESCE(mr.order_item_id,0),mr.quantity,mr.amount_cents,mr.shipping_cents,mr.credit_cents,o.currency,COALESCE(pa.provider_payment_id,''),mr.status,COALESCE(mr.provider_refund_id,'') FROM marketplace_refunds mr JOIN orders o ON o.id=mr.order_id JOIN LATERAL(SELECT provider_payment_id FROM payment_attempts WHERE order_id=o.id ORDER BY id DESC LIMIT 1) pa ON true WHERE mr.id=$1 FOR UPDATE OF mr`, id).Scan(&seller, &item, &quantity, &amount, &shipping, &credit, &currency, &paymentID, &previous, &providerID)
		if err != nil {
			return err
		}
		if (refund.ProviderRefundID == "" && amount+shipping-credit > 0) || refund.ProviderPaymentID != paymentID || refund.AmountCents != amount+shipping-credit || (refund.Currency != "" && refund.Currency != currency) || (providerID != "" && providerID != refund.ProviderRefundID) {
			return ports.ErrPaymentMismatch
		}
		if previous == "processed" {
			return nil
		}
		if previous != "submitting" && previous != "uncertain" && previous != "pending" {
			return ports.ErrRefundState
		}
		if refund.Status != "processed" && refund.Status != "pending" && refund.Status != "failed" {
			return ports.ErrRefundState
		}
		if _, err = tx.Exec(ctx, `UPDATE marketplace_refunds SET status=$2,provider_refund_id=NULLIF($3,''),updated_at=now() WHERE id=$1`, id, refund.Status, refund.ProviderRefundID); err != nil {
			return err
		}
		if refund.Status != "processed" {
			return nil
		}
		if err = restoreOrderCredit(ctx, tx, order, credit, fmt.Sprintf("refund:%d", id)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `SELECT marketplace_enqueue('credit_note',$1,$2)`, id, fmt.Sprintf("credit-note:%d", id)); err != nil {
			return err
		}
		var total, refunded int
		var commission int64
		if err = tx.QueryRow(ctx, `SELECT oi.quantity,oi.financially_reversed_quantity,COALESCE((SELECT (line->>'commission_cents')::bigint FROM orders o CROSS JOIN LATERAL jsonb_array_elements(o.seller_finance_snapshot->'lines') line WHERE o.id=oi.order_id AND (line->>'order_item_id')::bigint=oi.id),0) FROM order_items oi WHERE oi.id=$1 FOR UPDATE`, item).Scan(&total, &refunded, &commission); err != nil {
			return err
		}
		commissionRefund, e := domain.AllocateRefund(commission, total, refunded, quantity)
		if e != nil {
			return e
		}
		if _, err = tx.Exec(ctx, `INSERT INTO seller_ledger(seller_id,order_id,entry_type,gross_cents,commission_cents,net_cents,status,note,marketplace_refund_id) VALUES($1,$2,'refund',$3,$4,$5,'open',$6,$7)`, seller, order, -amount, -commissionRefund, -(amount - commissionRefund), fmt.Sprintf("Item refund %d", id), id); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE order_items SET refunded_quantity=refunded_quantity+$2,financially_reversed_quantity=financially_reversed_quantity+$2 WHERE id=$1 AND refunded_quantity+$2<=quantity AND financially_reversed_quantity+$2<=quantity`, item, quantity); err != nil {
			return err
		}
		// The captured status remains the fulfillment gate for unrefunded items.
		// The independent refunded amount projects partial/full payment refund state.
		if _, err = tx.Exec(ctx, `UPDATE payment_attempts pa SET refunded_amount_cents=COALESCE((SELECT sum(amount_cents+shipping_cents-credit_cents) FROM marketplace_refunds WHERE order_id=pa.order_id AND status='processed'),0),updated_at=now() WHERE order_id=$1`, order); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE payment_attempts SET status='refunded' WHERE order_id=$1 AND EXISTS(SELECT 1 FROM orders o WHERE o.id=payment_attempts.order_id AND o.total_cents=(SELECT COALESCE(sum(amount_cents+shipping_cents),0) FROM marketplace_refunds WHERE order_id=o.id AND status='processed'))`, order); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE orders SET status='refunded',updated_at=now() WHERE id=$1 AND EXISTS(SELECT 1 FROM payment_attempts WHERE order_id=$1 AND status='refunded')`, order); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE marketplace_cases SET status='resolved',resolution='All requested refunds reconciled',resolved_at=now() WHERE order_id=$1 AND seller_id=$2 AND kind='refund_request' AND NOT EXISTS(SELECT 1 FROM marketplace_refunds WHERE order_id=$1 AND seller_id=$2 AND status NOT IN ('processed','rejected'))`, order, seller); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_logs(action,resource_type,resource_id,metadata) VALUES('marketplace.refund_processed','marketplace_refund',$1,jsonb_build_object('amount_cents',$2::bigint,'provider_refund_id',$3::text))`, fmt.Sprint(id), amount+shipping, refund.ProviderRefundID)
		return err
	})
}

// RefundCallback matches an already approved partial refund. Unknown receipts
// remain available to the legacy payment/refund handler.
func (r *MarketplaceRepository) RefundCallback(ctx context.Context, event ports.PaymentWebhook) (bool, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `SELECT id FROM marketplace_refunds WHERE provider_refund_id=$1 OR ((CASE WHEN submission_attempt=1 THEN 'mp-refund-'||id::text ELSE 'mp-refund-'||id::text||'-a'||submission_attempt::text END)=$2 AND status IN ('submitting','uncertain','pending'))`, event.ProviderRefundID, event.RefundReceipt).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	err = r.SavePartialRefund(ctx, id, ports.PaymentRefund{ProviderRefundID: event.ProviderRefundID, ProviderPaymentID: event.ProviderPaymentID, AmountCents: event.AmountCents, Currency: event.Currency, Status: event.Status})
	if errors.Is(err, ports.ErrShippingPending) {
		err = nil
	}
	return true, err
}
