package postgres

import (
	"context"

	"github.com/wecratfs/commerce/internal/ports"
)

func (r *MarketplaceRepository) fulfillmentQueue(ctx context.Context) ([]ports.FulfillmentQueueItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT o.order_number,s.display_name,issue.label
		FROM seller_fulfillments sf JOIN orders o ON o.id=sf.order_id JOIN sellers s ON s.id=sf.seller_id
		CROSS JOIN LATERAL (
			VALUES
			('Acceptance overdue', sf.lifecycle_status='new' AND sf.created_at<=now()-interval '24 hours'
				AND o.status='paid'),
			('Preparing past dispatch target', sf.lifecycle_status IN ('accepted','packing')
				AND sf.accepted_at<=now()-interval '48 hours' AND NOT o.payment_review_required),
			('Seller cancellation', EXISTS (
				SELECT 1 FROM marketplace_cancellations c JOIN order_items i ON i.id=c.order_item_id
				WHERE c.order_id=o.id AND i.seller_id=s.id AND c.requested_by=s.owner_user_id
					AND c.status IN ('requested','completed') AND c.created_at>=now()-interval '7 days'))
		) issue(label,shown)
		WHERE issue.shown ORDER BY sf.created_at, sf.order_id, sf.seller_id, issue.label LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []ports.FulfillmentQueueItem
	for rows.Next() {
		var item ports.FulfillmentQueueItem
		if err := rows.Scan(&item.Number, &item.Seller, &item.Issue); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}
