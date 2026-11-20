package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestSellerPublishesAfterFirstListingIsApproved(t *testing.T) {
	pool := openIntegrationPool(t)
	f := newCommerceFixture(t, pool, 2, 2)
	ctx := t.Context()
	if _, err := pool.Exec(ctx, `UPDATE products SET status='draft' WHERE id=$1`, f.productID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'admin'),($2,'super_admin')`, f.userIDs[0], f.userIDs[1]); err != nil {
		t.Fatal(err)
	}
	repo := NewCommerceRepository(pool)
	next, err := repo.CreateDraft(ctx, f.ownerID, commerce.ProductDraftInput{Slug: f.brandSlug + "-next",
		BrandSlug: f.brandSlug, BrandName: "Fixture brand", CategorySlug: f.category, CategoryName: "Fixture category",
		Name: "Second listing", Description: "Fixture", SKU: f.brandSlug + "-next", InitialStock: 1, PriceCents: 1000})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM inventory_stock WHERE variant_id=$1`, next.VariantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM products WHERE id=$1`, next.ID)
	})
	if err := repo.SubmitProduct(ctx, f.ownerID, f.productID); err != nil {
		t.Fatal(err)
	}
	if err := repo.SubmitProduct(ctx, f.ownerID, next.ID); !errors.Is(err, ports.ErrFirstListingInReview) {
		t.Fatalf("second draft submitted while first under review: %v", err)
	}
	if err := repo.ApproveProduct(ctx, f.userIDs[0], f.productID, true, "Ordinary admin"); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("ordinary admin approved first listing: %v", err)
	}
	if err := repo.ApproveProduct(ctx, f.userIDs[1], f.productID, true, "First listing reviewed"); err != nil {
		t.Fatal(err)
	}
	if err := repo.ArchiveSellerProduct(ctx, f.ownerID, f.productID, "Discontinued listing"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SubmitProduct(ctx, f.ownerID, next.ID); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetSellerProduct(ctx, f.ownerID, next.ID)
	if err != nil || got.Status != "approved" {
		t.Fatalf("second listing: %+v, %v", got, err)
	}
}

func TestApprovedProductPublishesTransactionalIndexEvent(t *testing.T) {
	pool := openIntegrationPool(t)
	fixture := newCommerceFixture(t, pool, 1, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `UPDATE products SET status = 'pending_review' WHERE id = $1`, fixture.productID); err != nil {
		t.Fatalf("prepare product review: %v", err)
	}
	adminEmail := fmt.Sprintf("index-admin-%d@example.invalid", time.Now().UnixNano())
	var adminID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, display_name, status) VALUES ($1, 'Index Admin', 'active') RETURNING id`, adminEmail).Scan(&adminID); err != nil {
		t.Fatalf("insert index admin: %v", err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, `DELETE FROM outbox_events WHERE event_type = 'ProductIndexRequested' AND aggregate_id = $1`, fmt.Sprint(fixture.productID))
		_, _ = pool.Exec(cleanupContext, `DELETE FROM users WHERE id = $1`, adminID)
	})
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_slug) VALUES ($1, 'super_admin')`, adminID); err != nil {
		t.Fatalf("grant index admin role: %v", err)
	}
	if err := NewCommerceRepository(pool).ApproveProduct(ctx, adminID, fixture.productID, true, "ready for search"); err != nil {
		t.Fatalf("approve product: %v", err)
	}
	var eventCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_events WHERE event_type = 'ProductIndexRequested' AND aggregate_id = $1 AND payload->>'status' = 'approved'`, fmt.Sprint(fixture.productID)).Scan(&eventCount); err != nil {
		t.Fatalf("count product index events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected one product index event, got %d", eventCount)
	}
}
