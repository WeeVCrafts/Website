package postgres

import (
	"context"

	"github.com/wecratfs/commerce/internal/ports"
)

func (r *MarketplaceRepository) ShippingReadiness(ctx context.Context, actor, before int64) ([]ports.ShippingReadiness, int64, error) {
	return r.ShippingReadinessFiltered(ctx, actor, before, 0, "")
}

func (r *MarketplaceRepository) ShippingReadinessFiltered(ctx context.Context, actor, before, seller int64, search string) ([]ports.ShippingReadiness, int64, error) {
	a, err := marketplaceIdentity(ctx, r.pool, actor)
	if err != nil {
		return nil, 0, err
	}
	if a.role != "super_admin" {
		return nil, 0, ports.ErrForbidden
	}
	rows, err := r.pool.Query(ctx, `SELECT variant_id,(SELECT product_id FROM product_variants pv WHERE pv.id=variant_id),seller_id,product,sku,seller,shipping_weight_g,shipping_length_cm,shipping_width_cm,shipping_height_cm,pickup_ready,invoice_ready,tax_ready,classified,stock,selected,fulfillment_ready AND live_allowed AND stock>0,live_blocker
		FROM marketplace_checkout_options WHERE ($1=0 OR variant_id<$1) AND ($2=0 OR seller_id=$2) AND ($3='' OR strpos(lower(product||' '||sku),lower($3))>0) ORDER BY variant_id DESC LIMIT 51`, before, seller, search)
	if err != nil {
		return nil, 0, err
	}
	var list []ports.ShippingReadiness
	for rows.Next() {
		var item ports.ShippingReadiness
		if err = rows.Scan(&item.VariantID, &item.ProductID, &item.SellerID, &item.Product, &item.SKU, &item.Seller, &item.WeightGrams, &item.LengthCM, &item.WidthCM, &item.HeightCM, &item.PickupReady, &item.InvoiceReady, &item.TaxReady, &item.Classified, &item.Stock, &item.Selected, &item.LiveCheckout, &item.LiveBlocker); err != nil {
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
