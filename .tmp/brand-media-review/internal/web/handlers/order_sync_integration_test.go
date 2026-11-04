package handlers

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	postgresadapter "github.com/wecratfs/commerce/internal/adapters/postgres"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestSellerStagesReachCustomerAndCompanyProjections(t *testing.T) {
	pool := projectionTestPool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
INSERT INTO users(id,email,display_name,status) VALUES(901001,'owner@example.invalid','Owner','active'),(901002,'customer@example.invalid','Customer','active');
INSERT INTO user_roles(user_id,role_slug) VALUES(901001,'seller'),(901002,'customer');
INSERT INTO sellers(id,owner_user_id,display_name,status) VALUES(901001,901001,'Stage Seller','active');
INSERT INTO categories(slug,name) VALUES('stage','Stage');
INSERT INTO brands(slug,name) VALUES('stage','Stage');
INSERT INTO products(id,slug,brand_slug,category_slug,name,price_cents,seller_id,status) VALUES(901001,'stage','stage','stage','Stage Product',1000,901001,'approved');
INSERT INTO product_variants(id,product_id,sku,price_cents) VALUES(901001,901001,'STAGE-SKU',1000);
INSERT INTO orders(id,order_number,user_id,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key) VALUES(901001,'STAGE-ORDER',901002,'paid',1000,1000,'{}','stage','1');
INSERT INTO order_items(order_id,variant_id,product_name,sku,unit_price_cents,quantity,line_total_cents) VALUES(901001,901001,'Stage Product','STAGE-SKU',1000,1,1000);
INSERT INTO payment_attempts(order_id,provider,status,amount_cents) VALUES(901001,'local','captured',1000);
`)
	if err != nil {
		t.Fatal(err)
	}
	h := NewLivePortalHandler(pool, nil, nil)
	repository := postgresadapter.NewCommerceRepository(pool)
	for index, stage := range domaincommerce.FulfillmentStages[1:] {
		carrier, tracking := "", ""
		if stage.Index() >= domaincommerce.FulfillmentShipped.Index() {
			carrier, tracking = "QA courier", "QA-TRACK"
		}
		err := repository.UpdateSellerFulfillmentWithRevision(ctx, 901001, 901001, "STAGE-ORDER", string(stage), carrier, tracking, "", int64(index))
		if err != nil {
			t.Fatalf("first/new %s save failed: %v", stage, err)
		}
		// Duplicate network submissions retain their original revision.
		if err := repository.UpdateSellerFulfillmentWithRevision(ctx, 901001, 901001, "STAGE-ORDER", string(stage), carrier, tracking, "", int64(index)); err != nil {
			t.Fatalf("duplicate %s: %v", stage, err)
		}
		for _, route := range []string{"/admin/orders", "/super-admin/orders"} {
			r := projectionRequest(httptest.NewRequest("GET", route+"?stage="+string(stage), nil))
			orders := h.adminOrders(r)
			if len(orders) != 1 || orders[0].Fulfillment != string(stage) || orders[0].PaymentStatus != "captured" {
				t.Fatalf("%s did not reflect %s: %+v (%s)", route, stage, orders, projectionWarning(r, ""))
			}
			expectedOrder := "processing"
			if stage.Index() >= domaincommerce.FulfillmentShipped.Index() {
				expectedOrder = "shipped"
			}
			if stage == domaincommerce.FulfillmentDelivered {
				expectedOrder = "delivered"
			}
			if orders[0].Status != expectedOrder {
				t.Fatalf("financial lifecycle overwritten: %+v", orders[0])
			}
		}
		detail, err := repository.GetOrder(ctx, 901002, "STAGE-ORDER")
		if err != nil {
			t.Fatal(err)
		}
		if got := refreshOrderView(detail); got.Status != stage.Label() {
			t.Fatalf("customer stage=%s want=%s", got.Status, stage.Label())
		}
		wrong := projectionRequest(httptest.NewRequest("GET", "/admin/orders?stage=pending", nil))
		if len(h.adminOrders(wrong)) != 0 {
			t.Fatal("company filter included wrong stage")
		}
	}
	if err := repository.UpdateSellerFulfillmentWithRevision(ctx, 901001, 901001, "STAGE-ORDER", "processing", "", "", "", 0); !errors.Is(err, ports.ErrFulfillmentConflict) {
		t.Fatalf("stale save was accepted: %v", err)
	}
	var changes, shipping int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE action='seller.order_fulfillment_updated'`).Scan(&changes); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_events WHERE event_type='SellerOrderShipped'`).Scan(&shipping); err != nil {
		t.Fatal(err)
	}
	if changes != 5 || shipping != 1 {
		t.Fatalf("duplicate stage events: changes=%d shipping=%d", changes, shipping)
	}
}

func TestMixedSellerParcelViewsPreserveOrderLifecycle(t *testing.T) {
	pool := projectionTestPool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
 INSERT INTO users(id,email,display_name,status) VALUES(901001,'owner@example.invalid','Owner','active'),(901002,'customer@example.invalid','Customer','active'),(901003,'owner2@example.invalid','Second Owner','active');
 INSERT INTO sellers(id,owner_user_id,display_name,status) VALUES(901001,901001,'First Maker','active'),(901002,901003,'Second Maker','active');
 INSERT INTO categories(slug,name) VALUES('mixed','Mixed'); INSERT INTO brands(slug,name) VALUES('mixed','Mixed');
 INSERT INTO products(id,slug,brand_slug,category_slug,name,price_cents,seller_id,status) VALUES(901001,'mixed-first','mixed','mixed','First Product',1000,901001,'approved'),(901002,'mixed-second','mixed','mixed','Second Product',1000,901002,'approved');
 INSERT INTO product_variants(id,product_id,sku,price_cents) VALUES(901001,901001,'MIX-1',1000),(901002,901002,'MIX-2',1000);
 INSERT INTO orders(id,order_number,user_id,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key) VALUES(901001,'MIXED-ORDER',901002,'processing',2000,2000,'{}','mixed','1');
 INSERT INTO order_items(order_id,variant_id,product_name,sku,unit_price_cents,quantity,line_total_cents) VALUES(901001,901001,'First','MIX-1',1000,1,1000),(901001,901002,'Second','MIX-2',1000,1,1000);
 INSERT INTO payment_attempts(order_id,provider,status,amount_cents) VALUES(901001,'local','captured',2000);
 INSERT INTO seller_fulfillments(seller_id,order_id,status,carrier,tracking_number,revision) VALUES(901001,901001,'shipped','Courier','TRACK-1',3);
 `)
	if err != nil {
		t.Fatal(err)
	}
	h := NewLivePortalHandler(pool, nil, nil)
	repo := postgresadapter.NewCommerceRepository(pool)
	for _, stage := range []string{"shipped", "delivered"} {
		if _, err := pool.Exec(ctx, `UPDATE seller_fulfillments SET status=$1 WHERE order_id=901001`, stage); err != nil {
			t.Fatal(err)
		}
		orders := h.adminOrders(projectionRequest(httptest.NewRequest("GET", "/super-admin/orders?stage=processing", nil)))
		want := "partially_" + stage
		if len(orders) != 1 || orders[0].Fulfillment != want || orders[0].Status != "processing" || len(orders[0].Parcels) != 2 {
			t.Fatalf("mixed queue: %+v", orders)
		}
		detail, err := repo.GetOrder(ctx, 901002, "MIXED-ORDER")
		if err != nil {
			t.Fatal(err)
		}
		if got := refreshOrderView(detail); got.Status != displayOrderStatus(want) {
			t.Fatalf("mixed customer: %+v", got)
		}
	}
}

func TestOrderProgressCannotHideTerminalOrFailedPaymentState(t *testing.T) {
	for _, status := range []string{"cancelled", "refunded", "payment_failed", "pending_payment"} {
		detail := domaincommerce.OrderDetail{Order: domaincommerce.Order{Status: status}, PaymentStatus: "captured", Fulfillments: []domaincommerce.OrderFulfillment{{Status: "processing"}}}
		if got := refreshOrderView(detail); got.Status != displayOrderStatus(status) {
			t.Fatalf("%s hidden by fulfilment: %s", status, got.Status)
		}
	}
}
