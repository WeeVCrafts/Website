package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *RoleAdministrationRepository) UpdateRoleUser(ctx context.Context, actorID, userID int64, email, displayName, status, passwordHash, reason string) (bool, error) {
	if status != "active" && status != "suspended" {
		return false, ports.ErrRoleState
	}
	securityChanged := false
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := lockRoleAdministrator(ctx, tx, actorID); err != nil {
			return err
		}
		var oldEmail, oldStatus string
		if err := tx.QueryRow(ctx, `SELECT email,status FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&oldEmail, &oldStatus); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ports.ErrNotFound
			}
			return err
		}
		if oldStatus == "deleted" || (actorID == userID && status != "active") {
			return ports.ErrRoleState
		}
		if status != "active" {
			var protected bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_roles WHERE user_id=$1 AND role_slug='super_admin') AND NOT EXISTS(SELECT 1 FROM user_roles ur JOIN users u ON u.id=ur.user_id WHERE ur.role_slug='super_admin' AND u.status='active' AND u.id<>$1)`, userID).Scan(&protected); err != nil {
				return err
			}
			if protected {
				return ports.ErrRoleState
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET email_verified_at=CASE WHEN email<>$2 THEN NULL ELSE email_verified_at END,email=$2,display_name=$3,status=$4,updated_at=NOW() WHERE id=$1`, userID, email, displayName, status); err != nil {
			if isUniqueViolation(err) {
				return ports.ErrUserAlreadyExists
			}
			return err
		}
		if passwordHash != "" {
			result, err := tx.Exec(ctx, `UPDATE credentials SET password_hash=$2,password_changed_at=NOW(),failed_attempts=0,locked_until=NULL,updated_at=NOW() WHERE user_id=$1`, userID, passwordHash)
			if err != nil {
				return err
			}
			if result.RowsAffected() != 1 {
				return ports.ErrCredentialsNotFound
			}
		}
		securityChanged = oldEmail != email || oldStatus != status || passwordHash != ""
		if securityChanged {
			if _, err := tx.Exec(ctx, `UPDATE users SET sessions_revoked_at=NOW() WHERE id=$1`, userID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE sessions_metadata SET revoked_at=NOW() WHERE user_id=$1 AND revoked_at IS NULL`, userID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE auth_action_tokens SET consumed_at=NOW() WHERE user_id=$1 AND consumed_at IS NULL`, userID); err != nil {
				return err
			}
		}
		metadata, _ := json.Marshal(map[string]any{"reason": reason, "status": status, "password_changed": passwordHash != ""})
		_, err := tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'identity.user_updated','user',$2,$3,$4)`, fmt.Sprint(actorID), fmt.Sprint(userID), ports.RequestID(ctx), metadata)
		return err
	})
	if err != nil {
		return false, err
	}
	return securityChanged, nil
}

func lockRoleAdministrator(ctx context.Context, tx pgx.Tx, actorID int64) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(5165538220001)`); err != nil {
		return err
	}
	var allowed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.id=$1 AND u.status='active' AND ur.role_slug='super_admin')`, actorID).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return ports.ErrForbidden
	}
	return nil
}
