package postgres

import (
	"context"
	"fmt"
	"strings"
	"testing"

	application "github.com/wecratfs/commerce/internal/application/commerce"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
)

func TestSupportNotificationsRecipientsPrivacyAndCoordination(t *testing.T) {
	pool := isolatedRolePool(t, false)
	ctx := context.Background()
	fixture := newCommerceFixture(t, pool, 1, 4)
	admin := roleFixture(t, pool, "admin")
	super := roleFixture(t, pool, "super_admin")
	other := roleFixture(t, pool, "seller")
	customer := fixture.userIDs[0]
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'seller'),($2,'customer')`, fixture.ownerID, customer); err != nil {
		t.Fatal(err)
	}
	repo := NewCommerceRepository(pool)
	order, err := repo.CreateOrder(ctx, customer, fixture.cartIDs[0], "notification-order", domain.AddressInput{RecipientName: "Fixture", Phone: "9876543210", Line1: "1 Test Lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := repo.CreateSupportTicket(ctx, customer, order.OrderNumber, "Delivery question", "Private original text")
	if err != nil {
		t.Fatal(err)
	}
	check := func(user int64, key, link string, want int) {
		t.Helper()
		var count int
		var body, actual string
		if err := pool.QueryRow(ctx, `SELECT count(*),COALESCE(MAX(body),''),COALESCE(MAX(link_url),'') FROM notifications WHERE user_id=$1 AND source_key=$2`, user, key).Scan(&count, &body, &actual); err != nil {
			t.Fatal(err)
		}
		if count != want || want > 0 && actual != link {
			t.Fatalf("recipient %d: count=%d link=%q want %d %q", user, count, actual, want, link)
		}
		if strings.Contains(body, "Private") {
			t.Fatal("private message copied into notification")
		}
	}
	check(admin, fmt.Sprint("case:", ticket.ID), fmt.Sprintf("/admin/support?ticket_id=%d", ticket.ID), 1)
	check(super, fmt.Sprint("case:", ticket.ID), fmt.Sprintf("/super-admin/support?ticket_id=%d", ticket.ID), 1)
	message, err := repo.ReplySupportTicketWithID(ctx, super, ticket.ID, "Private admin response")
	if err != nil {
		t.Fatal(err)
	}
	check(customer, fmt.Sprint("reply:", message), fmt.Sprintf("/account/support?ticket_id=%d#ticket-%d", ticket.ID, ticket.ID), 1)
	check(other, fmt.Sprint("reply:", message), "", 0)
	sellerTicket, err := repo.CreateSupportTicket(ctx, fixture.ownerID, "", "Seller needs help", "Private seller text")
	if err != nil {
		t.Fatal(err)
	}
	message, err = repo.ReplySupportTicketWithID(ctx, admin, sellerTicket.ID, "Private company response")
	if err != nil {
		t.Fatal(err)
	}
	check(fixture.ownerID, fmt.Sprint("reply:", message), fmt.Sprintf("/seller-admin/support?ticket_id=%d#ticket-%d", sellerTicket.ID, sellerTicket.ID), 1)
	var sellerID int64
	_ = pool.QueryRow(ctx, `SELECT id FROM sellers WHERE owner_user_id=$1`, fixture.ownerID).Scan(&sellerID)
	service := application.NewSellerConfirmationService(repo, NewRoleAuthorizer(pool))
	request, err := service.Create(ctx, super, ticket.ID, sellerID, "Private dispatch question", "notification-request-01")
	if err != nil {
		t.Fatal(err)
	}
	check(fixture.ownerID, fmt.Sprintf("coordination:%d:open", request), fmt.Sprintf("/seller-admin/support?ticket_id=%d#confirmation-%d", ticket.ID, request), 1)
	check(customer, fmt.Sprintf("coordination:%d:open", request), "", 0)
	for i := 0; i < 2; i++ {
		if err = service.Reply(ctx, fixture.ownerID, request, "Private reply", "notification-reply-01"); err != nil {
			t.Fatal(err)
		}
	}
	var replyID int64
	_ = pool.QueryRow(ctx, `SELECT id FROM seller_confirmation_messages WHERE confirmation_id=$1`, request).Scan(&replyID)
	check(admin, fmt.Sprint("coordination-reply:", replyID), fmt.Sprintf("/admin/support?ticket_id=%d#confirmation-%d", ticket.ID, request), 1)
	check(super, fmt.Sprint("coordination-reply:", replyID), fmt.Sprintf("/super-admin/support?ticket_id=%d#confirmation-%d", ticket.ID, request), 1)
	if err = service.Close(ctx, admin, request, "Private recorded outcome"); err != nil {
		t.Fatal(err)
	}
	check(fixture.ownerID, fmt.Sprintf("coordination:%d:closed", request), fmt.Sprintf("/seller-admin/support?ticket_id=%d#confirmation-%d", ticket.ID, request), 1)
	check(other, fmt.Sprintf("coordination:%d:closed", request), "", 0)
	var state string
	_ = pool.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1`, order.ID).Scan(&state)
	if state != "pending_payment" {
		t.Fatalf("coordination changed order state: %s", state)
	}
}
