package postgres

import (
	"context"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *MarketplaceRepository) CommerceActor(ctx context.Context, actor int64) (a ports.CommerceActor, err error) {
	err = r.pool.QueryRow(ctx, `SELECT ur.role_slug,u.display_name,u.email,COALESCE(s.display_name,''),COALESCE(s.id,0) FROM users u JOIN user_roles ur ON ur.user_id=u.id LEFT JOIN sellers s ON s.owner_user_id=u.id AND s.status='active' WHERE u.id=$1 AND u.status='active'`, actor).Scan(&a.Role, &a.Name, &a.Email, &a.Seller, &a.SellerID)
	return
}

func (r *MarketplaceRepository) CommerceReads() *CommerceReadRepository {
	return NewCommerceReadRepository(r.pool)
}

func (r *MarketplaceRepository) SellerReadiness(ctx context.Context, actor, before int64, selected ...int64) (list []ports.SellerReadinessOption, next int64, err error) {
	a, err := marketplaceIdentity(ctx, r.pool, actor)
	if err != nil {
		return nil, 0, err
	}
	if a.role != "seller" || a.seller <= 0 {
		return nil, 0, ports.ErrForbidden
	}
	variant := int64(0)
	if len(selected) > 0 {
		variant = selected[0]
	}
	rows, err := r.pool.Query(ctx, `SELECT v.id,p.id,p.seller_id,p.name,v.sku,s.display_name,p.status,parcel.shipping_weight_g,parcel.shipping_length_cm,parcel.shipping_width_cm,parcel.shipping_height_cm,COALESCE(sp.verified,false),ip.seller_id IS NOT NULL AND NOT ip.checkout_review_hold,COALESCE(tp.variant_id IS NOT NULL AND NOT tp.checkout_review_hold AND (ip.registration<>'unregistered' OR tp.rate_basis_points=0),false),v.return_class<>'unclassified',COALESCE(i.available_quantity,0),lv.variant_id IS NOT NULL,COALESCE(review.id,0),COALESCE(review.status,'') FROM products p JOIN product_variants v ON v.product_id=p.id JOIN sellers s ON s.id=p.seller_id JOIN marketplace_variant_parcels parcel ON parcel.variant_id=v.id LEFT JOIN inventory_stock i ON i.variant_id=v.id LEFT JOIN seller_invoice_profiles ip ON ip.seller_id=s.id LEFT JOIN variant_tax_profiles tp ON tp.variant_id=v.id LEFT JOIN marketplace_shipping_profiles sp ON sp.seller_id=s.id LEFT JOIN marketplace_launch_variants lv ON lv.variant_id=v.id LEFT JOIN LATERAL(SELECT id,status FROM marketplace_setup_submissions WHERE seller_id=s.id AND (variant_id=v.id OR kind='invoice') AND status IN ('submitted','changes_requested') ORDER BY updated_at DESC LIMIT 1) review ON true WHERE p.seller_id=$1 AND p.status<>'archived' AND v.status='active' AND ($2=0 OR v.id<$2) AND ($3=0 OR v.id=$3) ORDER BY v.id DESC LIMIT 26`, a.seller, before, variant)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var x ports.SellerReadinessOption
		if err = rows.Scan(&x.VariantID, &x.ProductID, &x.SellerID, &x.Product, &x.SKU, &x.Seller, &x.ProductStatus, &x.WeightGrams, &x.LengthCM, &x.WidthCM, &x.HeightCM, &x.PickupReady, &x.InvoiceReady, &x.TaxReady, &x.Classified, &x.Stock, &x.Selected, &x.ReviewID, &x.ReviewStatus); err != nil {
			return nil, 0, err
		}
		list = append(list, x)
	}
	if len(list) > 25 {
		next = list[24].VariantID
		list = list[:25]
	}
	return list, next, rows.Err()
}

func (r *MarketplaceRepository) SellerFulfillmentSetup(ctx context.Context, actor int64) (ports.SellerFulfillmentSetup, error) {
	var setup ports.SellerFulfillmentSetup
	a, err := marketplaceIdentity(ctx, r.pool, actor)
	if err != nil {
		return setup, err
	}
	if a.role != "seller" || a.seller <= 0 {
		return setup, ports.ErrForbidden
	}
	err = r.pool.QueryRow(ctx, `SELECT i.seller_id IS NOT NULL AND NOT i.checkout_review_hold,
		COALESCE(pickup.verified,false),COALESCE(review.id,0),COALESCE(review.status,'')
		FROM sellers s LEFT JOIN seller_invoice_profiles i ON i.seller_id=s.id
		LEFT JOIN marketplace_shipping_profiles pickup ON pickup.seller_id=s.id
		LEFT JOIN LATERAL(SELECT id,status FROM marketplace_setup_submissions
			WHERE seller_id=s.id AND kind='invoice' AND status IN ('draft','submitted','changes_requested')
			ORDER BY id DESC LIMIT 1) review ON true
		WHERE s.id=$1 AND s.status='active'`, a.seller).
		Scan(&setup.InvoiceVerified, &setup.PickupVerified, &setup.InvoiceSubmissionID, &setup.InvoiceSubmissionStatus)
	return setup, err
}
