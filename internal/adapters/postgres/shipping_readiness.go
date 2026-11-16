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
	rows, err := r.pool.Query(ctx, `SELECT v.id,p.seller_id,p.name,v.sku,s.display_name,v.shipping_weight_g,v.shipping_length_cm,v.shipping_width_cm,v.shipping_height_cm,COALESCE(sp.verified,false),ip.seller_id IS NOT NULL,COALESCE(tp.variant_id IS NOT NULL AND (ip.registration<>'unregistered' OR tp.rate_basis_points=0),false),v.return_class<>'unclassified' FROM product_variants v JOIN products p ON p.id=v.product_id JOIN sellers s ON s.id=p.seller_id LEFT JOIN marketplace_shipping_profiles sp ON sp.seller_id=s.id LEFT JOIN seller_invoice_profiles ip ON ip.seller_id=s.id LEFT JOIN variant_tax_profiles tp ON tp.variant_id=v.id WHERE v.status='active' AND p.status='approved' AND s.status='active' AND ($1=0 OR v.id<$1) ORDER BY v.id DESC LIMIT 51`, before)
	if err != nil {
		return nil, 0, err
	}
	var list []ports.ShippingReadiness
	for rows.Next() {
		var item ports.ShippingReadiness
		if err = rows.Scan(&item.VariantID, &item.SellerID, &item.Product, &item.SKU, &item.Seller, &item.WeightGrams, &item.LengthCM, &item.WidthCM, &item.HeightCM, &item.PickupReady, &item.InvoiceReady, &item.TaxReady, &item.Classified); err != nil {
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
