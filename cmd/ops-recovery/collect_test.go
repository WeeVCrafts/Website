package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
)

func TestRecoveryInventoryCoversCurrentSchemaAndDetectsNewReferences(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL required for recovery inventory verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = postgres.ApplyMigrations(ctx, pool); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if err = checkReferenceInventory(ctx, tx); err != nil {
		t.Fatalf("current object references are not covered: %v", err)
	}
	query, err := requiredObjectQuery(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, query)
	if err != nil {
		t.Fatalf("new schema object inventory: %v", err)
	}
	rows.Close()
	// Verify the collector can be installed before the application applies
	// the additive derivative migration.
	if _, err = tx.Exec(ctx, `DROP TABLE product_image_derivatives`); err != nil {
		t.Fatal(err)
	}
	query, err = requiredObjectQuery(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	rows, err = tx.Query(ctx, query)
	if err != nil {
		t.Fatalf("previous schema object inventory: %v", err)
	}
	rows.Close()
	if _, err = tx.Exec(ctx, `CREATE TABLE recovery_inventory_probe(object_key text)`); err != nil {
		t.Fatal(err)
	}
	if err = checkReferenceInventory(ctx, tx); err == nil {
		t.Fatal("recovery collector accepted an unmapped object reference")
	}
}
