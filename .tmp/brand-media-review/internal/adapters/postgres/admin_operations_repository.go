package postgres

import (
	"context"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *CommerceRepository) ListAdminOrders(ctx context.Context, actorID int64, limit int) ([]domaincommerce.AdminOrder, error) {
	var allowed bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM user_roles WHERE user_id = $1 AND role_slug IN ('admin', 'super_admin'))`, actorID).Scan(&allowed); err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ports.ErrForbidden
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := r.pool.Query(ctx, `
		SELECT o.id, o.order_number, o.status, u.display_name,
		       CASE WHEN POSITION('@' IN u.email) > 1
		            THEN LEFT(u.email, 1) || '***@' || SPLIT_PART(u.email, '@', 2)
		            ELSE '***' END,
		       COALESCE(pa.status, 'unrecorded'), COALESCE((SELECT CASE
				WHEN bool_and(COALESCE(sf2.status,'pending')='delivered') THEN 'delivered'
				WHEN bool_and(COALESCE(sf2.status,'pending') IN ('out_for_delivery','delivered')) THEN 'out_for_delivery'
				WHEN bool_and(COALESCE(sf2.status,'pending') IN ('shipped','out_for_delivery','delivered')) THEN 'shipped'
				WHEN bool_or(COALESCE(sf2.status,'pending')='packing') THEN 'packing'
				WHEN bool_or(COALESCE(sf2.status,'pending')='processing') THEN 'processing'
				ELSE 'pending' END
				FROM order_items oi2 JOIN product_variants pv2 ON pv2.id=oi2.variant_id JOIN products p2 ON p2.id=pv2.product_id LEFT JOIN seller_fulfillments sf2 ON sf2.order_id=oi2.order_id AND sf2.seller_id=p2.seller_id WHERE oi2.order_id=o.id), 'pending'),
		       o.total_cents, o.currency, o.created_at
		FROM orders o
		JOIN users u ON u.id = o.user_id
		LEFT JOIN LATERAL (
			SELECT status FROM payment_attempts WHERE order_id = o.id ORDER BY id DESC LIMIT 1
		) pa ON TRUE
		ORDER BY o.created_at DESC, o.id DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := make([]domaincommerce.AdminOrder, 0)
	for rows.Next() {
		var order domaincommerce.AdminOrder
		if err := rows.Scan(&order.ID, &order.OrderNumber, &order.Status, &order.CustomerLabel, &order.CustomerEmailMasked, &order.PaymentStatus, &order.FulfillmentStatus, &order.TotalCents, &order.Currency, &order.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

var _ ports.AdminOperationsRepository = (*CommerceRepository)(nil)
