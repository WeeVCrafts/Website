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
	state, err := readSetupConfirmation(ctx, tx, seller, variant, kind)
	if err != nil {
		return err
	}
	if state.ReviewRequired {
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
