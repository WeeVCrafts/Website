package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *UserRepository) GetCredentialsByID(ctx context.Context, id int64) (domainidentity.Credentials, error) {
	var email string
	if err := r.pool.QueryRow(ctx, `SELECT email FROM users WHERE id=$1`, id).Scan(&email); err != nil {
		return domainidentity.Credentials{}, err
	}
	return r.GetCredentialsByEmail(ctx, email)
}

func (r *UserRepository) ChangePassword(ctx context.Context, id int64, expected, next string) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var active bool
		if err := tx.QueryRow(ctx, `SELECT status='active' FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&active); err != nil {
			return err
		}
		if !active {
			return ports.ErrForbidden
		}
		updated, err := tx.Exec(ctx, `UPDATE credentials SET password_hash=$3,password_changed_at=NOW(),failed_attempts=0,locked_until=NULL,updated_at=NOW() WHERE user_id=$1 AND password_hash=$2`, id, expected, next)
		if err != nil {
			return err
		}
		if updated.RowsAffected() != 1 {
			return ports.ErrCredentialsNotFound
		}
		if _, err = tx.Exec(ctx, `UPDATE users SET sessions_revoked_at=NOW(),updated_at=NOW() WHERE id=$1`, id); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE auth_action_tokens SET consumed_at=NOW() WHERE user_id=$1 AND purpose='password_reset' AND consumed_at IS NULL`, id); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'auth.password_changed','user',$1,$2,'{}'::jsonb)`, fmt.Sprint(id), ports.RequestID(ctx))
		return err
	})
}
