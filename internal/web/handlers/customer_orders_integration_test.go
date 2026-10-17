package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/wecratfs/commerce/internal/adapters/postgres"
	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestCustomerOrderPagesFilterCountAndIsolateOwners(t *testing.T) {
	pool := projectionTestPool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO users(id,email,display_name,status) VALUES(902001,'customer-pages@example.invalid','Customer','active'),(902002,'private-pages@example.invalid','Other customer','active');
 INSERT INTO orders(order_number,user_id,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key,created_at)
 SELECT 'CUSTOMER-PAGE-'||n,902001,CASE WHEN n<=45 THEN 'paid' WHEN n<=50 THEN 'delivered' ELSE 'cancelled' END,1000,1000,'{}','customer-pages',n::text,now()-n*interval '1 minute' FROM generate_series(1,55)n;
 INSERT INTO orders(order_number,user_id,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key) VALUES('PRIVATE-CUSTOMER-ORDER',902002,'paid',1000,1000,'{}','customer-private','1');`)
	if err != nil {
		t.Fatal(err)
	}
	h := &LiveStorefrontHandler{pool: pool, orders: applicationcommerce.NewOrderService(postgres.NewCommerceRepository(pool))}
	for _, tc := range []struct {
		query               string
		length, page, total int
		first               string
	}{
		{"", 20, 1, 55, "CUSTOMER-PAGE-1"}, {"page=2", 20, 2, 55, "CUSTOMER-PAGE-21"}, {"page=3", 15, 3, 55, "CUSTOMER-PAGE-41"},
		{"status=delivered", 5, 1, 5, "CUSTOMER-PAGE-46"}, {"status=cancelled&page=999", 5, 1, 5, "CUSTOMER-PAGE-51"}, {"status=active&page=3", 5, 3, 45, "CUSTOMER-PAGE-41"},
	} {
		values, _ := url.ParseQuery(tc.query)
		page := viewmodels.CustomerPage{Filters: values}
		orders, err := h.customerOrderPage(ctx, 902001, &page)
		if err != nil {
			t.Fatal(err)
		}
		if len(orders) != tc.length || page.Page != tc.page || page.TotalResults != tc.total || orders[0].OrderNumber != tc.first {
			t.Fatalf("%s: page=%+v orders=%+v", tc.query, page, orders)
		}
		if page.OrderTotals[""] != 55 || page.OrderTotals["active"] != 45 || page.OrderTotals["delivered"] != 5 {
			t.Fatalf("counts include other owner: %+v", page.OrderTotals)
		}
	}
	for _, tc := range []struct {
		path     string
		status   int
		location string
	}{
		{"/payments/success", 303, "/payments"}, {"/payments/failed?order=CUSTOMER-PAGE-1", 303, "/orders/CUSTOMER-PAGE-1/confirmation"},
		{"/payments/success?order=CUSTOMER-PAGE-55", 303, "/orders/CUSTOMER-PAGE-55/confirmation"}, {"/payments/pending?order=PRIVATE-CUSTOMER-ORDER", 404, ""},
	} {
		w := httptest.NewRecorder()
		h.legacyCustomerPayment(w, httptest.NewRequest(http.MethodGet, tc.path, nil), 902001)
		if w.Code != tc.status || w.Header().Get("Location") != tc.location {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Header().Get("Location"))
		}
		if w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("payment redirect is cacheable")
		}
	}
}
