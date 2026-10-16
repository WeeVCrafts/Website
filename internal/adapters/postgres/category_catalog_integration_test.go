package postgres

import (
	"context"
	"errors"
	"fmt"
	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	"github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"io/fs"
	"testing"
	"time"
)

func TestManagedCategoryCrossListingAndPrimaryProtection(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	f := newCommerceFixture(t, pool, 1, 12)
	actor := f.userIDs[0]
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'admin')`, actor); err != nil {
		t.Fatal(err)
	}
	category := fmt.Sprintf("category-test-%d", time.Now().UnixNano())
	manager := NewManagementRepository(pool)
	service := applicationcommerce.NewManagementService(manager)
	mutation := ports.ManagementMutation{Section: "category-management", Action: "create", ID: category, Name: "Cross listed gifts", DisplayOrder: 7, ShowInNavigation: true, Reason: "Verify category curation"}
	if err := service.Mutate(ctx, actor, mutation); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `UPDATE products SET category_slug=$1 WHERE id=$2`, f.category, f.productID)
		_, _ = pool.Exec(ctx, `DELETE FROM product_categories WHERE category_slug=$1`, category)
		_, _ = pool.Exec(ctx, `DELETE FROM categories WHERE slug=$1`, category)
	})
	if err := manager.AssignCategoryProducts(ctx, f.ownerID, category, "add", "Forbidden seller curation", []int64{f.productID}); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("unauthorized assignment: %v", err)
	}
	search := NewProductSearchRepository(pool)
	before, err := search.CatalogRevision(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.AssignCategoryProducts(ctx, actor, category, "add", "Cross list gift", []int64{f.productID}); err != nil {
		t.Fatal(err)
	}
	if err := manager.AssignCategoryProducts(ctx, actor, category, "add", "Repeat the assignment", []int64{f.productID}); err != nil {
		t.Fatal(err)
	}
	approved, err := NewProductRepository(pool).ListApproved(ctx, category, 12)
	if err != nil || len(approved) != 1 || approved[0].ID != f.productID {
		t.Fatalf("SQL-backed catalog membership: %+v %v", approved, err)
	}
	after, err := search.CatalogRevision(ctx)
	if err != nil || after <= before {
		t.Fatalf("cache revision: %d %d %v", before, after, err)
	}
	options := ports.ProductSearchOptions{CategorySlugs: []string{f.category, category}, InStock: true, Limit: 25}
	products, total, err := search.SearchWithOptions(ctx, options)
	if err != nil || total != 1 || len(products) != 1 || products[0].ID != f.productID {
		t.Fatalf("deduplicated memberships: %+v %d %v", products, total, err)
	}
	facets, err := search.SearchFacets(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, facet := range facets {
		if facet.Value == category && facet.Count == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("secondary category missing from facets: %+v", facets)
	}
	list, err := service.List(ctx, actor, "category-management", category, 1)
	if err != nil || len(list.Records) != 1 || list.Records[0].Products != 1 || list.Records[0].DisplayOrder != 7 {
		t.Fatalf("managed projection: %+v %v", list, err)
	}
	if err := manager.AssignCategoryProducts(ctx, actor, f.category, "remove", "Cannot remove primary", []int64{f.productID}); !errors.Is(err, ports.ErrManagementConflict) {
		t.Fatalf("removed primary: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM product_categories WHERE product_id=$1 AND category_slug=$2`, f.productID, f.category); err == nil {
		t.Fatal("database allowed removal of primary membership")
	}
	mutation.Action = "delete"
	if err := service.Mutate(ctx, actor, mutation); !errors.Is(err, ports.ErrManagementConflict) {
		t.Fatalf("deleted secondary category in use: %v", err)
	}
	input := commerce.ProductDraftInput{Slug: products[0].Slug, BrandSlug: f.brandSlug, BrandName: "Fixture Brand", CategorySlug: category, CategoryName: "Cross listed gifts", CategorySlugs: []string{category}, Name: "Fixture Product", Description: "Confirmed fixture details", PriceCents: 1000, SKU: products[0].SKU, DeliveryLabel: "Test delivery"}
	if _, err := NewCommerceRepository(pool).UpdateAdminProduct(ctx, actor, f.productID, input); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM product_categories WHERE product_id=$1 AND category_slug=$2`, f.productID, f.category).Scan(&count); err != nil || count != 0 {
		t.Fatalf("replacement primary did not permit removing old category: %d %v", count, err)
	}
}

func TestMerchantImportRetriesPreserveInventoryAndExistingRecords(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	f := newCommerceFixture(t, pool, 1, 12)
	actor := f.userIDs[0]
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'admin'),($2,'seller')`, actor, f.ownerID); err != nil {
		t.Fatal(err)
	}
	var seller int64
	if err := pool.QueryRow(ctx, `SELECT seller_id FROM products WHERE id=$1`, f.productID).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	input := commerce.ProductDraftInput{Slug: fmt.Sprintf("merchant-import-%d", time.Now().UnixNano()), SKU: fmt.Sprintf("IMPORT-%d", time.Now().UnixNano()), BrandSlug: f.brandSlug, BrandName: "Fixture Brand", CategorySlug: f.category, CategorySlugs: []string{f.category}, Name: "Merchant inventory", Description: "Actual source details", PriceCents: 59950, InitialStock: 10, DeliveryLabel: "Merchant delivery"}
	repo := NewCommerceRepository(pool)
	if err := repo.CheckMerchantImport(ctx, actor, seller, input); err != nil {
		t.Fatal(err)
	}
	id, ready, err := repo.CreateMerchantImport(ctx, actor, seller, input)
	if err != nil || ready {
		t.Fatalf("first import: %d %v %v", id, ready, err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM products WHERE id=$1`, id) })
	if _, err := pool.Exec(ctx, `UPDATE inventory_stock SET available_quantity=4 WHERE variant_id IN(SELECT id FROM product_variants WHERE product_id=$1)`, id); err != nil {
		t.Fatal(err)
	}
	input.InitialStock = 999
	input.Name = "Do not overwrite edited product"
	repeat, ready, err := repo.CreateMerchantImport(ctx, actor, seller, input)
	if err != nil || repeat != id || ready {
		t.Fatalf("resume photo: %d %v %v", repeat, ready, err)
	}
	var stock int
	var name, status string
	if err := pool.QueryRow(ctx, `SELECT i.available_quantity,p.name,p.status FROM products p JOIN product_variants v ON v.product_id=p.id JOIN inventory_stock i ON i.variant_id=v.id WHERE p.id=$1`, id).Scan(&stock, &name, &status); err != nil || stock != 4 || name != "Merchant inventory" || status != "draft" {
		t.Fatalf("retry changed data: %d %s %s %v", stock, name, status, err)
	}
	existing := input
	if err := pool.QueryRow(ctx, `SELECT slug FROM products WHERE id=$1`, f.productID).Scan(&existing.Slug); err != nil {
		t.Fatal(err)
	}
	if err := repo.CheckMerchantImport(ctx, actor, seller, existing); err == nil {
		t.Fatal("unrelated existing listing could be overwritten")
	}
}

func TestCategoryMigrationBackfillsWithoutChangingProductsOrStock(t *testing.T) {
	pool := isolatedRolePool(t, true)
	ctx := context.Background()
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		version, _ := migrationVersion(entry.Name())
		if version < 42 || version >= 48 {
			continue
		}
		data, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(data)); err != nil {
			t.Fatal(entry.Name(), err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); err != nil {
			t.Fatal(err)
		}
	}
	fixture := newCommerceFixture(t, pool, 1, 12)
	if err := ApplyMigrations(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var memberships, stock int
	var slug string
	if err := pool.QueryRow(ctx, `SELECT (SELECT COUNT(*) FROM product_categories WHERE product_id=p.id AND category_slug=p.category_slug),p.category_slug,i.available_quantity FROM products p JOIN product_variants v ON v.product_id=p.id JOIN inventory_stock i ON i.variant_id=v.id WHERE p.id=$1`, fixture.productID).Scan(&memberships, &slug, &stock); err != nil {
		t.Fatal(err)
	}
	if memberships != 1 || slug != fixture.category || stock != 12 {
		t.Fatalf("migration changed existing inventory: memberships=%d category=%s stock=%d", memberships, slug, stock)
	}
}
