package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *MarketplaceRepository) ReturnShippingOrder(ctx context.Context, id int64) (ports.ReturnShippingOrder, error) {
	var result ports.ReturnShippingOrder
	result.ClaimID = id
	err := r.pool.QueryRow(ctx, `SELECT c.order_id,c.seller_id,o.order_number,COALESCE(c.return_reference,'WC-RET-'||c.id),sp.pickup_location,c.weight_grams,c.length_cm,c.width_cm,c.height_cm,COALESCE(c.return_order_id,0),COALESCE(c.return_shipment_id,0),c.return_awb,c.return_courier,c.return_status,c.pending_action,o.address_snapshot,u.email,c.submitted_at FROM marketplace_claims c JOIN orders o ON o.id=c.order_id JOIN users u ON u.id=o.user_id JOIN marketplace_shipping_profiles sp ON sp.seller_id=c.seller_id AND sp.verified WHERE c.id=$1 AND c.status IN ('approved','pickup_pending')`, id).Scan(&result.OrderID, &result.SellerID, &result.OrderNumber, &result.Reference, &result.PickupLocation, &result.WeightGrams, &result.LengthCM, &result.WidthCM, &result.HeightCM, &result.ProviderOrderID, &result.ProviderShipmentID, &result.AWB, &result.Courier, &result.ProviderStatus, &result.PendingAction, &result.Address, &result.Email, &result.PlacedAt)
	if err != nil {
		return result, err
	}
	var item domain.SellerOrderItem
	err = r.pool.QueryRow(ctx, `SELECT oi.product_name,oi.sku,c.quantity,oi.unit_price_cents,oi.unit_price_cents*c.quantity,COALESCE(oi.allocated_discount_cents,0)*c.quantity/oi.quantity FROM marketplace_claims c JOIN order_items oi ON oi.id=c.order_item_id WHERE c.id=$1`, id).Scan(&item.ProductName, &item.SKU, &item.Quantity, &item.UnitPriceCents, &item.LineTotalCents, &result.DiscountCents)
	result.Items = []domain.SellerOrderItem{item}
	return result, err
}
func (r *MarketplaceRepository) StartReturnShipping(ctx context.Context, id int64) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var allowed bool
		if err := tx.QueryRow(ctx, `SELECT marketplace_order_funded(c.order_id) AND s.shipping_mode<>'off' AND c.status='approved' AND c.pending_action='' AND c.return_order_id IS NULL AND c.weight_grams>=100 AND c.length_cm>0 AND c.width_cm>0 AND c.height_cm>0 FROM marketplace_claims c CROSS JOIN marketplace_policy_settings s WHERE c.id=$1 AND s.id FOR UPDATE OF c`, id).Scan(&allowed); err != nil {
			return err
		}
		if !allowed {
			return ports.ErrShippingPending
		}
		_, err := tx.Exec(ctx, `UPDATE marketplace_claims SET pending_action='create',return_reference='WC-RET-'||id,updated_at=now() WHERE id=$1`, id)
		return err
	})
}
func (r *MarketplaceRepository) SaveReturnShipping(ctx context.Context, id int64, s ports.Shipment) error {
	if s.ProviderOrderID <= 0 || s.ProviderShipmentID <= 0 {
		return ports.ErrShippingPending
	}
	result, err := r.pool.Exec(ctx, `UPDATE marketplace_claims SET return_order_id=$2,return_shipment_id=$3,return_awb=COALESCE(NULLIF($4,''),return_awb),return_courier=COALESCE(NULLIF($5,''),return_courier),return_status=CASE WHEN return_status IN ('DELIVERED','RETURN DELIVERED','RTO DELIVERED') THEN return_status ELSE $6 END,pending_action='' ,status=CASE WHEN $4<>'' THEN 'pickup_pending' ELSE status END,updated_at=now(),next_check_at=now()+interval '5 minutes' WHERE id=$1 AND return_reference=$7 AND (return_order_id IS NULL OR return_order_id=$2) AND (return_shipment_id IS NULL OR return_shipment_id=$3) AND status IN ('approved','pickup_pending')`, id, s.ProviderOrderID, s.ProviderShipmentID, s.AWB, s.Courier, s.ProviderStatus, s.Reference)
	if err != nil {
		return err
	}
	if s.ProviderOrderID <= 0 || s.ProviderShipmentID <= 0 || result.RowsAffected() != 1 {
		return ports.ErrShippingPending
	}
	return nil
}
// Clear intent only after a definitive rejection or a failure before submission.
// Timeouts and ambiguous provider replies must keep the intent for reconciliation.
func (r *MarketplaceRepository) RejectReturnShipping(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE marketplace_claims SET pending_action='',updated_at=now() WHERE id=$1 AND pending_action='create' AND return_order_id IS NULL`, id)
	return err
}
func (r *MarketplaceRepository) QueuePolicyMaintenance(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `WITH due AS (SELECT id FROM marketplace_claims WHERE status IN ('approved','pickup_pending') AND return_reference IS NOT NULL AND next_check_at<=now() ORDER BY next_check_at,id LIMIT 20 FOR UPDATE SKIP LOCKED), scheduled AS (UPDATE marketplace_claims c SET next_check_at=now()+interval '5 minutes' FROM due WHERE c.id=due.id RETURNING c.id) SELECT marketplace_enqueue('return_pickup',id,'return-track:'||id||':'||to_char(now(),'YYYYMMDDHH24MI')) FROM scheduled`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO notifications(user_id,kind,title,body,link_url,source_key) SELECT ur.user_id,'order','Claim review overdue','A claim needs review: '||c.id,'/admin/fulfillment?order='||o.order_number,'claim-overdue:'||c.id FROM marketplace_claims c JOIN orders o ON o.id=c.order_id CROSS JOIN user_roles ur WHERE ur.role_slug IN ('admin','super_admin') AND c.status='requested' AND c.review_due_at<=now() ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO notifications(user_id,kind,title,body,link_url,source_key) SELECT ur.user_id,'order','Return resolution needs attention','Check reverse pickup or inspected resolution for claim '||c.id,'/admin/fulfillment?order='||o.order_number,'claim-operation-overdue:'||c.id||':'||c.status FROM marketplace_claims c JOIN orders o ON o.id=c.order_id CROSS JOIN user_roles ur WHERE ur.role_slug IN ('admin','super_admin') AND ((c.status='approved' AND c.pickup_due_at<=now()) OR (c.status='inspected' AND c.resolution_due_at<=now())) ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING`)
	return err
}
