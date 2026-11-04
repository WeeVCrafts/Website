package handlers

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestInsightRangeAndCSVRecovery(t *testing.T) {
	for _, test := range []struct {
		rangeValue string
		days       int
	}{{"7", 7}, {"30", 30}, {"90", 90}, {"-7", 30}, {"invalid", 30}, {"", 30}} {
		r := httptest.NewRequest("GET", "/super-admin/users?range="+test.rangeValue, nil)
		if got := insightDays(r); got != test.days {
			t.Fatalf("range %q: got %d", test.rangeValue, got)
		}
	}
	for _, value := range []string{"=SUM(A1:A2)", " +12", "-7", "@name", "\tformula", "\x00=1"} {
		if result := csvSafe(value); result == "" || result[0] != '\'' {
			t.Errorf("CSV formula was not escaped: %q -> %q", value, result)
		}
	}
	if got := csvSafe("Ordinary name"); got != "Ordinary name" {
		t.Fatal(got)
	}
	h := NewLivePortalHandler(nil, nil, nil)
	r := projectionRequest(httptest.NewRequest("GET", "/super-admin/users", nil))
	p := viewmodels.AdminPage{Active: "security", Workspace: "users"}
	h.superAdminInsights(r, &p)
	if p.Insights.Available || len(p.Stats) > 0 || projectionWarning(r, "") == "" {
		t.Fatal("failed database must not be presented as a zero-count success")
	}
}

func TestSuperAdminInsightsUseWholeFilteredDataset(t *testing.T) {
	pool := projectionTestPool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
INSERT INTO users(id,email,display_name,status,created_at)
SELECT 980000+n,'insight-'||n||'@example.invalid','Insight account '||n,'active',NOW()-CASE WHEN n=1 THEN INTERVAL '45 days' ELSE INTERVAL '1 day' END FROM generate_series(1,66) n;
INSERT INTO user_roles(user_id,role_slug) SELECT id,'customer' FROM users WHERE id BETWEEN 980001 AND 980066;
INSERT INTO sellers(id,owner_user_id,display_name,status) VALUES(980001,980001,'Insight seller','active');
INSERT INTO categories(slug,name) VALUES('insight','Insight');
INSERT INTO brands(slug,name) VALUES('insight','Insight');
INSERT INTO products(id,slug,brand_slug,category_slug,name,price_cents,seller_id,status) VALUES(980001,'insight','insight','insight','Insight product',1000,980001,'approved');
INSERT INTO product_variants(id,product_id,sku,price_cents) VALUES(980001,980001,'INSIGHT-SKU',1000);
INSERT INTO inventory_stock(variant_id,available_quantity,reserved_quantity) VALUES(980001,3,2);
INSERT INTO orders(id,order_number,user_id,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key) VALUES(980001,'INSIGHT-ORDER',980002,'paid',5000,5000,'{}','insight','one');
INSERT INTO payment_attempts(order_id,provider,status,amount_cents) VALUES(980001,'insight','captured',5000),(980001,'insight','refunded',2000);
INSERT INTO return_requests(order_id,user_id,reason) VALUES(980001,980002,'Insight return');
INSERT INTO support_tickets(ticket_number,user_id,order_id,subject,message) VALUES('INSIGHT-CASE',980002,980001,'Insight delivery question','Isolated test');
INSERT INTO seller_settlements(seller_id,period_start,period_end,reference,net_cents) VALUES(980001,CURRENT_DATE-1,CURRENT_DATE,'INSIGHT-SETTLEMENT',5000);
`)
	if err != nil {
		t.Fatal(err)
	}
	h := NewLivePortalHandler(pool, nil, nil)
	for _, page := range []int{1, 2} {
		r := withSuperAdminContext(projectionRequest(httptest.NewRequest("GET", "/super-admin/users?q=Insight&role=customer&range=7&page="+string(rune('0'+page)), nil)))
		p := viewmodels.AdminPage{Active: "security", Workspace: "users", Query: "Insight", RoleFilter: "customer"}
		h.superAdminInsights(r, &p)
		if !p.Insights.Available || len(p.Stats) != 4 || p.Stats[0].Value != "66" {
			t.Fatalf("whole filtered totals on page %d: %+v, warning %s", page, p, projectionWarning(r, ""))
		}
		var registrations int64
		for _, point := range p.Insights.Trend {
			registrations += point.Value
		}
		if len(p.Insights.Trend) != 7 || registrations != 65 || len(p.Insights.Distribution) != 1 || p.Insights.Distribution[0].Value != 66 {
			t.Fatalf("incorrect date/role aggregation: %+v", p.Insights)
		}
		rows := h.adminUsers(r, "Insight")
		if page == 2 && len(rows) != 16 {
			t.Fatalf("paged account rows: %d", len(rows))
		}
	}
	for _, active := range []string{"sellers", "products", "inventory", "returns", "support", "finance"} {
		r := withSuperAdminContext(projectionRequest(httptest.NewRequest("GET", "/super-admin/"+active+"?q=Insight&range=7", nil)))
		p := viewmodels.AdminPage{Active: active, Workspace: active, Query: "Insight"}
		h.superAdminInsights(r, &p)
		if !p.Insights.Available || len(p.Stats) != 4 {
			t.Fatalf("%s aggregates failed: %+v; %s", active, p, projectionWarning(r, ""))
		}
		if active == "inventory" && (p.Stats[0].Value != "1" || p.Stats[2].Value != "2") {
			t.Fatalf("inventory aggregate: %+v", p.Stats)
		}
		if active == "finance" {
			var total int64
			for _, point := range p.Insights.Trend {
				total += point.Value
			}
			if total != 5000 || p.Stats[2].Value != "1" {
				t.Fatalf("finance aggregate: %+v", p)
			}
		}
	}
}
