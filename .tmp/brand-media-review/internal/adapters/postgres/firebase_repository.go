package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	identity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
	"strings"
)

type FirebaseRepository struct{ pool *pgxpool.Pool }

func NewFirebaseRepository(pool *pgxpool.Pool) *FirebaseRepository { return &FirebaseRepository{pool} }

func (r *FirebaseRepository) FirebaseEmailRecipient(ctx context.Context, id int64, email string) (ports.FirebaseIdentity, bool, error) {
	var recipient ports.FirebaseIdentity
	err := r.pool.QueryRow(ctx, `SELECT f.firebase_uid,u.email,u.display_name FROM firebase_identities f JOIN users u ON u.id=f.user_id WHERE (u.id=$1 OR ($1=0 AND u.email=$2)) AND u.status='active' AND (SELECT count(*) FROM user_roles WHERE user_id=u.id)=1 AND EXISTS(SELECT 1 FROM user_roles WHERE user_id=u.id AND role_slug='customer')`, id, email).Scan(&recipient.UID, &recipient.Email, &recipient.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return recipient, false, nil
	}
	return recipient, err == nil, err
}

func (r *FirebaseRepository) FirebaseVerificationAllowed(ctx context.Context, email string) (bool, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM users WHERE email=$1) OR EXISTS(SELECT 1 FROM users u WHERE u.email=$1 AND u.status='active' AND (SELECT count(*) FROM user_roles WHERE user_id=u.id)=1 AND EXISTS(SELECT 1 FROM user_roles WHERE user_id=u.id AND role_slug='customer'))`, email).Scan(&allowed)
	return allowed, err
}
func (r *FirebaseRepository) FirebaseBinding(ctx context.Context, id int64) (string, string, error) {
	var uid, email string
	err := r.pool.QueryRow(ctx, `SELECT f.firebase_uid,u.email FROM firebase_identities f JOIN users u ON u.id=f.user_id WHERE u.id=$1`, id).Scan(&uid, &email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	return uid, email, err
}
func (r *FirebaseRepository) ResolveFirebase(ctx context.Context, proof ports.FirebaseIdentity, phone string, terms bool, linkID int64) (identity.User, error) {
	var u identity.User
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		// Serialize creation/linking of the same email without changing other users.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, proof.Email); err != nil {
			return err
		}
		var boundUID string
		err := tx.QueryRow(ctx, `SELECT u.id,u.email,u.display_name,u.status,u.created_at,COALESCE(f.firebase_uid,'') FROM users u LEFT JOIN firebase_identities f ON f.user_id=u.id WHERE u.email=$1 FOR UPDATE OF u`, proof.Email).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Status, &u.CreatedAt, &boundUID)
		if errors.Is(err, pgx.ErrNoRows) {
			if !terms {
				return applicationauth.ErrFirebaseTerms
			}
			name := strings.TrimSpace(proof.Name)
			if len([]rune(name)) > 120 {
				name = ""
			}
			err = tx.QueryRow(ctx, `INSERT INTO users(email,display_name,phone,status,email_verified_at) VALUES($1,$2,$3,'active',CASE WHEN $4 THEN NOW() ELSE NULL END) RETURNING id,email,display_name,status,created_at`, proof.Email, name, phone, proof.EmailVerified).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Status, &u.CreatedAt)
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'customer')`, u.ID); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO consent_records(user_id,purpose,policy_version,granted,source) VALUES($1,'necessary','privacy-2026-09',TRUE,'firebase-registration'),($1,'terms','privacy-2026-09',TRUE,'firebase-registration')`, u.ID); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		var roles []string
		if err = tx.QueryRow(ctx, `SELECT COALESCE(array_agg(role_slug),ARRAY[]::text[]) FROM user_roles WHERE user_id=$1`, u.ID).Scan(&roles); err != nil {
			return err
		}
		if u.Status != "active" || len(roles) != 1 || roles[0] != "customer" {
			return applicationauth.ErrFirebaseRole
		}
		u.Roles = []identity.Role{identity.RoleCustomer}
		u.EmailVerified = proof.EmailVerified
		if boundUID != "" && boundUID != proof.UID {
			return applicationauth.ErrFirebaseInvalid
		}
		if boundUID == "" {
			var hasCredentials bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM credentials WHERE user_id=$1)`, u.ID).Scan(&hasCredentials); err != nil {
				return err
			}
			if hasCredentials && (linkID != u.ID || !proof.EmailVerified) {
				return applicationauth.ErrFirebaseLinkRequired
			}
			if _, err = tx.Exec(ctx, `INSERT INTO firebase_identities(user_id,firebase_uid) VALUES($1,$2)`, u.ID, proof.UID); err != nil {
				return err
			}
			// Once explicitly linked, Firebase owns password recovery. Native passwords
			// must stop working, including previously issued native recovery links.
			if _, err = tx.Exec(ctx, `DELETE FROM credentials WHERE user_id=$1`, u.ID); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE auth_action_tokens SET consumed_at=NOW() WHERE user_id=$1 AND consumed_at IS NULL`, u.ID); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,metadata) VALUES($1,'auth.firebase_connected','user',$1,'{}'::jsonb)`, fmt.Sprint(u.ID)); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE users SET sessions_revoked_at=NOW() WHERE id=$1`, u.ID); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE users SET email_verified_at=CASE WHEN $2 THEN COALESCE(email_verified_at,NOW()) ELSE email_verified_at END WHERE id=$1`, u.ID, proof.EmailVerified)
		return err
	})
	return u, err
}
