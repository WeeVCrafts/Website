package postgres

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

func guardSetupReview(ctx context.Context, tx pgx.Tx, actor, seller, variant int64, kind string) error {
	a, err := marketplaceIdentity(ctx, tx, actor)
	if err != nil {
		return err
	}
	if a.role != "super_admin" {
		return ports.ErrForbidden
	}
	var locked int64
	if err := tx.QueryRow(ctx, `SELECT id FROM sellers WHERE id=$1 AND status='active' FOR UPDATE`, seller).Scan(&locked); err != nil {
		return err
	}
	var pending bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_setup_submissions WHERE seller_id=$1 AND kind=$2 AND COALESCE(variant_id,0)=$3 AND status IN ('submitted','changes_requested')) OR ($2='invoice' AND EXISTS(SELECT 1 FROM seller_invoice_profiles WHERE seller_id=$1 AND checkout_review_hold)) OR ($2='tax' AND EXISTS(SELECT 1 FROM variant_tax_profiles WHERE variant_id=$3 AND checkout_review_hold))`, seller, kind, variant).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return ports.ErrFulfillmentConflict
	}
	return nil
}

func recordOperatorSetup(ctx context.Context, tx pgx.Tx, actor, seller, variant int64, kind string, data ports.SetupSubmissionData, note string) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO marketplace_setup_submissions(seller_id,variant_id,kind,status,payload,review_note,created_by,reviewed_by,submitted_at,reviewed_at) VALUES($1,NULLIF($2,0),$3,'approved',$4,$5,$6,$6,now(),now())`, seller, variant, kind, payload, note, actor)
	return err
}
