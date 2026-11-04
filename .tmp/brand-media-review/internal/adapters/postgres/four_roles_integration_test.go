package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	application "github.com/wecratfs/commerce/internal/application/commerce"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	identity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
	"io/fs"
	"sync"
	"testing"
	"time"
)

func isolatedRolePool(t *testing.T, legacy bool) *pgxpool.Pool {
	t.Helper()
	base := openIntegrationPool(t)
	ctx := context.Background()
	schema := fmt.Sprintf("roles_qa_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE") })
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if !legacy {
		if err = ApplyMigrations(ctx, pool); err != nil {
			t.Fatal(err)
		}
		return pool
	}
	if _, err = pool.Exec(ctx, `CREATE TABLE schema_migrations(version BIGINT PRIMARY KEY,applied_at TIMESTAMPTZ DEFAULT NOW())`); err != nil {
		t.Fatal(err)
	}
	entries, _ := fs.ReadDir(migrationFiles, "migrations")
	for _, entry := range entries {
		version, _ := migrationVersion(entry.Name())
		if version > 41 {
			continue
		}
		data, _ := migrationFiles.ReadFile("migrations/" + entry.Name())
		if _, err = pool.Exec(ctx, string(data)); err != nil {
			t.Fatal(entry.Name(), err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); err != nil {
			t.Fatal(err)
		}
	}
	return pool
}

func roleFixture(t *testing.T, pool *pgxpool.Pool, role string) int64 {
	t.Helper()
	var id int64
	ctx := context.Background()
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status,email_verified_at) VALUES($1,$2,'active',NOW()) RETURNING id`, fmt.Sprintf("%s-%d@test.invalid", role, time.Now().UnixNano()), role).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,$2)`, id, role); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestFourRoleUpgradePreservesAccountsAndHistoricalAuthorship(t *testing.T) {
	pool := isolatedRolePool(t, true)
	ctx := context.Background()
	mapping := map[string]string{"customer": "customer", "seller_owner": "seller", "marketplace_admin": "admin", "support_agent": "admin", "finance_operator": "admin", "operations_returns": "admin", "super_admin": "super_admin"}
	ids := map[string]int64{}
	for old := range mapping {
		ids[old] = roleFixture(t, pool, old)
	}
	var ticket int64
	if err := pool.QueryRow(ctx, `INSERT INTO support_tickets(ticket_number,user_id,subject,message) VALUES('MIGRATION-CASE',$1,'Preserved case','Original message') RETURNING id`, ids["customer"]).Scan(&ticket); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO support_ticket_messages(ticket_id,author_id,author_role,body) VALUES($1,$2,'support_agent','Historical reply')`, ticket, ids["support_agent"]); err != nil {
		t.Fatal(err)
	}
	if err := ApplyMigrations(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for old, want := range mapping {
		var got string
		if err := pool.QueryRow(ctx, `SELECT role_slug FROM user_roles WHERE user_id=$1`, ids[old]).Scan(&got); err != nil || got != want {
			t.Fatal(old, got, err)
		}
	}
	var users, roles int
	var author string
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM users),(SELECT count(*) FROM roles),(SELECT author_role FROM support_ticket_messages WHERE ticket_id=$1)`, ticket).Scan(&users, &roles, &author); err != nil || users != 7 || roles != 4 || author != "support_agent" {
		t.Fatal(users, roles, author, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'admin')`, ids["customer"]); err == nil {
		t.Fatal("multiple assignments accepted")
	}
	if _, err := pool.Exec(ctx, `DELETE FROM user_roles WHERE user_id=$1`, ids["customer"]); err == nil {
		t.Fatal("roleless assignment accepted")
	}
}

func TestFourRoleUpgradeRejectsStaffAndAmbiguousAssignments(t *testing.T) {
	for _, mode := range []string{"staff", "conflict", "missing"} {
		t.Run(mode, func(t *testing.T) {
			pool := isolatedRolePool(t, true)
			ctx := context.Background()
			role := "customer"
			if mode == "staff" {
				role = "seller_staff"
			}
			id := roleFixture(t, pool, role)
			if mode == "conflict" {
				_, _ = pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'finance_operator')`, id)
			}
			if mode == "missing" {
				_, _ = pool.Exec(ctx, `DELETE FROM user_roles WHERE user_id=$1`, id)
			}
			if err := ApplyMigrations(ctx, pool); err == nil {
				t.Fatal("unsafe migration accepted")
			}
			var version int
			if err := pool.QueryRow(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil || version != 41 {
				t.Fatal("failed migration changed schema", version, err)
			}
		})
	}
}

func TestLastSuperAdminConcurrentProtectionAndPromotion(t *testing.T) {
	pool := isolatedRolePool(t, false)
	ctx := context.Background()
	first := roleFixture(t, pool, "super_admin")
	second := roleFixture(t, pool, "customer")
	repo := NewRoleAdministrationRepository(pool)
	if err := repo.UpdateRoleAssignment(ctx, first, first, 0, identity.RoleCustomer, true, "Demote final admin"); !errors.Is(err, ports.ErrRoleState) {
		t.Fatal(err)
	}
	if _, err := repo.UpdateRoleUser(ctx, first, first, "last@test.invalid", "Last Admin", "suspended", "", "Suspend final admin"); !errors.Is(err, ports.ErrRoleState) {
		t.Fatal(err)
	}
	if err := repo.DeleteRoleUser(ctx, first, first, "Delete final admin"); !errors.Is(err, ports.ErrRoleState) {
		t.Fatal(err)
	}
	if err := repo.UpdateRoleAssignment(ctx, first, second, 0, identity.RoleSuperAdmin, true, "Promote second admin"); err != nil {
		t.Fatal(err)
	}
	var revoked *time.Time
	if err := pool.QueryRow(ctx, `SELECT sessions_revoked_at FROM users WHERE id=$1`, second).Scan(&revoked); err != nil || revoked == nil {
		t.Fatal("promotion did not invalidate sessions", err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, pair := range [][2]int64{{first, first}, {second, second}} {
		wg.Add(1)
		go func(actor, target int64) {
			defer wg.Done()
			results <- repo.UpdateRoleAssignment(ctx, actor, target, 0, identity.RoleCustomer, true, "Concurrent demotion")
		}(pair[0], pair[1])
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, ports.ErrRoleState) && !errors.Is(err, ports.ErrForbidden) {
			t.Fatal(err)
		}
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.status='active' AND ur.role_slug='super_admin'`).Scan(&remaining); err != nil || remaining != 1 || success != 1 {
		t.Fatal(remaining, success, err)
	}
}

func TestSellerDemotionBlocksOrdersAndReturnsAndPreservesCatalogue(t *testing.T) {
	pool := isolatedRolePool(t, false)
	ctx := context.Background()
	fixture := newCommerceFixture(t, pool, 1, 5)
	admin := roleFixture(t, pool, "super_admin")
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'seller'),($2,'customer')`, fixture.ownerID, fixture.userIDs[0]); err != nil {
		t.Fatal(err)
	}
	commerce := NewCommerceRepository(pool)
	order, err := commerce.CreateOrder(ctx, fixture.userIDs[0], fixture.cartIDs[0], "demotion-fixture", domain.AddressInput{RecipientName: "Fixture", Phone: "9876543210", Line1: "1 Test Lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRoleAdministrationRepository(pool)
	if err = repo.UpdateRoleAssignment(ctx, admin, fixture.ownerID, 0, identity.RoleCustomer, true, "Seller transition"); !errors.Is(err, ports.ErrSellerPendingWork) {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE orders SET status='delivered' WHERE id=$1;`, order.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO return_requests(order_id,user_id,reason,status) VALUES($1,$2,'Return fixture','pending')`, order.ID, fixture.userIDs[0]); err != nil {
		t.Fatal(err)
	}
	if err = repo.UpdateRoleAssignment(ctx, admin, fixture.ownerID, 0, identity.RoleCustomer, true, "Seller transition"); !errors.Is(err, ports.ErrSellerPendingWork) {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE return_requests SET status='rejected' WHERE order_id=$1`, order.ID); err != nil {
		t.Fatal(err)
	}
	if err = repo.UpdateRoleAssignment(ctx, admin, fixture.ownerID, 0, identity.RoleCustomer, true, "Completed seller transition"); err != nil {
		t.Fatal(err)
	}
	var status string
	var products, orders int
	if err = pool.QueryRow(ctx, `SELECT s.status,(SELECT COUNT(*) FROM products WHERE id=$2),(SELECT COUNT(*) FROM orders WHERE id=$3) FROM sellers s WHERE owner_user_id=$1`, fixture.ownerID, fixture.productID, order.ID).Scan(&status, &products, &orders); err != nil || status != "suspended" || products != 1 || orders != 1 {
		t.Fatal(status, products, orders, err)
	}
	if err = repo.UpdateRoleAssignment(ctx, admin, fixture.ownerID, 0, identity.RoleSeller, true, "Restore seller role"); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT status FROM sellers WHERE owner_user_id=$1`, fixture.ownerID).Scan(&status); err != nil || status != "suspended" {
		t.Fatal("role bypassed suspension", status, err)
	}
}

func TestPrivateSellerConfirmationsScopeIdempotencyAndLifecycle(t *testing.T) {
	pool := isolatedRolePool(t, false)
	ctx := context.Background()
	one := newCommerceFixture(t, pool, 1, 4)
	two := newCommerceFixture(t, pool, 1, 4)
	admin := roleFixture(t, pool, "admin")
	super := roleFixture(t, pool, "super_admin")
	customer := one.userIDs[0]
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'seller'),($2,'seller'),($3,'customer')`, one.ownerID, two.ownerID, customer); err != nil {
		t.Fatal(err)
	}
	var sellerOne, sellerTwo int64
	_ = pool.QueryRow(ctx, `SELECT id FROM sellers WHERE owner_user_id=$1`, one.ownerID).Scan(&sellerOne)
	_ = pool.QueryRow(ctx, `SELECT id FROM sellers WHERE owner_user_id=$1`, two.ownerID).Scan(&sellerTwo)
	if _, err := NewCommerceRepository(pool).GetAdminProduct(ctx, one.ownerID, two.productID); !errors.Is(err, ports.ErrForbidden) {
		t.Fatal("seller acquired company catalogue scope", err)
	}
	if _, err := NewMediaRepository(pool).PrepareUpload(ctx, one.ownerID, two.productID, "foreign-media", "image/png", 20); !errors.Is(err, ports.ErrForbidden) {
		t.Fatal("foreign media access allowed", err)
	}
	repo := NewCommerceRepository(pool)
	order, err := repo.CreateOrder(ctx, customer, one.cartIDs[0], "private-confirmation-order", domain.AddressInput{RecipientName: "Fixture", Phone: "9876543210", Line1: "1 Test Lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := repo.CreateSupportTicket(ctx, customer, order.OrderNumber, "Delivery question", "Please confirm the dispatch details.")
	if err != nil {
		t.Fatal(err)
	}
	// Multi-seller order: each request must expose only its own lines.
	if _, err = pool.Exec(ctx, `INSERT INTO order_items(order_id,variant_id,product_name,sku,unit_price_cents,quantity,line_total_cents) VALUES($1,$2,'Other seller private item','OTHER-PRIVATE',1000,1,1000)`, order.ID, two.variantID); err != nil {
		t.Fatal(err)
	}
	service := application.NewSellerConfirmationService(repo, NewRoleAuthorizer(pool))
	key := "confirmation-key-0001"
	if _, err = service.Create(ctx, admin, ticket.ID, 999999999, "Foreign seller request", key); !errors.Is(err, ports.ErrSupportNotFound) {
		t.Fatal("foreign seller attached", err)
	}
	id, err := service.Create(ctx, admin, ticket.ID, sellerOne, "Please confirm dispatch", key)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Close(ctx, admin, id, "Premature outcome before seller confirmation"); !errors.Is(err, ports.ErrSupportState) {
		t.Fatal("open request closed before reply", err)
	}
	again, err := service.Create(ctx, admin, ticket.ID, sellerOne, "Please confirm dispatch", key)
	if err != nil || again != id {
		t.Fatal("duplicate create", again, err)
	}
	if _, err = service.Create(ctx, admin, ticket.ID, sellerOne, "Changed message", key); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	secondID, secondErr := service.Create(ctx, admin, ticket.ID, sellerTwo, "Confirm other seller dispatch", "confirmation-key-0002")
	if secondErr != nil {
		t.Fatal(secondErr)
	}
	secondList, secondErr := service.List(ctx, two.ownerID, 0, true)
	if secondErr != nil || len(secondList) != 1 || secondList[0].ID != secondID || len(secondList[0].Items) != 1 || secondList[0].Items[0] != "Other seller private item × 1" {
		t.Fatal("multi-seller items leaked", secondList, secondErr)
	}
	if _, err = service.List(ctx, customer, 0, false); !errors.Is(err, ports.ErrForbidden) {
		t.Fatal("customer saw coordination", err)
	}
	if err = service.Reply(ctx, two.ownerID, id, "Foreign seller reply", "seller-reply-key-0001"); !errors.Is(err, ports.ErrForbidden) {
		t.Fatal(err)
	}
	if list, err := service.List(ctx, two.ownerID, 0, true); err != nil || (len(list) != 1 || list[0].ID == id) {
		t.Fatal("foreign conversation visible", list, err)
	}
	if err = service.Reply(ctx, one.ownerID, id, "Dispatch planned tomorrow", "seller-reply-key-0001"); err != nil {
		t.Fatal(err)
	}
	if err = service.Reply(ctx, one.ownerID, id, "Dispatch planned tomorrow", "seller-reply-key-0001"); err != nil {
		t.Fatal("duplicate reply", err)
	}
	all, err := service.List(ctx, super, ticket.ID, false)
	list := []domain.SellerConfirmation{}
	for _, item := range all {
		if item.ID == id {
			list = append(list, item)
		}
	}
	if err != nil || len(list) != 1 || list[0].Status != "answered" || len(list[0].Messages) != 1 || len(list[0].Items) != 1 {
		t.Fatal(list, err)
	}
	customerTickets, err := repo.ListCustomerSupportTickets(ctx, customer)
	if err != nil || len(customerTickets) != 1 || customerTickets[0].LatestAgentReply != "" {
		t.Fatal("private reply entered customer thread", customerTickets, err)
	}
	if err = service.Close(ctx, admin, id, "Seller confirmed scheduled dispatch"); err != nil {
		t.Fatal(err)
	}
	if err = service.Reply(ctx, one.ownerID, id, "After closing", "seller-reply-key-0002"); !errors.Is(err, ports.ErrSupportState) {
		t.Fatal(err)
	}
	var orderStatus string
	if err = pool.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1`, order.ID).Scan(&orderStatus); err != nil || orderStatus != "pending_payment" {
		t.Fatal("coordination fabricated delivery", orderStatus, err)
	}
}
