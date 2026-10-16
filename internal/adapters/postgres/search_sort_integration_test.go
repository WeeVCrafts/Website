package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestProductSearchOptionsUsesStablePriceOrdering(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	brandSlug := "search-brand-" + suffix
	categorySlug := "search-category-" + suffix
	if _, err := pool.Exec(ctx, `INSERT INTO brands (slug, name) VALUES ($1, 'Search Fixture Brand')`, brandSlug); err != nil {
		t.Fatalf("insert search brand: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO categories (slug, name,show_in_navigation) VALUES ($1, 'Search Fixture Category',true)`, categorySlug); err != nil {
		t.Fatalf("insert search category: %v", err)
	}
	var ownerID, sellerID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, display_name, status) VALUES ($1, 'Search Fixture Owner', 'active') RETURNING id`, "search-owner-"+suffix+"@example.invalid").Scan(&ownerID); err != nil {
		t.Fatalf("insert search owner: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO sellers (owner_user_id, display_name, status) VALUES ($1, 'Search Fixture Seller', 'active') RETURNING id`, ownerID).Scan(&sellerID); err != nil {
		t.Fatalf("insert search seller: %v", err)
	}
	productIDs := make([]int64, 0, 2)
	variantIDs := make([]int64, 0, 2)
	for index, price := range []int{900, 1900} {
		var productID, variantID int64
		slug := fmt.Sprintf("search-product-%s-%d", suffix, index)
		if err := pool.QueryRow(ctx, `INSERT INTO products (slug, brand_slug, category_slug, name, description, price_cents, seller_id, status) VALUES ($1, $2, $3, $4, 'Search fixture', $5, $6, 'approved') RETURNING id`, slug, brandSlug, categorySlug, fmt.Sprintf("Search Product %d", index), price, sellerID).Scan(&productID); err != nil {
			t.Fatalf("insert search product: %v", err)
		}
		if err := pool.QueryRow(ctx, `INSERT INTO product_variants (product_id, sku, price_cents) VALUES ($1, $2, $3) RETURNING id`, productID, fmt.Sprintf("SEARCH-%s-%d", suffix, index), price).Scan(&variantID); err != nil {
			t.Fatalf("insert search variant: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO inventory_stock (variant_id, available_quantity) VALUES ($1, 5)`, variantID); err != nil {
			t.Fatalf("insert search stock: %v", err)
		}
		productIDs = append(productIDs, productID)
		variantIDs = append(variantIDs, variantID)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, `DELETE FROM products WHERE id = ANY($1)`, productIDs)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM sellers WHERE id = $1`, sellerID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM users WHERE id = $1`, ownerID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM brands WHERE slug = $1`, brandSlug)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM categories WHERE slug = $1`, categorySlug)
	})

	products, total, err := NewProductSearchRepository(pool).SearchWithOptions(ctx, ports.ProductSearchOptions{CategorySlug: categorySlug, Sort: "price_desc", Limit: 2})
	if err != nil {
		t.Fatalf("search with sort: %v", err)
	}
	if total != 2 || len(products) != 2 || products[0].PriceCents != 1900 || products[1].PriceCents != 900 {
		t.Fatalf("unexpected sorted products: total=%d products=%+v", total, products)
	}
	if _, err := pool.Exec(ctx, `UPDATE product_variants SET price_cents=250000 WHERE id=$1`, variantIDs[0]); err != nil {
		t.Fatalf("update variant price: %v", err)
	}
	products, total, err = NewProductSearchRepository(pool).SearchWithOptions(ctx, ports.ProductSearchOptions{CategorySlug: categorySlug, PriceRange: "2500-5000", Sort: "price_desc", Limit: 2})
	if err != nil || total != 1 || len(products) != 1 || products[0].PriceCents != 250000 {
		t.Fatalf("listing price/filter did not follow its variant: total=%d products=%+v err=%v", total, products, err)
	}
	hydrated, err := NewProductSearchRepository(pool).SearchBySlugs(ctx, []string{"search-product-" + suffix + "-0"})
	if err != nil || len(hydrated) != 1 || hydrated[0].PriceCents != 250000 {
		t.Fatalf("product detail price did not follow its variant: products=%+v err=%v", hydrated, err)
	}
	facets, err := NewProductSearchRepository(pool).SearchFacets(ctx, ports.ProductSearchOptions{Query: "Search fixture"})
	if err != nil || len(facets) == 0 || facets[0].Value != categorySlug || facets[0].Count != 2 {
		t.Fatalf("unexpected category facets: facets=%+v err=%v", facets, err)
	}
}

func TestSuspendedSellerProductsAreHiddenAndCannotBePurchased(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	brandSlug, categorySlug := "suspended-brand-"+suffix, "suspended-category-"+suffix
	if _, err := pool.Exec(ctx, `INSERT INTO brands (slug, name) VALUES ($1, $1)`, brandSlug); err != nil {
		t.Fatalf("insert brand: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO categories (slug, name) VALUES ($1, $1)`, categorySlug); err != nil {
		t.Fatalf("insert category: %v", err)
	}
	var userID, sellerID, productID, variantID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, display_name, status) VALUES ($1, 'Suspended seller fixture', 'active') RETURNING id`, "suspended-seller-"+suffix+"@example.invalid").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO sellers (owner_user_id, display_name, status) VALUES ($1, 'Suspended seller fixture', 'active') RETURNING id`, userID).Scan(&sellerID); err != nil {
		t.Fatalf("insert seller: %v", err)
	}
	slug := "suspended-product-" + suffix
	if err := pool.QueryRow(ctx, `INSERT INTO products (slug, brand_slug, category_slug, name, description, price_cents, seller_id, status) VALUES ($1,$2,$3,'Suspension fixture','Suspension fixture',1000,$4,'approved') RETURNING id`, slug, brandSlug, categorySlug, sellerID).Scan(&productID); err != nil {
		t.Fatalf("insert product: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO product_variants (product_id, sku, price_cents) VALUES ($1,$2,1000) RETURNING id`, productID, "SUSPENDED-"+suffix).Scan(&variantID); err != nil {
		t.Fatalf("insert variant: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO inventory_stock (variant_id, available_quantity) VALUES ($1, 3)`, variantID); err != nil {
		t.Fatalf("insert stock: %v", err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, `DELETE FROM carts WHERE user_id=$1`, userID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM products WHERE id=$1`, productID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM sellers WHERE id=$1`, sellerID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM users WHERE id=$1`, userID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM brands WHERE slug=$1`, brandSlug)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM categories WHERE slug=$1`, categorySlug)
	})

	search := NewProductSearchRepository(pool)
	products, total, err := search.SearchWithOptions(ctx, ports.ProductSearchOptions{CategorySlug: categorySlug, Limit: 10})
	if err != nil || total != 1 || len(products) != 1 {
		t.Fatalf("active seller product was not listed: total=%d products=%+v err=%v", total, products, err)
	}
	cartRepository := NewCommerceRepository(pool)
	cart, err := cartRepository.GetOrCreateCart(ctx, userID, "")
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}
	if err := cartRepository.AddCartItem(ctx, cart.ID, variantID, 1); err != nil {
		t.Fatalf("add active seller product: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sellers SET status='suspended' WHERE id=$1`, sellerID); err != nil {
		t.Fatalf("suspend seller: %v", err)
	}
	products, total, err = search.SearchWithOptions(ctx, ports.ProductSearchOptions{CategorySlug: categorySlug, Limit: 10})
	if err != nil || total != 0 || len(products) != 0 {
		t.Fatalf("suspended seller product remained listed: total=%d products=%+v err=%v", total, products, err)
	}
	products, err = search.SearchBySlugs(ctx, []string{slug})
	if err != nil || len(products) != 0 {
		t.Fatalf("suspended seller detail remained visible: products=%+v err=%v", products, err)
	}
	if err := cartRepository.AddCartItem(ctx, cart.ID, variantID, 1); err == nil {
		t.Fatal("cart accepted a new item from a suspended seller")
	}
	_, err = cartRepository.CreateOrder(ctx, userID, cart.ID, "suspended-checkout-"+suffix, domaincommerce.AddressInput{RecipientName: "Test", Phone: "9876543210", Line1: "12 Test Road", City: "Bengaluru", State: "Karnataka", PostalCode: "560034", CountryCode: "IN"})
	if !errors.Is(err, ports.ErrInvalidState) {
		t.Fatalf("checkout did not reject the suspended seller's cart item: %v", err)
	}
}

func TestProductSearchOptionsApplyAllListingFilters(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	brandA, brandB := "search-filter-a-"+suffix, "search-filter-b-"+suffix
	categoryA, categoryB := "search-filter-a-"+suffix, "search-filter-b-"+suffix
	for _, brand := range []string{brandA, brandB} {
		if _, err := pool.Exec(ctx, `INSERT INTO brands (slug, name) VALUES ($1, $1)`, brand); err != nil {
			t.Fatalf("insert search brand: %v", err)
		}
	}
	for _, category := range []string{categoryA, categoryB} {
		if _, err := pool.Exec(ctx, `INSERT INTO categories (slug, name) VALUES ($1, $1)`, category); err != nil {
			t.Fatalf("insert search category: %v", err)
		}
	}
	var ownerID, sellerID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, display_name, status) VALUES ($1, 'Search Filter Owner', 'active') RETURNING id`, "search-filter-owner-"+suffix+"@example.invalid").Scan(&ownerID); err != nil {
		t.Fatalf("insert search owner: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO sellers (owner_user_id, display_name, status) VALUES ($1, 'Search Filter Seller', 'active') RETURNING id`, ownerID).Scan(&sellerID); err != nil {
		t.Fatalf("insert search seller: %v", err)
	}
	productIDs := make([]int64, 0, 5)
	addProduct := func(name, brand, category string, price int64, compareAt any, available *int) {
		t.Helper()
		var productID, variantID int64
		slug := "search-filter-" + name + "-" + suffix
		if err := pool.QueryRow(ctx, `INSERT INTO products (slug, brand_slug, category_slug, name, description, price_cents, compare_at_cents, seller_id, status) VALUES ($1, $2, $3, $4, 'Listing filter fixture', $5, $6, $7, 'approved') RETURNING id`, slug, brand, category, name, price, compareAt, sellerID).Scan(&productID); err != nil {
			t.Fatalf("insert product %s: %v", name, err)
		}
		if err := pool.QueryRow(ctx, `INSERT INTO product_variants (product_id, sku, price_cents, compare_at_cents) VALUES ($1, $2, $3, $4) RETURNING id`, productID, "FILTER-"+name+"-"+suffix, price, compareAt).Scan(&variantID); err != nil {
			t.Fatalf("insert variant %s: %v", name, err)
		}
		if available != nil {
			if _, err := pool.Exec(ctx, `INSERT INTO inventory_stock (variant_id, available_quantity) VALUES ($1, $2)`, variantID, *available); err != nil {
				t.Fatalf("insert stock %s: %v", name, err)
			}
		}
		productIDs = append(productIDs, productID)
	}
	stock := 3
	addProduct("match-a", brandA, categoryA, 259900, int64(299900), &stock)
	addProduct("match-b", brandA, categoryB, 259900, int64(299900), &stock)
	addProduct("out-stock", brandB, categoryA, 259900, int64(299900), nil)
	addProduct("no-offer", brandA, categoryA, 259900, nil, &stock)
	addProduct("low-price", brandA, categoryA, 90000, nil, &stock)
	documents, err := NewProductSearchRepository(pool).ListApprovedForIndex(ctx)
	if err != nil {
		t.Fatalf("list product index documents: %v", err)
	}
	indexedFixture := false
	for _, document := range documents {
		if document.Slug == "search-filter-match-a-"+suffix && (!document.HasOffer || document.BrandSlug != brandA || document.CompareAtCents != 299900) {
			t.Fatalf("search document omitted listing filter fields: %+v", document)
		}
		if document.Slug == "search-filter-match-a-"+suffix {
			indexedFixture = true
		}
	}
	if !indexedFixture {
		t.Fatal("approved fixture was not returned for search indexing")
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, `DELETE FROM products WHERE id = ANY($1)`, productIDs)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM sellers WHERE id = $1`, sellerID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM users WHERE id = $1`, ownerID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM brands WHERE slug = ANY($1::text[])`, []string{brandA, brandB})
		_, _ = pool.Exec(cleanupContext, `DELETE FROM categories WHERE slug = ANY($1::text[])`, []string{categoryA, categoryB})
	})

	repository := NewProductSearchRepository(pool)
	products, total, err := repository.SearchWithOptions(ctx, ports.ProductSearchOptions{OffersOnly: true, Sort: "newest", Limit: 24})
	if err != nil || total < 2 || len(products) == 0 {
		t.Fatalf("offer search without category and maker filters: total=%d products=%+v err=%v", total, products, err)
	}
	foundOffer := false
	for _, product := range products {
		if product.Slug == "search-filter-match-a-"+suffix {
			foundOffer = true
		}
	}
	if !foundOffer {
		t.Fatalf("empty listing filters hid the offer fixture: products=%+v", products)
	}

	options := ports.ProductSearchOptions{
		CategorySlugs: []string{categoryA, categoryB}, BrandSlugs: []string{brandA},
		PriceRange: "2500-5000", InStock: true, OffersOnly: true, Limit: 24,
	}
	products, total, err = repository.SearchWithOptions(ctx, options)
	if err != nil || total != 2 || len(products) != 2 {
		t.Fatalf("multi-value listing filters: total=%d products=%+v err=%v", total, products, err)
	}
	for _, product := range products {
		if product.Slug != "search-filter-match-a-"+suffix && product.Slug != "search-filter-match-b-"+suffix {
			t.Fatalf("unexpected filtered result: %+v", product)
		}
	}

	options.BrandSlugs = []string{brandB}
	products, total, err = repository.SearchWithOptions(ctx, options)
	if err != nil || total != 0 || len(products) != 0 {
		t.Fatalf("in-stock filter included the out-of-stock product: total=%d products=%+v err=%v", total, products, err)
	}
	options.InStock = false
	products, total, err = repository.SearchWithOptions(ctx, options)
	if err != nil || total != 1 || len(products) != 1 || products[0].Slug != "search-filter-out-stock-"+suffix {
		t.Fatalf("brand filter or out-of-stock results failed: total=%d products=%+v err=%v", total, products, err)
	}

	options = ports.ProductSearchOptions{CategorySlug: categoryA, BrandSlugs: []string{brandA}, PriceRange: "2500-5000", InStock: true, Limit: 24}
	products, total, err = repository.SearchWithOptions(ctx, options)
	if err != nil || total != 2 || len(products) != 2 {
		t.Fatalf("price band or non-offer results failed: total=%d products=%+v err=%v", total, products, err)
	}
	options.PriceRange = "under-1000"
	products, total, err = repository.SearchWithOptions(ctx, options)
	if err != nil || total != 1 || len(products) != 1 || products[0].Slug != "search-filter-low-price-"+suffix {
		t.Fatalf("low price band failed: total=%d products=%+v err=%v", total, products, err)
	}
}
