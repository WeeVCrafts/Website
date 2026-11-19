package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

// FulfillmentSetup reads only after the current database identity is authorized.
func (r *MarketplaceRepository) FulfillmentSetup(ctx context.Context, actor int64, q ports.MarketplaceSetupQuery) (ports.MarketplaceSetup, error) {
	d := ports.MarketplaceSetup{Query: q}
	a, err := marketplaceIdentity(ctx, r.pool, actor)
	if err != nil {
		return d, err
	}
	if a.role != "super_admin" {
		return d, ports.ErrForbidden
	}
	options := func(sql string, args ...any) ([]ports.MarketplaceSetupOption, int64, error) {
		rows, err := r.pool.Query(ctx, sql, args...)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close()
		var list []ports.MarketplaceSetupOption
		for rows.Next() {
			var o ports.MarketplaceSetupOption
			if err := rows.Scan(&o.ID, &o.SellerID, &o.Label); err != nil {
				return nil, 0, err
			}
			list = append(list, o)
		}
		if err := rows.Err(); err != nil {
			return nil, 0, err
		}
		var next int64
		if len(list) > 25 {
			next = list[24].ID
			list = list[:25]
		}
		return list, next, nil
	}
	d.Sellers, d.NextSeller, err = options(`SELECT id,id,display_name||' · Seller #'||id FROM sellers WHERE status='active' AND ($1='' OR strpos(lower(display_name),lower($1))>0 OR id::text=$1) AND ($2=0 OR id<$2) ORDER BY id DESC LIMIT 26`, strings.TrimSpace(q.SellerSearch), q.SellerBefore)
	if err != nil {
		return d, err
	}
	if q.SellerID > 0 {
		var selected ports.MarketplaceSetupOption
		err = r.pool.QueryRow(ctx, `SELECT id,id,display_name||' · Seller #'||id FROM sellers WHERE id=$1 AND status='active'`, q.SellerID).Scan(&selected.ID, &selected.SellerID, &selected.Label)
		if errors.Is(err, pgx.ErrNoRows) {
			return d, ports.ErrNotFound
		}
		if err != nil {
			return d, err
		}
		if err = r.pool.QueryRow(ctx, `SELECT display_name FROM sellers WHERE id=$1`, q.SellerID).Scan(&d.Seller); err != nil {
			return d, err
		}
		found := false
		for _, o := range d.Sellers {
			found = found || o.ID == selected.ID
		}
		if !found {
			d.Sellers = append([]ports.MarketplaceSetupOption{selected}, d.Sellers...)
		}
		err = r.pool.QueryRow(ctx, `SELECT seller_id,legal_name,address,state_code,gstin,registration,prefix,NOT checkout_review_hold FROM seller_invoice_profiles WHERE seller_id=$1`, q.SellerID).Scan(&d.Invoice.SellerID, &d.Invoice.LegalName, &d.Invoice.Address, &d.Invoice.StateCode, &d.Invoice.GSTIN, &d.Invoice.Registration, &d.Invoice.Prefix, &d.InvoiceReady)
		d.InvoiceSaved = err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return d, err
		}
		err = r.pool.QueryRow(ctx, `SELECT pickup_location,postal_code,verified FROM marketplace_shipping_profiles WHERE seller_id=$1`, q.SellerID).Scan(&d.PickupLocation, &d.PickupPIN, &d.PickupReady)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return d, err
		}
	}
	d.Variants, d.NextVariant, err = options(`SELECT v.id,p.seller_id,p.name||' · '||v.name||' · '||v.sku||' · Variant #'||v.id FROM product_variants v JOIN products p ON p.id=v.product_id WHERE p.seller_id=$1 AND v.status='active' AND ($2='' OR strpos(lower(p.name||' '||v.name||' '||v.sku),lower($2))>0 OR v.id::text=$2) AND ($3=0 OR v.id<$3) ORDER BY v.id DESC LIMIT 26`, q.SellerID, strings.TrimSpace(q.ProductSearch), q.VariantBefore)
	if err != nil {
		return d, err
	}
	if q.VariantID > 0 {
		var selected ports.MarketplaceSetupOption
		err = r.pool.QueryRow(ctx, `SELECT v.id,p.seller_id,p.name||' · '||v.name||' · '||v.sku||' · Variant #'||v.id,v.return_class FROM product_variants v JOIN products p ON p.id=v.product_id WHERE v.id=$1 AND p.seller_id=$2 AND v.status='active'`, q.VariantID, q.SellerID).Scan(&selected.ID, &selected.SellerID, &selected.Label, &d.ReturnClass)
		if errors.Is(err, pgx.ErrNoRows) {
			return d, ports.ErrNotFound
		}
		if err != nil {
			return d, err
		}
		if err = r.pool.QueryRow(ctx, `SELECT p.id,p.name,v.sku FROM product_variants v JOIN products p ON p.id=v.product_id WHERE v.id=$1`, q.VariantID).Scan(&d.ProductID, &d.Product, &d.SKU); err != nil {
			return d, err
		}
		var readiness ports.ShippingReadiness
		err = r.pool.QueryRow(ctx, `SELECT variant_id,seller_id,product,sku,seller,shipping_weight_g,shipping_length_cm,shipping_width_cm,shipping_height_cm,pickup_ready,invoice_ready,tax_ready,classified,stock,selected FROM marketplace_variant_readiness WHERE variant_id=$1`, q.VariantID).Scan(&readiness.VariantID, &readiness.SellerID, &readiness.Product, &readiness.SKU, &readiness.Seller, &readiness.WeightGrams, &readiness.LengthCM, &readiness.WidthCM, &readiness.HeightCM, &readiness.PickupReady, &readiness.InvoiceReady, &readiness.TaxReady, &readiness.Classified, &readiness.Stock, &readiness.Selected)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return d, err
		}
		if err == nil {
			readiness.ProductID = d.ProductID
			d.Selected = &readiness
		}
		found := false
		for _, o := range d.Variants {
			found = found || o.ID == selected.ID
		}
		if !found {
			d.Variants = append([]ports.MarketplaceSetupOption{selected}, d.Variants...)
		}
		err = r.pool.QueryRow(ctx, `SELECT hsn,rate_basis_points FROM variant_tax_profiles WHERE variant_id=$1`, q.VariantID).Scan(&d.HSN, &d.TaxBasisPoints)
		d.TaxSaved = err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return d, err
		}
	}
	d.Customers, d.NextCustomer, err = options(`SELECT u.id,0::bigint,u.display_name||' · Customer #'||u.id FROM users u WHERE u.status='active' AND EXISTS(SELECT 1 FROM user_roles ur WHERE ur.user_id=u.id AND ur.role_slug='customer') AND ($1='' OR strpos(lower(u.display_name),lower($1))>0 OR u.id::text=$1) AND ($2=0 OR u.id<$2) ORDER BY u.id DESC LIMIT 26`, strings.TrimSpace(q.CustomerSearch), q.CustomerBefore)
	if err != nil {
		return d, err
	}
	d.LaunchVariants, d.NextLaunch, err = options(`SELECT r.variant_id,r.seller_id,r.product||' · '||r.sku||' · '||r.seller FROM marketplace_variant_readiness r JOIN marketplace_ready_variants v USING(variant_id) WHERE v.stock>0 AND ($1='' OR strpos(lower(r.product||' '||r.sku||' '||r.seller),lower($1))>0) AND ($2=0 OR r.variant_id<$2) ORDER BY r.variant_id DESC LIMIT 26`, strings.TrimSpace(q.LaunchSearch), q.LaunchBefore)
	return d, err
}
