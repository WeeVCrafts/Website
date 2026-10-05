package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestSellerProductVariantLifecycleGuards(t *testing.T) {
	pool := openIntegrationPool(t)
	fixture := newCommerceFixture(t, pool, 0, 5)
	repository := NewCommerceRepository(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var outsiderID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status) VALUES($1,'Variant Outsider','active') RETURNING id`, "variant-outsider-"+fixture.brandSlug+"@example.invalid").Scan(&outsiderID); err != nil {
		t.Fatalf("create unrelated user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, outsiderID) })
	compareAt := int64(2500)
	created, err := repository.CreateSellerProductVariant(ctx, fixture.ownerID, fixture.productID, commerce.ProductVariantInput{
		Name: "Peacock Blue", SKU: "FIXTURE-BLUE", PriceCents: 2000, CompareAtCents: &compareAt, InitialStock: 4,
	})
	if err != nil {
		t.Fatalf("create variant: %v", err)
	}
	if created.ID <= 0 || created.Available != 4 || created.Status != "active" || !created.HasCompareAt {
		t.Fatalf("unexpected created variant: %+v", created)
	}

	variants, err := repository.ListSellerProductVariants(ctx, fixture.ownerID, fixture.productID)
	if err != nil || len(variants) != 2 {
		t.Fatalf("list variants: got %d, err=%v", len(variants), err)
	}
	updated, err := repository.UpdateSellerProductVariant(ctx, fixture.ownerID, fixture.productID, created.ID, commerce.ProductVariantInput{
		Name: "Deep Blue", SKU: "FIXTURE-BLUE-2", PriceCents: 2200,
	})
	if err != nil {
		t.Fatalf("update variant: %v", err)
	}
	if updated.Name != "Deep Blue" || updated.SKU != "FIXTURE-BLUE-2" || updated.PriceCents != 2200 || updated.Available != 4 {
		t.Fatalf("update changed wrong fields: %+v", updated)
	}
	if _, err := pool.Exec(ctx, `UPDATE inventory_stock SET reserved_quantity=1 WHERE variant_id=$1`, created.ID); err != nil {
		t.Fatalf("set fixture reservation: %v", err)
	}
	if err := repository.ArchiveSellerProductVariant(ctx, fixture.ownerID, fixture.productID, created.ID, "retire blue option"); !errors.Is(err, ports.ErrVariantReservedStock) {
		t.Fatalf("reserved variant archive returned %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE inventory_stock SET reserved_quantity=0 WHERE variant_id=$1`, created.ID); err != nil {
		t.Fatalf("clear fixture reservation: %v", err)
	}
	if err := repository.ArchiveSellerProductVariant(ctx, fixture.ownerID, fixture.productID, created.ID, "retire blue option"); err != nil {
		t.Fatalf("archive unreserved variant: %v", err)
	}
	variants, err = repository.ListSellerProductVariants(ctx, fixture.ownerID, fixture.productID)
	if err != nil || len(variants) != 2 || variants[1].Status != "archived" {
		t.Fatalf("archived variant not retained in history: variants=%+v err=%v", variants, err)
	}
	if err := repository.ArchiveSellerProductVariant(ctx, fixture.ownerID, fixture.productID, fixture.variantID, "retire last option"); !errors.Is(err, ports.ErrFinalActiveVariant) {
		t.Fatalf("archiving final active variant returned %v", err)
	}
	if err := repository.ArchiveSellerProductVariant(ctx, outsiderID, fixture.productID, fixture.variantID, "wrong seller"); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("unrelated account variant archive returned %v", err)
	}
}
