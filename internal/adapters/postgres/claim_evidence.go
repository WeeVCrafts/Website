package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *MarketplaceRepository) AuthorizeClaimEvidence(ctx context.Context, actor, claim int64, write bool) error {
	var allowed bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_claims c JOIN orders o ON o.id=c.order_id JOIN users u ON u.id=$1 AND u.status='active' JOIN user_roles ur ON ur.user_id=u.id LEFT JOIN sellers s ON s.owner_user_id=u.id AND s.status='active' WHERE c.id=$2 AND ((ur.role_slug='customer' AND o.user_id=$1) OR ur.role_slug IN ('admin','super_admin') OR (NOT $3 AND ur.role_slug='seller' AND s.id=c.seller_id AND c.evidence_shared)) AND (NOT $3 OR c.status NOT IN ('completed','rejected')))`, actor, claim, write).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return ports.ErrForbidden
	}
	return nil
}
func (r *MarketplaceRepository) SaveClaimEvidence(ctx context.Context, actor, claim int64, key, name, kind string, size int64) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var locked int64
		if err := tx.QueryRow(ctx, `SELECT id FROM marketplace_claims WHERE id=$1 FOR UPDATE`, claim).Scan(&locked); err != nil {
			return err
		}
		var allowed bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_claims c JOIN orders o ON o.id=c.order_id JOIN users u ON u.id=$1 AND u.status='active' JOIN user_roles ur ON ur.user_id=u.id WHERE c.id=$2 AND c.status NOT IN ('completed','rejected') AND ((ur.role_slug='customer' AND o.user_id=$1) OR ur.role_slug IN ('admin','super_admin')) AND (SELECT count(*) FROM marketplace_claim_evidence WHERE claim_id=c.id)<10)`, actor, claim).Scan(&allowed); err != nil {
			return err
		}
		if !allowed {
			return ports.ErrForbidden
		}
		_, err := tx.Exec(ctx, `INSERT INTO marketplace_claim_evidence(claim_id,uploaded_by,object_key,filename,content_type,size_bytes) VALUES($1,$2,$3,$4,$5,$6)`, claim, actor, key, name, kind, size)
		return err
	})
}
func (r *MarketplaceRepository) ClaimEvidenceObject(ctx context.Context, actor, id int64) (key, name, kind string, err error) {
	var claim int64
	if err = r.pool.QueryRow(ctx, `SELECT claim_id FROM marketplace_claim_evidence WHERE id=$1`, id).Scan(&claim); err != nil {
		return
	}
	if err = r.AuthorizeClaimEvidence(ctx, actor, claim, false); err != nil {
		return
	}
	err = r.pool.QueryRow(ctx, `SELECT object_key,filename,content_type FROM marketplace_claim_evidence WHERE id=$1`, id).Scan(&key, &name, &kind)
	return
}
