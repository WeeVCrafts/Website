// Explicit operator tool for syncing designated identities without a data reset.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	"github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/config"
	"github.com/wecratfs/commerce/internal/domain/identity"
)

type account struct {
	Username      string        `json:"username"`
	Email         string        `json:"email"`
	ExistingEmail string        `json:"existing_email"`
	Role          identity.Role `json:"role"`
	Password      string        `json:"password"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if os.Getenv("WECRATFS_SYNC_DESIGNATED_ACCOUNTS") != "true" {
		return errors.New("explicit WECRATFS_SYNC_DESIGNATED_ACCOUNTS=true required")
	}
	var accounts []account
	decoder := json.NewDecoder(io.LimitReader(os.Stdin, 16*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&accounts); err != nil {
		return errors.New("invalid account input")
	}
	if err := validate(accounts); err != nil {
		return err
	}
	cfg := config.FromEnv()
	poolConfig, err := cfg.DatabasePoolConfig()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return errors.New("cannot create database connection")
	}
	defer pool.Close()
	hashes := make([]string, len(accounts))
	for i, item := range accounts {
		hashes[i], err = auth.NewArgon2idHasher().Hash(item.Password)
		if err != nil {
			return errors.New("cannot hash account password")
		}
	}
	if err := postgres.WithinTransaction(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		// Share the role-management lock. This tool requires existing roles to
		// match and cannot bypass seller exit or last-super-admin protections.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(5165538220001)`); err != nil {
			return err
		}
		for i, item := range accounts {
			var id int64
			err := tx.QueryRow(ctx, `SELECT id FROM users WHERE email=$1 FOR UPDATE`, item.Email).Scan(&id)
			if errors.Is(err, pgx.ErrNoRows) && item.ExistingEmail != "" {
				err = tx.QueryRow(ctx, `SELECT id FROM users WHERE email=$1 FOR UPDATE`, item.ExistingEmail).Scan(&id)
			}
			if errors.Is(err, pgx.ErrNoRows) {
				if item.ExistingEmail != "" || item.Role == identity.RoleSeller {
					return errors.New("expected account or seller business profile missing")
				}
				if err := tx.QueryRow(ctx, `INSERT INTO users(email,display_name,status,email_verified_at) VALUES($1,$2,'active',NOW()) RETURNING id`, item.Email, item.Username).Scan(&id); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,$2)`, id, string(item.Role)); err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
			var allowed bool
			if err := tx.QueryRow(ctx, `SELECT (SELECT COUNT(*) FROM user_roles WHERE user_id=$1)=1 AND EXISTS(SELECT 1 FROM user_roles WHERE user_id=$1 AND role_slug=$2) AND EXISTS(SELECT 1 FROM users WHERE id=$1 AND status='active')`, id, string(item.Role)).Scan(&allowed); err != nil {
				return err
			}
			if !allowed {
				return fmt.Errorf("%s must be active with the requested role before credential sync", item.Username)
			}
			if _, err := tx.Exec(ctx, `UPDATE users SET email=$2,username=$3,display_name=$3,email_verified_at=NOW(),sessions_revoked_at=NOW(),updated_at=NOW() WHERE id=$1`, id, item.Email, item.Username); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO credentials(user_id,password_hash) VALUES($1,$2) ON CONFLICT(user_id) DO UPDATE SET password_hash=EXCLUDED.password_hash,password_changed_at=NOW(),failed_attempts=0,locked_until=NULL,updated_at=NOW()`, id, hashes[i]); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE sessions_metadata SET revoked_at=NOW() WHERE user_id=$1 AND revoked_at IS NULL`, id); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO audit_logs(action,resource_type,resource_id,metadata) VALUES('identity.designated_account_synced','user',$1,jsonb_build_object('reason','Owner requested designated credentials for local and Railway','username',$2::text,'role',$3::text))`, fmt.Sprint(id), item.Username, string(item.Role)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		// Database errors may contain input or hashes; never print their details.
		return errors.New("account sync rolled back; check identity uniqueness, active roles and database schema")
	}
	fmt.Println("Four designated accounts synced; prior sessions revoked; business records preserved.")
	return nil
}

func validate(accounts []account) error {
	if len(accounts) != 4 {
		return errors.New("exactly four designated accounts required")
	}
	roles, names, emails := map[identity.Role]bool{}, map[string]bool{}, map[string]bool{}
	pattern := regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)
	for _, item := range accounts {
		// Owner-approved bootstrap credentials are independent of the public
		// 12-character registration/recovery policy, which remains unchanged.
		if !identity.ValidRole(item.Role) || roles[item.Role] || !pattern.MatchString(item.Username) || names[item.Username] || emails[item.Email] || !strings.Contains(item.Email, "@") || item.Email != strings.ToLower(strings.TrimSpace(item.Email)) || len(item.Password) < 10 || len(item.Password) > 1024 {
			return errors.New("invalid or duplicate designated identity")
		}
		roles[item.Role], names[item.Username], emails[item.Email] = true, true, true
	}
	return nil
}
