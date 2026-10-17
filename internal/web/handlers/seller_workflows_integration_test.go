package handlers

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestSellerInventoryFiltersAggregateAndHistoryStayScoped(t *testing.T) {
	pool := projectionTestPool(t)
	_, err := pool.Exec(context.Background(), `
INSERT INTO users(id,email,display_name,status) VALUES(910001,'stock1@example.invalid','Stock one','active'),(910002,'stock2@example.invalid','Stock two','active');
INSERT INTO sellers(id,owner_user_id,display_name,status) VALUES(910001,910001,'Stock one','active'),(910002,910002,'Stock two','active');
INSERT INTO categories(slug,name) VALUES('stock-test','Stock test'); INSERT INTO brands(slug,name) VALUES('stock-test','Stock test');
INSERT INTO products(id,slug,brand_slug,category_slug,name,price_cents,seller_id,status) VALUES(910001,'stock-one','stock-test','stock-test','Stock product',1000,910001,'approved'),(910002,'stock-two','stock-test','stock-test','Private product',1000,910002,'approved');
INSERT INTO product_variants(id,product_id,sku,price_cents,status) VALUES(910001,910001,'STOCK-ONE',1000,'active'),(910002,910001,'STOCK-TWO',1000,'active'),(910003,910001,'STOCK-ARCHIVED',1000,'archived'),(910004,910002,'STOCK-PRIVATE',1000,'active');
INSERT INTO inventory_stock(variant_id,available_quantity,reserved_quantity) VALUES(910001,0,2),(910002,8,1),(910003,100,0),(910004,3,0);
INSERT INTO inventory_transactions(seller_id,variant_id,actor_id,delta,before_quantity,after_quantity,reason) SELECT 910001,910002,910001,1,n,n+1,'Recorded restock '||n FROM generate_series(1,25) n;
INSERT INTO inventory_transactions(seller_id,variant_id,actor_id,delta,before_quantity,after_quantity,reason) VALUES(910002,910004,910002,1,2,3,'Private stock history');`)
	if err != nil {
		t.Fatal(err)
	}
	h := NewLivePortalHandler(pool, nil, nil)
	for _, filter := range []string{"low", "out"} {
		r := projectionRequest(httptest.NewRequest("GET", "/seller-admin/inventory?stock="+filter, nil))
		items := h.sellerInventory(r, 910001, "", 5)
		if len(items) != 1 || items[0].VariantID != "910001" || items[0].Reserved != "2" || projectionPagination(r, "seller-inventory").Total != 1 {
			t.Fatalf("filter %s: %+v", filter, items)
		}
	}
	r := projectionRequest(httptest.NewRequest("GET", "/seller-admin/products", nil))
	products := h.sellerProducts(r, 910001, "")
	if len(products) != 1 || products[0].Stock != "8" {
		t.Fatalf("active-variant stock total: %+v", products)
	}
	r = projectionRequest(httptest.NewRequest("GET", "/seller-admin/inventory", nil))
	if rows := h.sellerInventoryHistory(r, 910001, 910002); len(rows) != 20 || rows[0].Delta != "+1" {
		t.Fatalf("bounded history: %+v", rows)
	}
	if rows := h.sellerInventoryHistory(r, 910001, 910004); len(rows) != 0 {
		t.Fatal("other seller history leaked")
	}
	if warning := projectionWarning(r, ""); warning != "" {
		t.Fatal(warning)
	}
}
