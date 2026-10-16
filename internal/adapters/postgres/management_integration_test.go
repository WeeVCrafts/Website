package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestManagementCRUDSearchAuthorizationAuditAndCommissionLedger(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	fixture := newCommerceFixture(t, pool, 1, 3)
	suffix := fmt.Sprint(time.Now().UnixNano())
	var actorID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status) VALUES($1,'Management administrator','active') RETURNING id`, "management-"+suffix+"@example.invalid").Scan(&actorID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'super_admin'),($2,'customer')`, actorID, fixture.userIDs[0]); err != nil {
		t.Fatal(err)
	}
	repository := NewManagementRepository(pool)
	service := applicationcommerce.NewManagementService(repository)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM seller_ledger WHERE order_id IN(SELECT id FROM orders WHERE user_id=$1)`, fixture.userIDs[0])
		_, _ = pool.Exec(ctx, `DELETE FROM commission_rules WHERE created_by=$1`, actorID)
		_, _ = pool.Exec(ctx, `DELETE FROM brands WHERE slug LIKE $1`, "management-"+suffix+"-%")
		_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE actor_id=$1`, fmt.Sprint(actorID))
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mutate := func(m ports.ManagementMutation) {
		t.Helper()
		m.Reason = "Integration verification"
		if err := service.Mutate(ctx, actorID, m); err != nil {
			t.Fatalf("%s %s: %v", m.Section, m.Action, err)
		}
	}
	if _, err := service.List(ctx, fixture.userIDs[0], "customers", "", 1); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("customer read should be forbidden: %v", err)
	}
	if err := service.Mutate(ctx, fixture.userIDs[0], ports.ManagementMutation{Section: "brands", Action: "create", ID: "forbidden", Name: "Forbidden", Reason: "Unauthorized attempt"}); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("customer mutation should be forbidden: %v", err)
	}
	for i := 0; i < 31; i++ {
		mutate(ports.ManagementMutation{Section: "brands", Action: "create", ID: fmt.Sprintf("management-%s-%02d", suffix, i), Name: fmt.Sprintf("Management %s %02d", suffix, i)})
	}
	result, err := service.List(ctx, actorID, "brands", suffix, 1)
	if err != nil || len(result.Records) != 25 || result.Total != 31 {
		t.Fatalf("pagination page 1: %+v %v", result, err)
	}
	result, err = service.List(ctx, actorID, "brands", suffix, 2)
	if err != nil || len(result.Records) != 6 {
		t.Fatalf("pagination page 2: %+v %v", result, err)
	}
	result, err = service.List(ctx, actorID, "brands", fmt.Sprintf("management-%s-30", suffix), 1)
	if err != nil || len(result.Records) != 1 {
		t.Fatalf("search must apply before limit: %+v %v", result, err)
	}
	slug := result.Records[0].ID
	mutate(ports.ManagementMutation{Section: "brands", Action: "update", ID: slug, Name: "Renamed brand"})
	var name string
	if err := pool.QueryRow(ctx, `SELECT name FROM brands WHERE slug=$1`, slug).Scan(&name); err != nil || name != "Renamed brand" {
		t.Fatalf("slug-preserving update: %s %v", name, err)
	}
	mutate(ports.ManagementMutation{Section: "brands", Action: "delete", ID: slug})
	if err := service.Mutate(ctx, actorID, ports.ManagementMutation{Section: "brands", Action: "delete", ID: fixture.brandSlug, Reason: "Reject in-use brand"}); !errors.Is(err, ports.ErrManagementConflict) {
		t.Fatalf("in-use brand deleted: %v", err)
	}
	if e:=service.Mutate(ctx,actorID,ports.ManagementMutation{Section:"category-management",Action:"update",ID:fixture.category,Name:"Renamed category",Reason:"Requires publication"});!errors.Is(e,ports.ErrForbidden){t.Fatalf("taxonomy bypass: %v",e)}
	if err := service.Mutate(ctx, actorID, ports.ManagementMutation{Section: "category-management", Action: "delete", ID: fixture.category, Reason: "Reject in-use category"}); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("legacy category bypass: %v", err)
	}
	var customerEmail string
	if err := pool.QueryRow(ctx, `SELECT email FROM users WHERE id=$1`, fixture.userIDs[0]).Scan(&customerEmail); err != nil {
		t.Fatal(err)
	}
	result, err = service.List(ctx, actorID, "customers", customerEmail, 1)
	if err != nil || len(result.Records) != 1 || result.Records[0].Orders != 0 {
		t.Fatalf("customer directory: %+v %v", result, err)
	}
	ruleName := "Integration commission " + suffix
	mutate(ports.ManagementMutation{Section: "commission-rules", Action: "create", Name: ruleName, Category: fixture.category, BasisPoints: 750, Active: true})
	var ruleID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM commission_rules WHERE name=$1`, ruleName).Scan(&ruleID); err != nil {
		t.Fatal(err)
	}
	createLedger := func(index int) int64 {
		t.Helper()
		var orderID int64
		if err := pool.QueryRow(ctx, `INSERT INTO orders(order_number,user_id,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key) VALUES($1,$2,'paid',10000,10000,'{}','management',$1) RETURNING id`, fmt.Sprintf("MGMT-%s-%d", suffix, index), fixture.userIDs[0]).Scan(&orderID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO order_items(order_id,variant_id,product_name,sku,unit_price_cents,quantity,line_total_cents) VALUES($1,$2,'Fixture','FIXTURE',10000,1,10000)`, orderID, fixture.variantID); err != nil {
			t.Fatal(err)
		}
		if err := WithinTransaction(ctx, pool, func(ctx context.Context, tx pgx.Tx) error { return recordSellerSalesLedger(ctx, tx, orderID) }); err != nil {
			t.Fatal(err)
		}
		return orderID
	}
	first := createLedger(1)
	var commission int64
	if err := pool.QueryRow(ctx, `SELECT commission_cents FROM seller_ledger WHERE order_id=$1 AND entry_type='sale'`, first).Scan(&commission); err != nil || commission != 750 {
		t.Fatalf("configured commission captured: %d %v", commission, err)
	}
	mutate(ports.ManagementMutation{Section: "commission-rules", Action: "update", ID: strconv.FormatInt(ruleID, 10), Name: ruleName, Category: fixture.category, BasisPoints: 950, Active: true})
	second := createLedger(2)
	if err := pool.QueryRow(ctx, `SELECT commission_cents FROM seller_ledger WHERE order_id=$1 AND entry_type='sale'`, second).Scan(&commission); err != nil || commission != 950 {
		t.Fatalf("updated commission captured: %d %v", commission, err)
	}
	if err := pool.QueryRow(ctx, `SELECT commission_cents FROM seller_ledger WHERE order_id=$1 AND entry_type='sale'`, first).Scan(&commission); err != nil || commission != 750 {
		t.Fatalf("historic ledger changed: %d %v", commission, err)
	}
	mutate(ports.ManagementMutation{Section: "commission-rules", Action: "delete", ID: strconv.FormatInt(ruleID, 10)})

	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE actor_id=$1 AND action LIKE 'management.%'`, fmt.Sprint(actorID)).Scan(&auditCount); err != nil || auditCount != 36 {
		t.Fatalf("successful mutations audited atomically: %d %v", auditCount, err)
	}
}

func TestLegacyManagementSettingsRequirePublication(t *testing.T) {
 pool:=openIntegrationPool(t);ctx:=context.Background();actor:=roleFixture(t,pool,"super_admin");before,e:=ReadStorefrontSettings(ctx,pool);if e!=nil{t.Fatal(e)}
 service:=applicationcommerce.NewManagementService(NewManagementRepository(pool))
 e=service.Mutate(ctx,actor,ports.ManagementMutation{Section:"settings",Action:"save",Reason:"Reject legacy settings bypass",Settings:ports.StorefrontSettings{SupportEmail:"draft@example.invalid",ShippingInformation:"This must not publish immediately."}})
 if !errors.Is(e,ports.ErrForbidden){t.Fatalf("legacy settings bypass: %v",e)};after,e:=ReadStorefrontSettings(ctx,pool);if e!=nil||before!=after{t.Fatalf("legacy save changed live settings: %+v %v",after,e)}
}
