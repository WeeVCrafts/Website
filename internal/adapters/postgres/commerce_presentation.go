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
	rows, err := r.pool.Query(ctx, `
		SELECT f.variant_id,f.product_id,f.seller_id,f.product,f.sku,f.seller,f.product_status,
			f.shipping_weight_g,f.shipping_length_cm,f.shipping_width_cm,f.shipping_height_cm,
			f.pickup_ready,f.invoice_ready,f.tax_ready,f.classified,f.stock,f.selected,
			COALESCE(review.id,0),COALESCE(review.status,''),
			COALESCE(f.image_url,''),COALESCE(f.has_custom_tax,false),COALESCE(f.return_class,'')
		FROM marketplace_option_setup_facts f
		LEFT JOIN LATERAL (
			SELECT id,status FROM marketplace_setup_submissions
			WHERE seller_id=f.seller_id AND (variant_id=f.variant_id OR kind='invoice')
				AND status IN ('submitted','changes_requested') ORDER BY updated_at DESC,id DESC LIMIT 1
		) review ON true
		WHERE f.seller_id=$1 AND ($2=0 OR f.variant_id<$2) AND ($3=0 OR f.variant_id=$3)
		ORDER BY f.variant_id DESC LIMIT 26`, a.seller, before, variant)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var x ports.SellerReadinessOption
		if err = rows.Scan(&x.VariantID, &x.ProductID, &x.SellerID, &x.Product, &x.SKU, &x.Seller, &x.ProductStatus, &x.WeightGrams, &x.LengthCM, &x.WidthCM, &x.HeightCM, &x.PickupReady, &x.InvoiceReady, &x.TaxReady, &x.Classified, &x.Stock, &x.Selected, &x.ReviewID, &x.ReviewStatus, &x.ImageURL, &x.HasCustomTax, &x.ReturnClass); err != nil {
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
		i.attested_by IS NOT NULL,
		COALESCE(pickup.verified,false),COALESCE(review.id,0),COALESCE(review.status,'')
		FROM sellers s LEFT JOIN seller_invoice_profiles i ON i.seller_id=s.id
		LEFT JOIN marketplace_shipping_profiles pickup ON pickup.seller_id=s.id
		LEFT JOIN LATERAL(SELECT id,status FROM marketplace_setup_submissions
			WHERE seller_id=s.id AND kind='invoice' AND status IN ('draft','submitted','changes_requested')
			ORDER BY id DESC LIMIT 1) review ON true
		WHERE s.id=$1 AND s.status='active'`, a.seller).
		Scan(&setup.InvoiceReady, &setup.InvoiceAttested, &setup.PickupVerified, &setup.InvoiceSubmissionID, &setup.InvoiceSubmissionStatus)
	return setup, err
}
