package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *CommerceRepository) ListSellerOrders(ctx context.Context, userID int64) ([]domaincommerce.SellerOrder, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT o.id, s.id, o.order_number, o.status, COALESCE(sf.status, 'pending'), o.currency,
		       COALESCE(sf.carrier, ''), COALESCE(sf.tracking_number, ''), COALESCE(sf.last_note, ''), COALESCE(sf.revision, 0),
		       o.created_at, oi.product_name, oi.sku, oi.quantity, oi.unit_price_cents, oi.line_total_cents
		FROM orders o
		JOIN order_items oi ON oi.order_id = o.id
		JOIN product_variants pv ON pv.id = oi.variant_id
		JOIN products p ON p.id = pv.product_id
		JOIN sellers s ON s.id = p.seller_id AND s.status = 'active'
		LEFT JOIN seller_fulfillments sf ON sf.order_id = o.id AND sf.seller_id = s.id
		WHERE o.status IN ('paid', 'processing', 'shipped', 'delivered', 'pending_payment')
		  AND (s.owner_user_id = $1 )
		ORDER BY o.created_at DESC, o.id DESC, s.id, oi.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type orderKey struct{ orderID, sellerID int64 }
	orders := make([]domaincommerce.SellerOrder, 0)
	indexes := make(map[orderKey]int)
	for rows.Next() {
		var order domaincommerce.SellerOrder
		var item domaincommerce.SellerOrderItem
		if err := rows.Scan(&order.ID, &order.SellerID, &order.OrderNumber, &order.OverallStatus, &order.FulfillmentStatus, &order.Currency, &order.Carrier, &order.TrackingNumber, &order.LastNote, &order.FulfillmentRevision, &order.CreatedAt, &item.ProductName, &item.SKU, &item.Quantity, &item.UnitPriceCents, &item.LineTotalCents); err != nil {
			return nil, err
		}
		key := orderKey{orderID: order.ID, sellerID: order.SellerID}
		index, ok := indexes[key]
		if !ok {
			order.Items = make([]domaincommerce.SellerOrderItem, 0, 2)
			orders = append(orders, order)
			index = len(orders) - 1
			indexes[key] = index
		}
		orders[index].Items = append(orders[index].Items, item)
		orders[index].SellerSubtotalCents += item.LineTotalCents
	}
	return orders, rows.Err()
}

func (r *CommerceRepository) LookupSellerDispatchContact(ctx context.Context, sellerID int64, orderNumber string) (ports.SellerDispatchContact, error) {
	var contact ports.SellerDispatchContact
	err := r.pool.QueryRow(ctx, `SELECT u.email,u.display_name FROM orders o JOIN users u ON u.id=o.user_id WHERE o.order_number=$1 AND EXISTS(SELECT 1 FROM order_items oi JOIN product_variants v ON v.id=oi.variant_id JOIN products p ON p.id=v.product_id WHERE oi.order_id=o.id AND p.seller_id=$2)`, orderNumber, sellerID).Scan(&contact.Email, &contact.DisplayName)
	return contact, err
}

func (r *CommerceRepository) UpdateSellerFulfillment(ctx context.Context, userID, sellerID int64, orderNumber, status, carrier, trackingNumber, note string) error {
	return r.UpdateSellerFulfillmentWithRevision(ctx, userID, sellerID, orderNumber, status, carrier, trackingNumber, note, -1)
}

func (r *CommerceRepository) UpdateSellerFulfillmentWithRevision(ctx context.Context, userID, sellerID int64, orderNumber, status, carrier, trackingNumber, note string, expectedRevision int64) error {
	return WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		var orderID int64
		var currentStatus, overallStatus string
		var revision int64
		err := tx.QueryRow(transactionContext, `
			SELECT o.id, COALESCE(sf.status, 'pending'), o.status, COALESCE(sf.revision, 0)
			FROM orders o
			JOIN sellers s ON s.id = $2 AND s.status = 'active'
			LEFT JOIN seller_fulfillments sf ON sf.order_id = o.id AND sf.seller_id = s.id
			WHERE o.order_number = $1 AND s.id = $2
			  AND EXISTS (
				SELECT 1 FROM order_items oi
				JOIN product_variants pv ON pv.id = oi.variant_id
				JOIN products p ON p.id = pv.product_id
				WHERE oi.order_id = o.id AND p.seller_id = s.id
			  )
			  AND (s.owner_user_id = $3 )
			FOR UPDATE OF o`, orderNumber, sellerID, userID).Scan(&orderID, &currentStatus, &overallStatus, &revision)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrOrderNotFound
		}
		if err != nil {
			return err
		}
		var lockedRevision int64
		if lockErr := tx.QueryRow(transactionContext, `SELECT revision FROM seller_fulfillments WHERE seller_id=$1 AND order_id=$2 FOR UPDATE`, sellerID, orderID).Scan(&lockedRevision); lockErr == nil {
			revision = lockedRevision
		}
		if expectedRevision >= 0 && revision != expectedRevision {
			return ports.ErrFulfillmentConflict
		}
		if overallStatus != "paid" && overallStatus != "processing" && overallStatus != "shipped" && overallStatus != "delivered" {
			return ports.ErrInvalidState
		}
		currentStage, currentOK := domaincommerce.ParseFulfillmentStage(currentStatus)
		nextStage, nextOK := domaincommerce.ParseFulfillmentStage(status)
		if !currentOK || !nextOK || !currentStage.CanAdvanceTo(nextStage) {
			return ports.ErrInvalidState
		}
		if currentStatus == status {
			var existingCarrier, existingTracking, existingNote string
			if err := tx.QueryRow(transactionContext, `SELECT COALESCE(carrier,''),COALESCE(tracking_number,''),COALESCE(last_note,'') FROM seller_fulfillments WHERE seller_id=$1 AND order_id=$2`, sellerID, orderID).Scan(&existingCarrier, &existingTracking, &existingNote); err == nil && existingCarrier == carrier && existingTracking == trackingNumber && existingNote == note {
				return nil
			}
		}
		if _, err := tx.Exec(transactionContext, `
			INSERT INTO seller_fulfillments (seller_id, order_id, status, carrier, tracking_number, last_note, updated_by, revision)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 1)
			ON CONFLICT (seller_id, order_id) DO UPDATE SET status = EXCLUDED.status, carrier = EXCLUDED.carrier, tracking_number = EXCLUDED.tracking_number, last_note = EXCLUDED.last_note, updated_by = EXCLUDED.updated_by, revision = seller_fulfillments.revision + 1, updated_at = NOW()`, sellerID, orderID, status, carrier, trackingNumber, note, userID); err != nil {
			return err
		}
		// Keep the customer-facing order lifecycle in sync with seller fulfilment.
		// An order may contain products from multiple sellers, so only promote it
		// to shipped or delivered when every seller's fulfilment has reached that
		// stage.
		var aggregatedStatus string
		if err := tx.QueryRow(transactionContext, `
			UPDATE orders o
			SET status = CASE
				WHEN NOT EXISTS (
					SELECT 1
					FROM order_items oi
					JOIN product_variants pv ON pv.id = oi.variant_id
					JOIN products p ON p.id = pv.product_id
					WHERE oi.order_id = o.id
					  AND NOT EXISTS (
						SELECT 1 FROM seller_fulfillments sf
						WHERE sf.order_id = o.id AND sf.seller_id = p.seller_id AND sf.status = 'delivered'
					  )
				) THEN 'delivered'
				WHEN NOT EXISTS (
					SELECT 1
					FROM order_items oi
					JOIN product_variants pv ON pv.id = oi.variant_id
					JOIN products p ON p.id = pv.product_id
					WHERE oi.order_id = o.id
					  AND NOT EXISTS (
						SELECT 1 FROM seller_fulfillments sf
						WHERE sf.order_id = o.id AND sf.seller_id = p.seller_id AND sf.status IN ('shipped', 'out_for_delivery', 'delivered')
					  )
				) THEN 'shipped'
				WHEN EXISTS (
					SELECT 1
					FROM seller_fulfillments sf
					WHERE sf.order_id = o.id AND sf.status IN ('processing', 'packing', 'shipped', 'out_for_delivery', 'delivered')
				) THEN 'processing'
				ELSE o.status
			END
			WHERE o.id = $1
			RETURNING o.status`, orderID).Scan(&aggregatedStatus); err != nil {
			return err
		}
		if aggregatedStatus != overallStatus {
			if _, err := tx.Exec(transactionContext, `INSERT INTO order_status_history (order_id, from_status, to_status, actor_id, reason) VALUES ($1,$2,$3,$4,$5)`, orderID, overallStatus, aggregatedStatus, userID, "seller fulfilment update"); err != nil {
				return err
			}
		}
		metadata, _ := json.Marshal(map[string]any{
			"seller_id": sellerID, "order_id": orderID, "order_number": orderNumber,
			"from_status": currentStatus, "to_status": status, "carrier": carrier,
			"tracking_number": trackingNumber, "note": note,
		})
		if _, err := tx.Exec(transactionContext, `INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, request_id, metadata) VALUES ($1, 'seller.order_fulfillment_updated', 'seller_fulfillment', $2, $3, $4)`, fmt.Sprint(userID), fmt.Sprintf("%d:%d", sellerID, orderID), ports.RequestID(transactionContext), metadata); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"seller_id": sellerID, "order_id": orderID, "order_number": orderNumber, "status": status, "revision": revision + 1})
		_, err = tx.Exec(transactionContext, `INSERT INTO outbox_events (event_type, aggregate_type, aggregate_id, payload) VALUES ('SellerFulfillmentUpdated', 'SellerFulfillment', $1, $2)`, fmt.Sprintf("%d:%d", sellerID, orderID), payload)
		if err != nil {
			return err
		}
		sourceKey := fmt.Sprintf("fulfillment:%d:%d:%d", orderID, sellerID, revision+1)
		if _, err := tx.Exec(transactionContext, `
			INSERT INTO notifications (user_id, kind, title, body, link_url, source_key)
			SELECT target.user_id, 'order', 'Delivery update', $1,
			       CASE WHEN target.company THEN '/admin/orders?order=' || $2 ELSE '/orders/' || $2 END, $3
			FROM (
				SELECT o.user_id, FALSE AS company FROM orders o WHERE o.id=$4
				UNION
				SELECT ur.user_id, TRUE AS company FROM user_roles ur WHERE ur.role_slug IN ('admin','super_admin')
			) target
			ON CONFLICT (user_id, source_key) WHERE source_key IS NOT NULL DO NOTHING`, fmt.Sprintf("Order %s is now %s.", orderNumber, nextStage.Label()), orderNumber, sourceKey, orderID); err != nil {
			return err
		}
		if currentStatus != "shipped" && status == "shipped" {
			dispatchPayload, _ := json.Marshal(map[string]any{"seller_id": sellerID, "order_id": orderID, "order_number": orderNumber, "carrier": carrier, "tracking_number": trackingNumber})
			_, err = tx.Exec(transactionContext, `INSERT INTO outbox_events(event_type,aggregate_type,aggregate_id,payload) VALUES('SellerOrderShipped','SellerFulfillment',$1,$2)`, fmt.Sprintf("%d:%d", sellerID, orderID), dispatchPayload)
		}
		return err
	})
}
