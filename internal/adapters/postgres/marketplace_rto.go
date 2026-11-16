package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

func inspectMarketplaceRTO(ctx context.Context, tx pgx.Tx, order int64, c ports.MarketplaceCommand) error {
	var inspected bool
	if err := tx.QueryRow(ctx, `SELECT rto_inspected_at IS NOT NULL FROM marketplace_packages WHERE id=$1 AND order_id=$2 AND status='rto' AND provider_status='RTO DELIVERED' FOR UPDATE`, c.PackageID, order).Scan(&inspected); err != nil {
		return err
	}
	if inspected {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT oi.id,oi.variant_id,pi.quantity,GREATEST(0,oi.quantity-COALESCE((SELECT sum(quantity) FROM marketplace_refunds WHERE order_item_id=oi.id AND status<>'rejected'),0)) FROM marketplace_package_items pi JOIN order_items oi ON oi.id=pi.order_item_id WHERE package_id=$1 ORDER BY oi.id`, c.PackageID)
	if err != nil {
		return err
	}
	type line struct {
		item, variant        int64
		quantity, refundable int
	}
	var lines []line
	for rows.Next() {
		var l line
		if err = rows.Scan(&l.item, &l.variant, &l.quantity, &l.refundable); err != nil {
			break
		}
		lines = append(lines, l)
	}
	if err = errorsJoinRows(err, rows); err != nil {
		return err
	}
	for _, l := range lines {
		if c.Restock {
			if _, err = tx.Exec(ctx, `UPDATE inventory_stock SET available_quantity=available_quantity+$2,updated_at=now() WHERE variant_id=$1`, l.variant, l.quantity); err != nil {
				return err
			}
		}
		qty := min(l.quantity, l.refundable)
		if qty > 0 {
			refund := c
			refund.ItemID = l.item
			refund.Quantity = qty
			refund.Key = fmt.Sprintf("rto-refund:%d:%d", c.PackageID, l.item)
			if err = requestExceptionRefund(ctx, tx, order, refund); err != nil {
				return err
			}
		}
	}
	_, err = tx.Exec(ctx, `UPDATE marketplace_packages SET rto_inspected_at=now() WHERE id=$1`, c.PackageID)
	return err
}
