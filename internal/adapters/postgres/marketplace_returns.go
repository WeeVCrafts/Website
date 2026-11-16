package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"strings"
)

func marketplaceReturnAction(ctx context.Context, tx pgx.Tx, actor, order int64, a marketplaceActor, c ports.MarketplaceCommand) error {
	if len(strings.TrimSpace(c.Reason)) < 10 {
		return ports.ErrInvalidState
	}
	if c.Action == "request_return" {
		if a.role != "customer" && !a.company() {
			return ports.ErrForbidden
		}
		if c.Quantity < 1 || len(c.Key) < 16 {
			return ports.ErrInvalidState
		}
		var exists, match bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_returns WHERE request_key=$1),EXISTS(SELECT 1 FROM marketplace_returns WHERE request_key=$1 AND order_id=$2 AND order_item_id=$3 AND quantity=$4 AND reason=$5)`, c.Key, order, c.ItemID, c.Quantity, c.Reason).Scan(&exists, &match); err != nil {
			return err
		}
		if exists {
			if match {
				return nil
			}
			return ports.ErrIdempotencyConflict
		}
		var seller int64
		var remaining int
		if err := tx.QueryRow(ctx, `SELECT oi.seller_id,oi.quantity-oi.cancelled_quantity-COALESCE((SELECT sum(quantity) FROM marketplace_returns WHERE order_item_id=oi.id AND status<>'rejected'),0) FROM order_items oi WHERE oi.id=$1 AND oi.order_id=$2 AND EXISTS(SELECT 1 FROM seller_fulfillments WHERE order_id=$2 AND seller_id=oi.seller_id AND status='delivered') FOR UPDATE`, c.ItemID, order).Scan(&seller, &remaining); err != nil {
			return err
		}
		if c.Quantity > remaining {
			return ports.ErrInvalidState
		}
		if _, err := tx.Exec(ctx, `INSERT INTO marketplace_returns(order_id,seller_id,order_item_id,quantity,reason,request_key) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(request_key) DO NOTHING`, order, seller, c.ItemID, c.Quantity, c.Reason, c.Key); err != nil {
			return err
		}
		return marketplaceCase(ctx, tx, order, seller, 0, "return_dispute", c.Reason)
	}
	if !a.company() {
		return ports.ErrForbidden
	}
	var previous string
	var item, variant, seller int64
	var quantity int
	var restocked bool
	if err := tx.QueryRow(ctx, `SELECT mr.status,mr.order_item_id,oi.variant_id,mr.seller_id,mr.quantity,mr.restocked FROM marketplace_returns mr JOIN order_items oi ON oi.id=mr.order_item_id WHERE mr.id=$1 AND mr.order_id=$2 FOR UPDATE OF mr`, c.CaseID, order).Scan(&previous, &item, &variant, &seller, &quantity, &restocked); err != nil {
		return err
	}
	if previous == c.Status {
		return nil
	}
	if !domain.CanTransition("return", previous, c.Status) {
		return ports.ErrInvalidState
	}
	if c.Status == "pickup_pending" && (len(c.Carrier) < 2 || len(c.Carrier) > 80 || len(c.Tracking) < 3 || len(c.Tracking) > 120) {
		return ports.ErrShippingInput
	}
	if c.Status == "completed" && c.Restock && !restocked {
		if _, err := tx.Exec(ctx, `UPDATE inventory_stock SET available_quantity=available_quantity+$2,updated_at=now() WHERE variant_id=$1`, variant, quantity); err != nil {
			return err
		}
		restocked = true
	}
	if _, err := tx.Exec(ctx, `UPDATE marketplace_returns SET status=$2,reason=$3,carrier=COALESCE(NULLIF($4,''),carrier),tracking_number=COALESCE(NULLIF($5,''),tracking_number),restocked=$6,updated_at=now() WHERE id=$1`, c.CaseID, c.Status, c.Reason, c.Carrier, c.Tracking, restocked); err != nil {
		return err
	}
	if c.Status == "completed" {
		// Completion creates a refund request, never an unapproved money movement.
		c.ItemID, c.Quantity, c.Key = item, quantity, "return-refund:"+c.Key
		if err := requestExceptionRefund(ctx, tx, order, c); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO marketplace_history(order_id,seller_id,system,from_status,to_status,actor_id,reason) VALUES($1,$2,'return',$3,$4,$5,$6)`, order, seller, previous, c.Status, actor, c.Reason)
	return err
}
