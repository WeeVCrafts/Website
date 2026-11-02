package handlers

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestSellerReturnFiltersAndSummariesBeforePagination(t *testing.T) {
	pool := projectionTestPool(t)
	_, err := pool.Exec(context.Background(), `
 INSERT INTO users(id,email,display_name,status) VALUES(920001,'reference-one@example.invalid','One','active'),(920002,'reference-two@example.invalid','Two','active');
 INSERT INTO sellers(id,owner_user_id,display_name,status) VALUES(920001,920001,'One','active'),(920002,920002,'Two','active');
 INSERT INTO categories(slug,name) VALUES('reference','Reference'); INSERT INTO brands(slug,name) VALUES('reference','Reference');
 INSERT INTO products(id,slug,brand_slug,category_slug,name,price_cents,seller_id,status) VALUES(920001,'reference-one','reference','reference','Owned product',1000,920001,'approved'),(920002,'reference-two','reference','reference','Foreign product',1000,920002,'approved');
 INSERT INTO product_variants(id,product_id,sku,price_cents,status) VALUES(920001,920001,'REF-ONE',1000,'active'),(920002,920002,'REF-TWO',1000,'active');
 INSERT INTO orders(id,order_number,user_id,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key)
 SELECT 920000+n,'REF-'||n,920001,'paid',1000,1000,'{}','reference',n::text FROM generate_series(1,60)n;
 INSERT INTO order_items(order_id,variant_id,product_name,sku,unit_price_cents,quantity,line_total_cents)
 SELECT id,CASE WHEN id=920060 THEN 920002 ELSE 920001 END,'Reference','REF',1000,1,1000 FROM orders;
 INSERT INTO return_requests(order_id,user_id,reason,status,created_at)
 SELECT id,920001,'Reference return',CASE WHEN (id-920000)%3=0 THEN 'received' WHEN (id-920000)%3=1 THEN 'pending' ELSE 'approved' END,CASE WHEN id=920001 THEN NOW() ELSE NOW()-INTERVAL '40 days' END FROM orders;
 `)
	if err != nil {
		t.Fatal(err)
	}
	h := NewLivePortalHandler(pool, nil, nil)
	r := projectionRequest(httptest.NewRequest("GET", "/seller-admin/returns?page=2", nil))
	stats := h.sellerWorkspaceStats(r, 920001, viewmodels.SellerAdminPage{Active: "returns"})
	for i, want := range []string{"59", "20", "20", "19"} {
		if len(stats) != 4 || stats[i].Value != want {
			t.Fatalf("summary %d: %+v, want %s", i, stats, want)
		}
	}
	rows := h.sellerReturns(r, 920001)
	if len(rows) != 9 || projectionPagination(r, "seller-returns").Total != 59 {
		t.Fatalf("page two: %d rows, %+v", len(rows), projectionPagination(r, "seller-returns"))
	}
	r = projectionRequest(httptest.NewRequest("GET", "/seller-admin/returns?status=approved&page=99", nil))
	rows = h.sellerReturns(r, 920001)
	if len(rows) != 20 || projectionPagination(r, "seller-returns").Page != 1 {
		t.Fatalf("filtered page: %d, %+v", len(rows), projectionPagination(r, "seller-returns"))
	}
	for _, row := range rows {
		if row.Status != "approved" || row.Product != "Owned product" {
			t.Fatalf("foreign or unfiltered return: %+v", row)
		}
	}
	r = projectionRequest(httptest.NewRequest("GET", "/seller-admin/returns?range=week", nil))
	rows = h.sellerReturns(r, 920001)
	if len(rows) != 1 || rows[0].Order != "REF-1" {
		t.Fatalf("week filter: %+v", rows)
	}
	if warning := projectionWarning(r, ""); warning != "" {
		t.Fatal(warning)
	}
}
