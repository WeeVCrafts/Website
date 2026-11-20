package postgres

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *MarketplaceRepository) SellerSetupConfirmation(ctx context.Context, actor, variant int64, kind string) (ports.SetupConfirmation, error) {
	a, err := marketplaceIdentity(ctx, r.pool, actor)
	if err != nil {
		return ports.SetupConfirmation{}, err
	}
	if a.role != "seller" || a.seller <= 0 {
		return ports.SetupConfirmation{}, ports.ErrForbidden
	}
	return readSetupConfirmation(ctx, r.pool, a.seller, variant, kind)
}

func readSetupConfirmation(ctx context.Context, q catalogProductQuerier, seller, variant int64, kind string) (state ports.SetupConfirmation, err error) {
	if kind != "invoice" && kind != "tax" {
		return state, ports.ErrInvalidState
	}
	// An old changes-requested submission may have been superseded by a
	// draft before the seller had a profile on which to store a hold.
	err = q.QueryRow(ctx, `
		SELECT CASE WHEN $2='invoice' THEN COALESCE(i.revision,0) ELSE COALESCE(t.revision,0) END,
			COALESCE(CASE WHEN NOT i.checkout_review_hold THEN i.registration END,''),
			CASE WHEN $2='invoice' THEN COALESCE(i.checkout_review_hold,false) ELSE COALESCE(t.checkout_review_hold,false) END
				OR COALESCE(review.unresolved,false)
		FROM sellers s
		LEFT JOIN seller_invoice_profiles i ON i.seller_id=s.id
		LEFT JOIN variant_tax_profiles t ON t.variant_id=$3
		LEFT JOIN LATERAL (
			SELECT MAX(m.id) FILTER(WHERE m.status IN ('submitted','changes_requested','superseded'))
				> COALESCE(MAX(m.id) FILTER(WHERE m.status='approved'),0) AS unresolved
			FROM marketplace_setup_submissions m
			WHERE m.seller_id=s.id AND m.kind=$2 AND COALESCE(m.variant_id,0)=$3
		) review ON true
		WHERE s.id=$1 AND s.status='active'
		AND ($2='invoice' AND $3=0 OR $2='tax' AND EXISTS(
			SELECT 1 FROM product_variants v JOIN products p ON p.id=v.product_id
			WHERE v.id=$3 AND p.seller_id=s.id AND v.status='active' AND p.status<>'archived'
		))`, seller, kind, variant).Scan(&state.ProfileRevision, &state.Registration, &state.ReviewRequired)
	if errors.Is(err, pgx.ErrNoRows) {
		return state, ports.ErrForbidden
	}
	return state, err
}

// Save the same checkout profile for either route and record the provenance
// without changing the platform's invoice-number sequence.
func applySetupProfile(ctx context.Context, tx pgx.Tx, seller, variant, actor int64, kind string, data ports.SetupSubmissionData, attest bool) error {
	reviewer, attester := actor, int64(0)
	if attest {
		reviewer, attester = 0, actor
	}
	if kind == "invoice" {
		prefix := "S" + strings.ToUpper(strconv.FormatInt(seller, 36))
		if len(prefix) > 6 {
			return ports.ErrInvalidState
		}
		_, err := tx.Exec(ctx, `INSERT INTO seller_invoice_profiles
			(seller_id,legal_name,address,state_code,gstin,registration,prefix,verified_by,verified_at,attested_by,attested_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8::bigint,0),CASE WHEN $8<>0 THEN now() END,NULLIF($9::bigint,0),CASE WHEN $9<>0 THEN now() END)
			ON CONFLICT(seller_id) DO UPDATE SET legal_name=EXCLUDED.legal_name,address=EXCLUDED.address,
			state_code=EXCLUDED.state_code,gstin=EXCLUDED.gstin,registration=EXCLUDED.registration,
			verified_by=EXCLUDED.verified_by,verified_at=EXCLUDED.verified_at,
			attested_by=EXCLUDED.attested_by,attested_at=EXCLUDED.attested_at,
			revision=seller_invoice_profiles.revision+1,checkout_review_hold=false`,
			seller, data.LegalName, data.Address, data.StateCode, data.GSTIN, data.Registration, prefix, reviewer, attester)
		return err
	}
	var registration string
	if err := tx.QueryRow(ctx, `SELECT registration FROM seller_invoice_profiles
		WHERE seller_id=$1 AND NOT checkout_review_hold FOR SHARE`, seller).Scan(&registration); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrSetupBillingRequired
		}
		return err
	}
	if registration == "unregistered" && data.RateBasisPoints != 0 {
		return ports.ErrSetupTaxRegistration
	}
	var owned int64
	if err := tx.QueryRow(ctx, `SELECT v.id FROM product_variants v JOIN products p ON p.id=v.product_id
		WHERE v.id=$1 AND p.seller_id=$2 AND v.status='active' AND p.status<>'archived' FOR UPDATE OF v`, variant, seller).Scan(&owned); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrInvalidState
		}
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO variant_tax_profiles
		(variant_id,hsn,rate_basis_points,verified_by,verified_at,attested_by,attested_at)
		VALUES ($1,$2,$3,NULLIF($4::bigint,0),CASE WHEN $4<>0 THEN now() END,NULLIF($5::bigint,0),CASE WHEN $5<>0 THEN now() END)
		ON CONFLICT(variant_id) DO UPDATE SET hsn=EXCLUDED.hsn,rate_basis_points=EXCLUDED.rate_basis_points,
		verified_by=EXCLUDED.verified_by,verified_at=EXCLUDED.verified_at,
		attested_by=EXCLUDED.attested_by,attested_at=EXCLUDED.attested_at,
		revision=variant_tax_profiles.revision+1,checkout_review_hold=false`,
		variant, data.HSN, data.RateBasisPoints, reviewer, attester); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE product_variants SET return_class=$2,return_class_request=$2,updated_at=now() WHERE id=$1`, variant, data.ReturnClass)
	return err
}
