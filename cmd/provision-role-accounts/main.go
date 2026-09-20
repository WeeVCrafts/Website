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
	Password string `json:"password"`
}

var roleAccounts = []account{
	{Email: "customer@weevcrafts.test", Name: "QA Customer", Role: "customer"},
	{Email: "seller@weevcrafts.test", Name: "QA Seller Owner", Role: "seller_owner"},
	{Email: "sellerstaff@weevcrafts.test", Name: "QA Seller Staff", Role: "seller_staff"},
	{Email: "marketplaceadmin@weevcrafts.test", Name: "QA Marketplace Admin", Role: "marketplace_admin"},
	{Email: "superadmin@weevcrafts.test", Name: "QA Super Admin", Role: "super_admin"},
	{Email: "support@weevcrafts.test", Name: "QA Support Agent", Role: "support_agent"},
	{Email: "finance@weevcrafts.test", Name: "QA Finance Operator", Role: "finance_operator"},
	{Email: "operations@weevcrafts.test", Name: "QA Operations and Returns", Role: "operations_returns"},
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
	hasher := auth.NewArgon2idHasher()
	hashes := make([]string, len(accounts))
	for i := range accounts {
		accounts[i].Password, err = newPassword()
		if err != nil {
			log.Fatal(err)
		}
		hashes[i], err = hasher.Hash(accounts[i].Password)
		if err != nil {
			log.Fatal(err)
		}
	}

	if err := postgres.WithinTransaction(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		ids := make(map[string]int64, len(accounts))
		for i, item := range accounts {
			var id int64
			err := tx.QueryRow(ctx, `
				INSERT INTO users (email, display_name, status, email_verified_at)
				VALUES ($1, $2, 'active', NOW())
				RETURNING id`, item.Email, item.Name).Scan(&id)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("create %s: no user id returned", item.Email)
				}
				return fmt.Errorf("create %s (existing QA emails are not overwritten): %w", item.Email, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO credentials (user_id, password_hash) VALUES ($1, $2)`, id, hashes[i]); err != nil {
				return fmt.Errorf("create credentials for %s: %w", item.Email, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_slug) VALUES ($1, $2)`, id, item.Role); err != nil {
				return fmt.Errorf("assign role %s to %s: %w", item.Role, item.Email, err)
			}
			ids[item.Role] = id
		}

		var sellerID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO sellers (owner_user_id, display_name, status)
			VALUES ($1, 'WeeVCrafts QA Seller', 'active')
			RETURNING id`, ids["seller_owner"]).Scan(&sellerID); err != nil {
			return fmt.Errorf("create QA seller: %w", err)
		}
		permissions := []string{"PRODUCT_READ", "PRODUCT_WRITE", "INVENTORY_READ", "INVENTORY_WRITE", "ORDER_READ", "ORDER_FULFILL", "RETURN_READ", "REPORT_READ"}
		if _, err := tx.Exec(ctx, `
			INSERT INTO seller_users (seller_id, user_id, status, permissions, added_by)
			VALUES ($1, $2, 'active', $3, $4)`, sellerID, ids["seller_staff"], permissions, ids["seller_owner"]); err != nil {
			return fmt.Errorf("associate QA seller staff: %w", err)
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
	}{accounts, "Save these one-time passwords securely. This command never prints or stores them again."}); err != nil {
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
