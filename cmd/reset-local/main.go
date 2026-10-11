package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"github.com/wecratfs/commerce/internal/adapters/meilisearch"
	"github.com/wecratfs/commerce/internal/ports"
	"log"
	"net"
	"net/http"
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
	reset := os.Getenv("WECRATFS_RESET_LOCAL") == "true"
	bootstrap := os.Getenv("WECRATFS_SEED_EMPTY") == "true"
	if !reset && !bootstrap {
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
	if bootstrap && !reset {
		var count int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
			log.Fatal(err)
		}
		if count > 0 {
			fmt.Println("Existing accounts preserved; clean bootstrap skipped.")
			return
		}
	}
	if reset {
		if err := validateLocalServices(cfg); err != nil {
			log.Fatal(err)
		}
	}
	password := envOrDefault("WECRATFS_RESET_PASSWORD", defaultLocalPassword)
	if len(password) < 12 {
		log.Fatal("local password must contain at least 12 characters")
	}
	if err := resetAndSeed(ctx, pool, password, reset); err != nil {
		log.Fatal(err)
	}
	if reset {
		if err := clearLocalServices(ctx, cfg); err != nil {
			log.Fatal("Database reset succeeded; finish dependent data cleanup before restarting app: ", err)
		}
	}
	fmt.Println("Local setup complete: 3 active accounts, 1 seller and an empty catalogue.")
}

func validateLocalDatabase(databaseURL string) error {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return fmt.Errorf("refusing to reset: invalid DATABASE_URL: %w", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || (host != "localhost" && host != "127.0.0.1" && host != "::1" && host != "postgres") {
		return fmt.Errorf("refusing to reset: DATABASE_URL must target local Docker Postgres (localhost or postgres), got %q", parsed.Host)
	}
	if strings.Trim(parsed.Path, "/") != "wecratfs" {
		return fmt.Errorf("refusing to reset: database name must be wecratfs, got %q", parsed.Path)
	}
	return nil
}

func resetAndSeed(ctx context.Context, pool *pgxpool.Pool, password string, reset bool) error {
	hasher := auth.NewArgon2idHasher()
	passwordHash, err := hasher.Hash(password)
	if err != nil {
		return fmt.Errorf("hash local password: %w", err)
	}
	accounts := []localAccount{
		{email: "customer@weevcrafts.local", name: "Local Customer", phone: "", role: "customer"},
		{email: "seller@weevcrafts.local", name: "Local Seller Owner", phone: "", role: "seller_owner"},
		{email: "superadmin@weevcrafts.local", name: "Local Super Admin", phone: "", role: "super_admin"},
	}

	return postgres.WithinTransaction(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(816231)"); err != nil {
			return err
		}
		if !reset {
			var count int
			if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
				return err
			}
			if count > 0 {
				return nil
			}
		}

		if reset {
			if err := truncateApplicationTables(ctx, tx); err != nil {
				return err
			}
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
		_, err := seedSeller(ctx, tx, ids["seller_owner"])
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO categories (slug,name) VALUES ('textiles','Textiles'),('home','Home and living'),('art','Art'),('accessories','Accessories'),('wellness','Wellness') ON CONFLICT (slug) DO NOTHING; INSERT INTO brands (slug,name) VALUES ('local-studio','Local Studio') ON CONFLICT (slug) DO NOTHING`); err != nil {
			return err
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
		('operations_returns', 'Operations and returns') ON CONFLICT (slug) DO NOTHING`)
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

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// Validate every destructive endpoint before clearing any database rows.
func localService(address string, aliases ...string) bool {
	if strings.Contains(address, "://") {
		u, err := url.Parse(address)
		if err != nil {
			return false
		}
		address = u.Host
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	for _, alias := range aliases {
		if host == alias {
			return true
		}
	}
	return false
}
func validateLocalServices(cfg config.Config) error {
	if cfg.RedisURL != "" || !localService(cfg.RedisAddress, "redis") || !localService(cfg.MeiliAddress, "meilisearch") || !localService(cfg.S3Address, "garage") || cfg.S3Bucket != "wecratfs-media" || cfg.S3Secure {
		return errors.New("reset requires the local Compose Redis, Meilisearch and wecratfs-media storage bucket")
	}
	return nil
}
func clearLocalServices(ctx context.Context, cfg config.Config) error {
	cache := redis.NewClient(&redis.Options{Addr: cfg.RedisAddress})
	defer cache.Close()
	if err := cache.FlushDB(ctx).Err(); err != nil {
		return fmt.Errorf("clear local sessions/cache: %w", err)
	}
	if err := meilisearch.NewSearchEngineWithKey(cfg.MeiliAddress, cfg.MeiliAPIKey).Rebuild(ctx, []ports.SearchDocument{}); err != nil {
		return fmt.Errorf("clear product search: %w", err)
	}
	storage, err := minio.New(cfg.S3Address, &minio.Options{Creds: credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""), Secure: false, Region: "garage"})
	if err != nil {
		return err
	}
	for object := range storage.ListObjects(ctx, cfg.S3Bucket, minio.ListObjectsOptions{Recursive: true}) {
		if object.Err != nil {
			return object.Err
		}
		if err := storage.RemoveObject(ctx, cfg.S3Bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}
	// Mailpit is the local development inbox; remove messages containing old user data.
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "http://localhost:8025/api/v1/messages", nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("clear local Mailpit inbox: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("Mailpit cleanup status %d", response.StatusCode)
	}
	return nil
}
