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
 LEFT JOIN seller_fulfillments sf ON sf.order_id=o.id AND sf.seller_id=p.seller_id
 WHERE p.seller_id=$1 AND (o.status='pending_payment' OR (o.status IN ('paid','processing','shipped') AND COALESCE(sf.status,'pending')<>'delivered'))
 ) OR EXISTS(
 SELECT 1 FROM return_requests rr JOIN orders o ON o.id=rr.order_id JOIN order_items oi ON oi.order_id=o.id JOIN product_variants v ON v.id=oi.variant_id JOIN products p ON p.id=v.product_id
 WHERE p.seller_id=$1 AND (rr.status IN ('pending','approved') OR (rr.status='received' AND o.status<>'refunded'))
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
