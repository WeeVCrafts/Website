package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wecratfs/commerce/internal/adapters/postgres"
	"github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/config"
)

type account struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Password string `json:"password,omitempty"`
	Created  bool   `json:"created"`
}

var roleAccounts = []account{
	{Email: "customer@weevcrafts.test", Name: "QA Customer", Role: "customer"},
	{Email: "seller@weevcrafts.test", Name: "QA Seller", Role: "seller"},
	{Email: "admin@weevcrafts.test", Name: "QA Admin", Role: "admin"},
	{Email: "superadmin@weevcrafts.test", Name: "QA Super Admin", Role: "super_admin"},
}

func main() {
	if os.Getenv("WECRATFS_PROVISION_ROLE_ACCOUNTS") != "true" {
		log.Fatal("refusing to provision: set WECRATFS_PROVISION_ROLE_ACCOUNTS=true explicitly")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cfg := config.FromEnv()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatal(err)
	}
	if err := postgres.ApplyMigrations(ctx, pool); err != nil {
		log.Fatal(err)
	}

	accounts := make([]account, len(roleAccounts))
	copy(accounts, roleAccounts)
	if err := postgres.WithinTransaction(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		ids := make(map[string]int64, len(accounts))
		for i, item := range accounts {
			var id int64
			err := tx.QueryRow(ctx, `
				INSERT INTO users (email, display_name, status, email_verified_at)
				VALUES ($1, $2, 'active', NOW())
				ON CONFLICT (email) DO NOTHING
				RETURNING id`, item.Email, item.Name).Scan(&id)
			created := err == nil
			if errors.Is(err, pgx.ErrNoRows) {
				if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE email=$1`, item.Email).Scan(&id); err != nil {
					return fmt.Errorf("load %s: %w", item.Email, err)
				}
			} else if err != nil {
				return fmt.Errorf("create %s: %w", item.Email, err)
			}
			if created {
				password, err := newPassword()
				if err != nil {
					return err
				}
				hash, err := auth.NewArgon2idHasher().Hash(password)
				if err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `INSERT INTO credentials (user_id, password_hash) VALUES ($1, $2)`, id, hash); err != nil {
					return fmt.Errorf("create credentials for %s: %w", item.Email, err)
				}
				accounts[i].Password = password
				accounts[i].Created = true
			}
			if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_slug) VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, item.Role); err != nil {
				return fmt.Errorf("assign role %s to %s: %w", item.Role, item.Email, err)
			}
			ids[item.Role] = id
			username := strings.SplitN(item.Email, "@", 2)[0]
			if _, err := tx.Exec(ctx, `UPDATE users SET username=$2 WHERE id=$1`, id, username); err != nil {
				return fmt.Errorf("set QA username: %w", err)
			}
		}

		var sellerID int64
		err := tx.QueryRow(ctx, `
			INSERT INTO sellers (owner_user_id, display_name, status)
			VALUES ($1, 'Weevcrafts QA Seller', 'active')
			ON CONFLICT (owner_user_id) DO NOTHING
			RETURNING id`, ids["seller"]).Scan(&sellerID)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `SELECT id FROM sellers WHERE owner_user_id=$1`, ids["seller"]).Scan(&sellerID)
		}
		if err != nil {
			return fmt.Errorf("create QA seller: %w", err)
		}

		return nil
	}); err != nil {
		log.Fatal(err)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(struct {
		Accounts []account `json:"accounts"`
		Note     string    `json:"note"`
	}{accounts, "Passwords appear only for newly created accounts. Existing accounts keep their current credentials."}); err != nil {
		log.Fatal(err)
	}
}

func newPassword() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
