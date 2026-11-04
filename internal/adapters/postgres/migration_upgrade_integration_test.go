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

func TestPaymentReviewMigrationBackfillsRecordedLateCapture(t *testing.T) {
	base := openIntegrationPool(t)
	ctx := context.Background()
	schema := fmt.Sprintf("qa_payment_upgrade_%d", time.Now().UnixNano())
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
		if version >= 54 {
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
	if _, err = pool.Exec(ctx, `
	INSERT INTO users(email,display_name,status) VALUES('payment-upgrade@example.invalid','QA Finance','active');
	INSERT INTO user_roles(user_id,role_slug) SELECT id,'super_admin' FROM users;
	INSERT INTO orders(user_id,order_number,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key)
	 SELECT id,'QA-HISTORICAL-LATE','payment_failed',1000,1000,'{}','upgrade','late' FROM users;
	INSERT INTO orders(user_id,order_number,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key)
	 SELECT id,'QA-HISTORICAL-PAID','paid',1000,1000,'{}','upgrade','paid' FROM users;
	INSERT INTO payment_attempts(order_id,provider,provider_order_id,provider_payment_id,status,amount_cents)
	 SELECT id,'razorpay','order_'||order_number,'pay_'||order_number,'captured',1000 FROM orders;`); err != nil {
		t.Fatal(err)
	}
	if err = ApplyMigrations(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var cases, attempts, notifications, audits int
	var lateHeld, paidHeld bool
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM payment_review_cases),(SELECT count(*) FROM provider_payment_attempts),(SELECT count(*) FROM notifications WHERE kind='payment_review'),(SELECT count(*) FROM audit_logs WHERE action='payment.review_backfill'),(SELECT payment_review_required FROM orders WHERE order_number='QA-HISTORICAL-LATE'),(SELECT payment_review_required FROM orders WHERE order_number='QA-HISTORICAL-PAID')`).Scan(&cases, &attempts, &notifications, &audits, &lateHeld, &paidHeld); err != nil {
		t.Fatal(err)
	}
	if cases != 1 || attempts != 2 || notifications != 1 || audits != 1 || !lateHeld || paidHeld {
		t.Fatalf("upgrade cases=%d attempts=%d notifications=%d audits=%d holds=%v/%v", cases, attempts, notifications, audits, lateHeld, paidHeld)
	}
	if err = ApplyMigrations(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM payment_review_cases`).Scan(&cases); err != nil || cases != 1 {
		t.Fatalf("upgrade replay cases=%d err=%v", cases, err)
	}
}
