package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *MarketplaceRepository) SetupDefaults(ctx context.Context, actor, seller, variant int64, kind string) (d ports.SetupSubmissionData, profile *ports.SellerInvoiceProfile, err error) {
	a, err := marketplaceIdentity(ctx, r.pool, actor)
	if err != nil {
		return d, nil, err
	}
	if a.role != "super_admin" && (a.role != "seller" || a.seller != seller) {
		return d, nil, ports.ErrForbidden
	}
	if kind == "invoice" {
		p := &ports.SellerInvoiceProfile{SellerID: seller}
		err = r.pool.QueryRow(ctx, `SELECT legal_name,address,state_code,registration,gstin,prefix FROM seller_invoice_profiles WHERE seller_id=$1`, seller).Scan(&d.LegalName, &d.Address, &d.StateCode, &d.Registration, &d.GSTIN, &p.Prefix)
		if errors.Is(err, pgx.ErrNoRows) {
			return d, nil, nil
		}
		if err != nil {
			return d, nil, err
		}
		p.LegalName, p.Address, p.StateCode, p.Registration, p.GSTIN = d.LegalName, d.Address, d.StateCode, d.Registration, d.GSTIN
		return d, p, nil
	}
	var exists bool
	err = r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products p JOIN product_variants v ON v.product_id=p.id WHERE p.seller_id=$1 AND v.id=$2)`, seller, variant).Scan(&exists)
	if err != nil {
		return d, nil, err
	}
	if !exists {
		return d, nil, ports.ErrForbidden
	}
	err = r.pool.QueryRow(ctx, `SELECT COALESCE(t.hsn,''),COALESCE(t.rate_basis_points,0),v.return_class FROM product_variants v LEFT JOIN variant_tax_profiles t ON t.variant_id=v.id WHERE v.id=$1`, variant).Scan(&d.HSN, &d.RateBasisPoints, &d.ReturnClass)
	return d, nil, err
}

func (r *MarketplaceRepository) ActiveSetupSubmission(ctx context.Context, actor int64, kind string, variant int64) (s *ports.SetupSubmission, err error) {
	a, err := marketplaceIdentity(ctx, r.pool, actor)
	if err != nil {
		return nil, err
	}
	if a.role != "seller" || a.seller <= 0 {
		return nil, ports.ErrForbidden
	}
	var id int64
	err = r.pool.QueryRow(ctx, `SELECT id FROM marketplace_setup_submissions WHERE seller_id=$1 AND kind=$2 AND COALESCE(variant_id,0)=$3 AND status IN ('draft','submitted','changes_requested')`, a.seller, kind, variant).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	list, _, err := r.SetupSubmissions(ctx, actor, id, 0)
	if err != nil {
		return nil, err
	}
	if len(list) > 0 {
		s = &list[0]
	}
	return
}
