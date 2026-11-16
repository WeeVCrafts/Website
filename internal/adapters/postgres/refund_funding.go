package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

func allocateRefundFunding(ctx context.Context, tx pgx.Tx, order, refund int64) error {
	var total, credit, previous, previousCredit, current, legacy int64
	if err := tx.QueryRow(ctx, `SELECT total_cents,credit_cents,COALESCE((SELECT sum(amount_cents+shipping_cents) FROM marketplace_refunds WHERE order_id=o.id AND id<>$2 AND status IN ('approved','submitting','uncertain','pending','processed')),0),COALESCE((SELECT sum(credit_cents) FROM marketplace_refunds WHERE order_id=o.id AND id<>$2 AND status IN ('approved','submitting','uncertain','pending','processed')),0),(SELECT amount_cents+shipping_cents FROM marketplace_refunds WHERE id=$2 AND order_id=o.id),COALESCE((SELECT sum(amount_cents) FROM payment_refunds WHERE order_id=o.id AND status<>'failed'),0) FROM orders o WHERE o.id=$1`, order, refund).Scan(&total, &credit, &previous, &previousCredit, &current, &legacy); err != nil {
		return err
	}
	if total <= 0 || previous+current+legacy > total {
		return ports.ErrRefundState
	}
	part := (previous+current)*credit/total - previousCredit
	if part < 0 || part > current || previousCredit+part > credit {
		return ports.ErrRefundState
	}
	var cash int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT sum(amount_cents+shipping_cents-credit_cents) FROM marketplace_refunds WHERE order_id=$1 AND id<>$2 AND status IN ('approved','submitting','uncertain','pending','processed')),0)`, order, refund).Scan(&cash); err != nil {
		return err
	}
	if cash+current-part+legacy > total-credit {
		return ports.ErrRefundState
	}
	_, err := tx.Exec(ctx, `UPDATE marketplace_refunds SET credit_cents=$2 WHERE id=$1`, refund, part)
	return err
}
