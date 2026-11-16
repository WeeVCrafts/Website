package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

// Lock the seller before inspecting obligations; checkout uses this same row lock.
// The caller holds the identity administration lock and changes roles atomically.
func guardSellerRoleExit(ctx context.Context, tx pgx.Tx, userID int64) error {
	var sellerID int64
	err := tx.QueryRow(ctx, `SELECT id FROM sellers WHERE owner_user_id=$1 FOR UPDATE`, userID).Scan(&sellerID)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var unfinished bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM orders o JOIN order_items oi ON oi.order_id=o.id JOIN product_variants v ON v.id=oi.variant_id JOIN products p ON p.id=v.product_id
 LEFT JOIN seller_fulfillments sf ON sf.order_id=o.id AND sf.seller_id=COALESCE(oi.seller_id,p.seller_id)
 WHERE COALESCE(oi.seller_id,p.seller_id)=$1 AND (o.status='pending_payment'
 OR (o.workflow_version=1 AND o.status IN ('paid','processing','shipped') AND COALESCE(sf.status,'pending')<>'delivered')
 OR (o.workflow_version>=2 AND EXISTS(SELECT 1 FROM payment_attempts pa WHERE pa.order_id=o.id AND pa.status='captured') AND COALESCE(sf.lifecycle_status,'new') NOT IN ('delivered','cancelled')))
 ) OR EXISTS(
 SELECT 1 FROM return_requests rr JOIN orders o ON o.id=rr.order_id JOIN order_items oi ON oi.order_id=o.id JOIN product_variants v ON v.id=oi.variant_id JOIN products p ON p.id=v.product_id
 WHERE p.seller_id=$1 AND (rr.status IN ('pending','approved') OR (rr.status='received' AND o.status<>'refunded'))
 ) OR EXISTS(
 SELECT 1 FROM marketplace_returns WHERE seller_id=$1 AND status NOT IN ('rejected','completed')
 ) OR EXISTS(
 SELECT 1 FROM marketplace_refunds WHERE seller_id=$1 AND status NOT IN ('processed','rejected')
 ) OR EXISTS(
 SELECT 1 FROM marketplace_cases c WHERE c.status='open' AND (c.seller_id=$1 OR (c.seller_id IS NULL AND EXISTS(
 SELECT 1 FROM order_items oi WHERE oi.order_id=c.order_id AND oi.seller_id=$1)))
 )`, sellerID).Scan(&unfinished)
	if err != nil {
		return err
	}
	if unfinished {
		return ports.ErrSellerPendingWork
	}
	_, err = tx.Exec(ctx, `UPDATE sellers SET status='suspended',updated_at=NOW() WHERE id=$1 AND status<>'closed'`, sellerID)
	return err
}
