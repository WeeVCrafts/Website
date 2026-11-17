package postgres

import (
	"context"

	"github.com/wecratfs/commerce/internal/ports"
)

func (r *MarketplaceRepository) ShippingReadiness(ctx context.Context, actor, before int64) ([]ports.ShippingReadiness, int64, error) {
	a, err := marketplaceIdentity(ctx, r.pool, actor)
	if err != nil {
		return nil, 0, err
	}
	if a.role != "super_admin" {
		return nil, 0, ports.ErrForbidden
	}
	rows, err := r.pool.Query(ctx, `SELECT variant_id,(SELECT product_id FROM product_variants pv WHERE pv.id=variant_id),seller_id,product,sku,seller,shipping_weight_g,shipping_length_cm,shipping_width_cm,shipping_height_cm,pickup_ready,invoice_ready,tax_ready,classified,stock,selected
		FROM marketplace_variant_readiness WHERE ($1=0 OR variant_id<$1) ORDER BY variant_id DESC LIMIT 51`, before)
	if err != nil {
		return nil, 0, err
	}
	var list []ports.ShippingReadiness
	for rows.Next() {
		var item ports.ShippingReadiness
		if err = rows.Scan(&item.VariantID, &item.ProductID, &item.SellerID, &item.Product, &item.SKU, &item.Seller, &item.WeightGrams, &item.LengthCM, &item.WidthCM, &item.HeightCM, &item.PickupReady, &item.InvoiceReady, &item.TaxReady, &item.Classified, &item.Stock, &item.Selected); err != nil {
			break
		}
		list = append(list, item)
	}
	err = errorsJoinRows(err, rows)
	next := int64(0)
	if len(list) > 50 {
		next = list[49].VariantID
		list = list[:50]
	}
	return list, next, err
}
