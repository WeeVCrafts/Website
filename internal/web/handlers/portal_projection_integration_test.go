package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	redisclient "github.com/redis/go-redis/v9"
	postgresadapter "github.com/wecratfs/commerce/internal/adapters/postgres"
	redisadapter "github.com/wecratfs/commerce/internal/adapters/redis"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
)

func projectionTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("PROJECTION_DATABASE_URL")
	if dsn == "" {
		t.Skip("PROJECTION_DATABASE_URL required for isolated real PostgreSQL projection tests")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if config.ConnConfig.Database != "weevcrafts_projections_20261009" {
		t.Fatal("projection tests require their dedicated database")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("projection_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
	})
	if err := postgresadapter.ApplyMigrations(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestPortalProjectionsSearchBeforePaginationAgainstPostgres(t *testing.T) {
	pool := projectionTestPool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
INSERT INTO users(id,email,display_name,status) VALUES(900001,'projection@example.invalid','Real Customer Name','active');
INSERT INTO sellers(id,owner_user_id,display_name,status) VALUES(900001,900001,'Projection Seller','active');
INSERT INTO categories(slug,name) VALUES('projection','Projection');
INSERT INTO brands(slug,name) VALUES('projection','Projection');
INSERT INTO products(id,slug,brand_slug,category_slug,name,price_cents,seller_id,status) VALUES(900001,'projection','projection','projection','Projection Product',1000,900001,'approved');
INSERT INTO product_variants(id,product_id,sku,price_cents) VALUES(900001,900001,'PROJECTION-SKU',1000);
INSERT INTO orders(id,order_number,user_id,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key,created_at)
SELECT 900000+n,CASE WHEN n=1 THEN 'needle-order' ELSE 'projection-order-'||n END,900001,'paid',1000,1000,'{}','projection',n::text,NOW()-INTERVAL '226 days'+n*INTERVAL '1 day' FROM generate_series(1,225) n;
INSERT INTO order_items(order_id,variant_id,product_name,sku,unit_price_cents,quantity,line_total_cents)
SELECT id,900001,'Projection Product','PROJECTION-SKU',1000,1,1000 FROM orders;
INSERT INTO return_requests(order_id,user_id,reason,created_at)
SELECT id,900001,CASE WHEN order_number='needle-order' THEN 'needle-reason' ELSE 'Projection reason' END,created_at FROM orders ORDER BY id;
INSERT INTO payment_attempts(order_id,provider,status,amount_cents,created_at)
SELECT id,CASE WHEN order_number='needle-order' THEN 'needle-provider' ELSE 'projection-provider' END,CASE WHEN order_number='needle-order' THEN 'failed' ELSE 'captured' END,1000,created_at FROM orders ORDER BY id;
INSERT INTO support_tickets(ticket_number,user_id,order_id,subject,message,created_at)
SELECT 'projection-ticket-'||id,900001,id,CASE WHEN order_number='needle-order' THEN 'needle-subject' ELSE 'Projection subject' END,'Real customer message',created_at FROM orders ORDER BY id;
INSERT INTO seller_ledger(seller_id,order_id,entry_type,gross_cents,net_cents) SELECT 900001,id,'sale',1000,1000 FROM orders ORDER BY id;
INSERT INTO seller_settlements(seller_id,period_start,period_end,reference,net_cents)
SELECT 900001,CURRENT_DATE-1,CURRENT_DATE,CASE WHEN n=1 THEN 'needle-settlement' ELSE 'projection-settlement-'||n END,1000 FROM generate_series(1,225) n;
INSERT INTO support_ticket_messages(ticket_id,author_id,author_role,body) SELECT id,900001,'customer','Real message' FROM support_tickets ORDER BY id LIMIT 1;
`)
	if err != nil {
		t.Fatal(err)
	}
	h := NewLivePortalHandler(pool, nil, nil)
	r := projectionRequest(httptest.NewRequest("GET", "/admin/orders?q=needle-order", nil))
	if rows := h.adminOrders(r); len(rows) != 1 || rows[0].Number != "needle-order" {
		t.Fatalf("old admin order missing: %+v", rows)
	}
	if p := projectionPagination(r, "admin-orders"); p.Total != 1 {
		t.Fatalf("filtered total=%d", p.Total)
	}
	r = projectionRequest(httptest.NewRequest("GET", "/admin/returns?q=needle-reason", nil))
	if rows := h.adminReturns(r); len(rows) != 1 {
		t.Fatalf("old admin return missing: %+v", rows)
	}
	r = projectionRequest(httptest.NewRequest("GET", "/admin/failed-payments", nil))
	if rows := h.adminTransactions(r); len(rows) != 1 || rows[0].Status != "failed" {
		t.Fatalf("old failed payment missing: %+v", rows)
	}
	r = projectionRequest(httptest.NewRequest("GET", "/admin/support?q=needle-subject", nil))
	if rows := h.adminCases(r); len(rows) != 1 {
		t.Fatalf("old admin case missing: %+v", rows)
	}
	r = projectionRequest(httptest.NewRequest("GET", "/seller-admin/orders?q=needle-order", nil))
	if rows := h.sellerOrders(r, 900001); len(rows) != 1 {
		t.Fatalf("old seller order missing: %+v", rows)
	}
	r = projectionRequest(httptest.NewRequest("GET", "/seller-admin/returns?q=needle-reason", nil))
	if rows := h.sellerReturns(r, 900001); len(rows) != 1 {
		t.Fatalf("old seller return missing: %+v", rows)
	}
	r = projectionRequest(httptest.NewRequest("GET", "/seller-admin/earnings?q=needle-order", nil))
	if rows := h.sellerLedger(r, 900001); len(rows) != 1 {
		t.Fatalf("old ledger entry missing: %+v", rows)
	}
	r = projectionRequest(httptest.NewRequest("GET", "/seller-admin/earnings?q=needle-settlement", nil))
	if rows := h.sellerSettlements(r, 900001); len(rows) != 1 {
		t.Fatalf("old settlement missing: %+v", rows)
	}
	r = projectionRequest(httptest.NewRequest("GET", "/support-portal/cases?q=needle-subject", nil))
	cases := h.supportCases(r)
	if len(cases) != 1 {
		t.Fatalf("old support case missing: %+v", cases)
	}
	if messages := h.supportTicketMessages(r, cases[0].DatabaseID); len(messages) != 1 || messages[0].Author == "Real Customer Name" {
		t.Fatalf("customer author unmasked: %+v", messages)
	}
	r = projectionRequest(httptest.NewRequest("GET", "/admin/orders?page=2", nil))
	if rows := h.adminOrders(r); len(rows) != 50 {
		t.Fatalf("page2 rows=%d", len(rows))
	}
	if p := projectionPagination(r, "admin-orders"); p.Total != 225 || p.Page != 2 || p.Pages != 5 || p.NextURL == "" || p.PreviousURL == "" {
		t.Fatalf("pagination incorrect: %+v", p)
	}
	r = projectionRequest(httptest.NewRequest("GET", "/seller-admin/orders?page=5", nil))
	if rows := h.sellerOrders(r, 900001); len(rows) != 25 || rows[len(rows)-1].Number != "needle-order" {
		t.Fatalf("last seller order page inaccessible: %+v", rows)
	}
	if p := projectionPagination(r, "seller-orders"); p.Total != 225 || p.Page != 5 || p.Pages != 5 {
		t.Fatalf("seller orders pagination incorrect: %+v", p)
	}
	r = projectionRequest(httptest.NewRequest("GET", "/seller-admin/earnings?ledger_page=2&settlement_page=5&q=projection", nil))
	if ledger, settlements := h.sellerLedger(r, 900001), h.sellerSettlements(r, 900001); len(ledger) != 50 || len(settlements) != 24 {
		t.Fatalf("independent earnings pages: ledger=%d settlements=%d", len(ledger), len(settlements))
	}
	if p := projectionPagination(r, "seller-ledger"); p.Page != 2 || !strings.Contains(p.NextURL, "settlement_page=5") || !strings.Contains(p.NextURL, "q=projection") {
		t.Fatalf("ledger page failed to preserve other filters: %+v", p)
	}
	if p := projectionPagination(r, "seller-settlements"); p.Page != 5 || !strings.Contains(p.PreviousURL, "ledger_page=2") {
		t.Fatalf("settlement page failed to preserve ledger page: %+v", p)
	}
	if projectionWarning(r, "") != "" {
		t.Fatal(projectionWarning(r, ""))
	}
	// Exercise the remaining searchable projections against real PostgreSQL,
	// including empty queues, so SQL errors cannot masquerade as empty data.
	for name, load := range map[string]func(*http.Request){
		"admin-products": func(r *http.Request) {
			if len(h.adminProducts(r, "Projection")) != 1 {
				t.Error("persisted admin product missing")
			}
		},
		"admin-inventory": func(r *http.Request) {
			if len(h.adminInventory(r)) != 1 {
				t.Error("persisted admin inventory missing")
			}
		},
		"admin-sellers": func(r *http.Request) {
			if len(h.adminSellers(r)) != 1 {
				t.Error("persisted seller missing")
			}
		},
		"admin-reviews":   func(r *http.Request) { h.adminReviews(r) },
		"admin-campaigns": func(r *http.Request) { h.adminCampaigns(r) },
		"admin-audit":     func(r *http.Request) { h.adminAudit(r) },
		"seller-products": func(r *http.Request) {
			if len(h.sellerProducts(r, 900001, "Projection")) != 1 {
				t.Error("persisted seller product missing")
			}
		},
		"seller-inventory": func(r *http.Request) {
			if len(h.sellerInventory(r, 900001, "Projection", 5)) != 1 {
				t.Error("persisted seller inventory missing")
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := projectionRequest(httptest.NewRequest("GET", "/?q=Projection", nil))
			load(r)
			if warning := projectionWarning(r, ""); warning != "" {
				t.Fatal(warning)
			}
			if p := projectionPagination(r, name); p.Page != 1 || p.PageSize != 50 {
				t.Fatalf("missing pagination: %+v", p)
			}
		})
	}
}

func TestPortalProjectionFailureReturns503AgainstPostgres(t *testing.T) {
	pool := projectionTestPool(t)
	pool.Close()
	h := NewLivePortalHandler(pool, nil, nil)
	r := httptest.NewRequest("GET", "/admin/orders", nil)
	w := httptest.NewRecorder()
	h.renderAdmin(w, r, 900001, domainidentity.RoleSuperAdmin, "/super-admin", "Projection Admin", "csrf")
	if w.Code != 503 || !strings.Contains(w.Body.String(), "could not be loaded") {
		t.Fatalf("failed projection returned %d without warning", w.Code)
	}
	if strings.Contains(w.Body.String(), ">0</") {
		t.Fatal("failed projection rendered fabricated zero stats")
	}
}

func TestPortalAuthorizationOutageReturns503ThroughHTTP(t *testing.T) {
	pool := projectionTestPool(t)
	if os.Getenv("REDIS_ADDR") == "" {
		t.Skip("REDIS_ADDR required for real persisted HTTP sessions")
	}
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,email,display_name,status) VALUES(900001,'projection-auth@example.invalid','Projection Admin','active'); INSERT INTO user_roles(user_id,role_slug) VALUES(900001,'super_admin')`); err != nil {
		t.Fatal(err)
	}
	client := redisclient.NewClient(&redisclient.Options{Addr: os.Getenv("REDIS_ADDR")})
	t.Cleanup(func() { _ = client.Close() })
	sessions := applicationauth.NewSessionService(redisadapter.NewSessionStore(client))
	session, err := sessions.Start(ctx, domainidentity.User{ID: 900001}, "projection-test", "127.0.0.1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sessions.Revoke(context.Background(), session.ID) })
	h := NewLivePortalHandler(pool, sessions, nil)
	healthy := projectionRequest(httptest.NewRequest(http.MethodGet, "/super-admin/orders", nil))
	if !h.hasRole(healthy, 900001, "super_admin") {
		t.Fatal("persisted admin role was not authorized before outage")
	}
	// A healthy database distinguishes a genuine permission denial from outage.
	healthy.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: session.ID})
	denied := httptest.NewRecorder()
	h.ServeHTTP(denied, requestWithPath(healthy, "/seller-admin/orders"))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("super admin was not denied seller-only access: %d", denied.Code)
	}
	pool.Close()
	for _, path := range []string{"/super-admin/orders", "/admin/orders", "/seller-admin/orders", "/seller/orders", "/support-portal/cases", "/support/cases", "/super-admin/brands", "/seller-admin/support"} {
		t.Run(path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, path, nil)
			r.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: session.ID})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusServiceUnavailable || w.Body.Len() == 0 || strings.Contains(w.Body.String(), "forbidden") {
				t.Fatalf("authorization outage returned %d: %s", w.Code, w.Body.String())
			}
		})
	}
}
