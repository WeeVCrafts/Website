package postgres

import (
	"context"
	"fmt"
	"io/fs"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestUpgradeRepairsEitherHistoricalMigration28(t *testing.T) {
	base := openIntegrationPool(t)
	for _, oldMigration := range []string{"0028_seller_follows.sql", "0034_support_attachments.sql"} {
		t.Run(oldMigration, func(t *testing.T) {
			ctx := context.Background()
			schema := fmt.Sprintf("qa_upgrade_%d", time.Now().UnixNano())
			quoted := pgx.Identifier{schema}.Sanitize()
			if _, err := base.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = base.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE") })
			cfg := base.Config().Copy()
			cfg.ConnConfig.RuntimeParams["search_path"] = schema
			pool, err := pgxpool.NewWithConfig(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(pool.Close)
			if _, err = pool.Exec(ctx, `CREATE TABLE schema_migrations(version BIGINT PRIMARY KEY,applied_at TIMESTAMPTZ DEFAULT NOW())`); err != nil {
				t.Fatal(err)
			}
			entries, err := fs.ReadDir(migrationFiles, "migrations")
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				version, err := migrationVersion(entry.Name())
				if err != nil {
					t.Fatal(err)
				}
				if version >= 28 {
					continue
				}
				data, err := migrationFiles.ReadFile("migrations/" + entry.Name())
				if err != nil {
					t.Fatal(err)
				}
				if _, err = pool.Exec(ctx, string(data)); err != nil {
					t.Fatalf("historical migration %s: %v", entry.Name(), err)
				}
				if _, err = pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); err != nil {
					t.Fatal(err)
				}
			}
			data, err := migrationFiles.ReadFile("migrations/" + oldMigration)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, string(data)); err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES(28); INSERT INTO users(email,display_name,status) VALUES('upgrade@example.invalid','Preserved customer','active'); INSERT INTO user_roles(user_id,role_slug) SELECT id,'customer' FROM users WHERE email='upgrade@example.invalid'`); err != nil {
				t.Fatal(err)
			}
			if err = ApplyMigrations(ctx, pool); err != nil {
				t.Fatal(err)
			}
			var follows, attachments, users int
			if err = pool.QueryRow(ctx, `SELECT (SELECT COUNT(*) FROM seller_follows),(SELECT COUNT(*) FROM support_attachments),(SELECT COUNT(*) FROM users WHERE email='upgrade@example.invalid')`).Scan(&follows, &attachments, &users); err != nil {
				t.Fatal(err)
			}
			if users != 1 {
				t.Fatalf("upgrade preserved %d fixture users; want 1", users)
			}
		})
	}
}
