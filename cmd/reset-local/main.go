package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wecratfs/commerce/internal/adapters/postgres"
	"github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/config"
)

const defaultLocalPassword = "WeeVCrafts-Local-2026!"

type localAccount struct {
	email, name, phone, role string
}

func main() {
	if os.Getenv("WECRATFS_RESET_LOCAL") != "true" {
		log.Fatal("refusing to reset: set WECRATFS_RESET_LOCAL=true for an explicit local reset")
	}
	if os.Getenv("WECRATFS_LOCAL_MODE") != "true" {
		log.Fatal("refusing to reset: set WECRATFS_LOCAL_MODE=true for a local Docker reset")
	}
	if strings.EqualFold(os.Getenv("WECRATFS_ENV"), "production") {
		log.Fatal("refusing to reset a production environment")
	}

	cfg := config.FromEnv()
	if err := validateLocalDatabase(cfg.DatabaseURL); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
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
	password := envOrDefault("WECRATFS_RESET_PASSWORD", defaultLocalPassword)
	if err := resetAndSeed(ctx, pool, password); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Local database reset complete: 3 accounts and 1 local catalog fixture are ready.")
	fmt.Println("All three accounts use the password:", password)
}

func validateLocalDatabase(databaseURL string) error {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return fmt.Errorf("refusing to reset: invalid DATABASE_URL: %w", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if parsed.Scheme != "postgres" || (host != "localhost" && host != "postgres") {
		return fmt.Errorf("refusing to reset: DATABASE_URL must target local Docker Postgres (localhost or postgres), got %q", parsed.Host)
	}
	if strings.Trim(parsed.Path, "/") != "wecratfs" {
		return fmt.Errorf("refusing to reset: database name must be wecratfs, got %q", parsed.Path)
	}
	return nil
}

func resetAndSeed(ctx context.Context, pool *pgxpool.Pool, password string) error {
	hasher := auth.NewArgon2idHasher()
	passwordHash, err := hasher.Hash(password)
	if err != nil {
		return fmt.Errorf("hash local password: %w", err)
	}
	accounts := []localAccount{
		{email: "customer@weevcrafts.local", name: "Local Customer", phone: "+91 98765 43210", role: "customer"},
		{email: "seller@weevcrafts.local", name: "Local Seller Owner", phone: "+91 98765 43211", role: "seller_owner"},
		{email: "superadmin@weevcrafts.local", name: "Local Super Admin", phone: "+91 98765 43212", role: "super_admin"},
	}

	return postgres.WithinTransaction(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := truncateApplicationTables(ctx, tx); err != nil {
			return err
		}
		if err := seedRoles(ctx, tx); err != nil {
			return err
		}
		ids := make(map[string]int64, len(accounts))
		for _, account := range accounts {
			id, err := seedAccount(ctx, tx, account, passwordHash)
			if err != nil {
				return err
			}
			ids[account.role] = id
		}
		sellerID, err := seedSeller(ctx, tx, ids["seller_owner"])
		if err != nil {
			return err
		}
		if err := seedCatalogFixture(ctx, tx, sellerID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO addresses (user_id, recipient_name, phone, line1, city, state, postal_code, country_code) VALUES ($1, 'Local Customer', '+91 98765 43210', '12 Artisan Lane', 'Bengaluru', 'Karnataka', '560001', 'IN')`, ids["customer"])
		if err != nil {
			return fmt.Errorf("seed customer address: %w", err)
		}
		return nil
	})
}

func truncateApplicationTables(ctx context.Context, tx pgx.Tx) error {
	rows, err := tx.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename <> 'schema_migrations' ORDER BY tablename`)
	if err != nil {
		return fmt.Errorf("list application tables: %w", err)
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return fmt.Errorf("scan application table: %w", err)
		}
		tables = append(tables, `"`+strings.ReplaceAll(table, `"`, `""`)+`"`)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read application tables: %w", err)
	}
	if len(tables) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `TRUNCATE TABLE `+strings.Join(tables, ", ")+` RESTART IDENTITY CASCADE`); err != nil {
		return fmt.Errorf("truncate local application data: %w", err)
	}
	return nil
}

func seedRoles(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `INSERT INTO roles (slug, name) VALUES
		('customer', 'Customer'),
		('seller_owner', 'Seller owner'),
		('seller_staff', 'Seller staff'),
		('marketplace_admin', 'Marketplace admin'),
		('super_admin', 'Super admin'),
		('support_agent', 'Support agent'),
		('finance_operator', 'Finance operator'),
		('operations_returns', 'Operations and returns')`)
	if err != nil {
		return fmt.Errorf("seed roles: %w", err)
	}
	return nil
}

func seedAccount(ctx context.Context, tx pgx.Tx, account localAccount, passwordHash string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `INSERT INTO users (email, display_name, phone, status, email_verified_at) VALUES ($1, $2, $3, 'active', NOW()) RETURNING id`, account.email, account.name, account.phone).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("seed account %s: %w", account.email, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO credentials (user_id, password_hash) VALUES ($1, $2)`, id, passwordHash); err != nil {
		return 0, fmt.Errorf("seed credentials %s: %w", account.email, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_slug) VALUES ($1, $2)`, id, account.role); err != nil {
		return 0, fmt.Errorf("seed role %s: %w", account.email, err)
	}
	return id, nil
}

func seedSeller(ctx context.Context, tx pgx.Tx, ownerID int64) (int64, error) {
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO sellers (owner_user_id, display_name, status) VALUES ($1, 'Local Seller Studio', 'active') RETURNING id`, ownerID).Scan(&id); err != nil {
		return 0, fmt.Errorf("seed seller: %w", err)
	}
	return id, nil
}

func seedCatalogFixture(ctx context.Context, tx pgx.Tx, sellerID int64) error {
	if _, err := tx.Exec(ctx, `INSERT INTO brands (slug, name) VALUES ('local-studio', 'Local Studio')`); err != nil {
		return fmt.Errorf("seed local brand: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO categories (slug, name) VALUES ('textiles', 'Textiles')`); err != nil {
		return fmt.Errorf("seed local category: %w", err)
	}
	var productID int64
	if err := tx.QueryRow(ctx, `INSERT INTO products (slug, brand_slug, category_slug, name, description, price_cents, compare_at_cents, delivery_label, image_url, seller_id, status, featured_rank) VALUES ('local-starter-tote', 'local-studio', 'textiles', 'Indigo Everyday Tote', 'A local reset fixture for verifying catalog, cart and checkout flows.', 89900, 119900, 'Delivery in 4 - 6 days', '/assets/images/customer/blue-tote.webp', $1, 'approved', 1) RETURNING id`, sellerID).Scan(&productID); err != nil {
		return fmt.Errorf("seed local product: %w", err)
	}
	var variantID int64
	if err := tx.QueryRow(ctx, `INSERT INTO product_variants (product_id, sku, name, price_cents, compare_at_cents) VALUES ($1, 'LOCAL-TOTE-001', 'Standard', 89900, 119900) RETURNING id`, productID).Scan(&variantID); err != nil {
		return fmt.Errorf("seed local variant: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO inventory_stock (variant_id, available_quantity) VALUES ($1, 25)`, variantID); err != nil {
		return fmt.Errorf("seed local inventory: %w", err)
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
