package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

func TestDiscoverySearchTagsTyposFacetsAndFeatured(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	suffix := fmt.Sprint(time.Now().UnixNano())
	brand, category := "discovery-"+suffix, "pottery-"+suffix
	must := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	must(`INSERT INTO brands(slug,name) VALUES($1,'Copper Workshop')`, brand)
	must(`INSERT INTO categories(slug,name,show_in_navigation) VALUES($1,'Pottery Ceramics',true)`, category)
	var user, seller int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status) VALUES($1,'Discovery QA','active') RETURNING id`, suffix+"@example.invalid").Scan(&user); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO sellers(owner_user_id,display_name,status) VALUES($1,'Discovery QA','active') RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	ids, variants := []int64{}, []int64{}
	for i, name := range []string{"Ceramic coffee mug", "Copper bowl"} {
		var id, variant int64
		if err := pool.QueryRow(ctx, `INSERT INTO products(slug,brand_slug,category_slug,name,description,price_cents,seller_id,status,search_tags) VALUES($1,$2,$3,$4,'Handcrafted kitchenware',10000,$5,'approved',ARRAY['floral','kitchen']) RETURNING id`, fmt.Sprintf("discovery-%s-%d", suffix, i), brand, category, name, seller).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `INSERT INTO product_variants(product_id,sku,price_cents) VALUES($1,$2,10000) RETURNING id`, id, fmt.Sprintf("DISCOVERY-%s-%d", suffix, i)).Scan(&variant); err != nil {
			t.Fatal(err)
		}
		must(`INSERT INTO inventory_stock(variant_id,available_quantity) VALUES($1,10)`, variant)
		ids = append(ids, id)
		variants = append(variants, variant)
	}
	repository := NewProductSearchRepository(pool)
	for query, want := range map[string]int{"ceramic mug": 1, "cermaic mug": 1, "floral": 2, "Pottery": 2, "Copper Workshop": 2, fmt.Sprintf("DISCOVERY-%s-0", suffix): 1} {
		options := ports.ProductSearchOptions{Query: query, CategorySlug: category, Limit: 24}
		products, total, err := repository.SearchWithOptions(ctx, options)
		if err != nil || total != want || len(products) != want {
			t.Fatalf("query %q: total=%d products=%d error=%v", query, total, len(products), err)
		}
		facets, err := repository.SearchFacets(ctx, options)
		if err != nil || len(facets) != 1 || facets[0].Count != want {
			t.Fatalf("query %q facets=%+v err=%v", query, facets, err)
		}
	}
	must(`UPDATE products SET search_tags=ARRAY['tea'] WHERE id=$1`, ids[0])
	products, total, err := repository.SearchWithOptions(ctx, ports.ProductSearchOptions{Query: "tea", CategorySlug: category, Limit: 24})
	if err != nil || total != 1 || products[0].ID != ids[0] {
		t.Fatalf("updated tags not searchable: %v", err)
	}
	var order int64
	if err := pool.QueryRow(ctx, `INSERT INTO orders(order_number,user_id,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key) VALUES($1,$2,'paid',20000,20000,'{}',$1,$1) RETURNING id`, "DISCOVERY-"+suffix, user).Scan(&order); err != nil {
		t.Fatal(err)
	}
	must(`INSERT INTO order_items(order_id,variant_id,product_name,sku,unit_price_cents,quantity,line_total_cents) VALUES($1,$2,'Copper bowl','saved-sku',10000,2,20000)`, order, variants[1])
	must(`INSERT INTO payment_attempts(order_id,provider,status,amount_cents) VALUES($1,'qa','captured',20000),($1,'qa','captured',20000)`, order)
	// Advance the existing snapshot before asserting newly recorded sales.
	must(`UPDATE featured_popularity SET refreshed_at=now()-interval '16 minutes'`)
	options := ports.ProductSearchOptions{CategorySlug: category, Featured: true, InStock: true, Limit: 24}
	products, _, err = repository.SearchWithOptions(ctx, options)
	if err != nil || products[0].ID != ids[1] {
		t.Fatalf("popular product not first: %v", err)
	}
	var units int64
	if err := pool.QueryRow(ctx, `SELECT paid_units FROM featured_popularity WHERE product_id=$1`, ids[1]).Scan(&units); err != nil || units != 2 {
		t.Fatalf("duplicate payments inflated sales: units=%d err=%v", units, err)
	}
	must(`UPDATE products SET featured_pinned=true,featured_rank=1 WHERE id=$1`, ids[0])
	products, _, err = repository.SearchWithOptions(ctx, options)
	if err != nil || products[0].ID != ids[0] {
		t.Fatalf("pin not first: %v", err)
	}
	must(`UPDATE inventory_stock SET available_quantity=0 WHERE variant_id=$1`, variants[0])
	products, total, err = repository.SearchWithOptions(ctx, options)
	if err != nil || total != 1 || products[0].ID != ids[1] {
		t.Fatalf("unavailable pin visible: %v", err)
	}
	must(`INSERT INTO payment_refunds(order_id,request_key,amount_cents,currency,status) VALUES($1,$2,20000,'INR','processed')`, order, suffix)
	must(`UPDATE featured_popularity SET refreshed_at=now()-interval '16 minutes'`)
	if _, _, err = repository.SearchWithOptions(ctx, options); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT paid_units FROM featured_popularity WHERE product_id=$1`, ids[1]).Scan(&units); err != nil || units != 0 {
		t.Fatalf("refunded sale counted: units=%d err=%v", units, err)
	}
	// This test intentionally retains synthetic records in the isolated QA DB.
}
