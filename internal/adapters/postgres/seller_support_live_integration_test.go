package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestSellerSupportRequestLinksSellerOrderAndRejectsOtherSeller(t *testing.T) {
	pool := openIntegrationPool(t)
	fixture := newCommerceFixture(t, pool, 2, 5)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'seller'),($2,'seller')`, fixture.ownerID, fixture.userIDs[1]); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sellers(owner_user_id,display_name,status) VALUES($1,'Other support seller','active')`, fixture.userIDs[1]); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM sellers WHERE owner_user_id=$1`, fixture.userIDs[1])
	})
	number := fmt.Sprintf("SELLER-SUPPORT-ORDER-%d", time.Now().UnixNano())
	var orderID int64
	if err := pool.QueryRow(ctx, `INSERT INTO orders(order_number,user_id,status,currency,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key) VALUES($1,$2,'delivered','INR',1000,1000,'{}','seller-support-test',$1) RETURNING id`, number, fixture.userIDs[0]).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO order_items(order_id,variant_id,product_name,sku,unit_price_cents,quantity,line_total_cents) VALUES($1,$2,'Support item','SUPPORT-ITEM',1000,1,1000)`, orderID, fixture.variantID); err != nil {
		t.Fatal(err)
	}
	repository := NewCommerceRepository(pool)
	service := applicationcommerce.NewSupportService(repository, NewRoleAuthorizer(pool))
	ticket, err := service.CreateSellerRequest(ctx, fixture.ownerID, number, "Order fulfilment issue", "Please review this customer's seller shipment.")
	if err != nil {
		t.Fatalf("seller's customer order rejected: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE actor_id=$1 AND action LIKE 'support.%'`, fmt.Sprint(fixture.ownerID))
		_, _ = pool.Exec(context.Background(), `DELETE FROM support_tickets WHERE id=$1`, ticket.ID)
	})
	var linkedOrderID int64
	if err := pool.QueryRow(ctx, `SELECT order_id FROM support_tickets WHERE id=$1`, ticket.ID).Scan(&linkedOrderID); err != nil || linkedOrderID != orderID {
		t.Fatalf("support ticket has incorrect order: %d %v", linkedOrderID, err)
	}
	if _, err := service.CreateSellerRequest(ctx, fixture.userIDs[1], number, "Foreign order issue", "This seller has no products in this order."); !errors.Is(err, ports.ErrOrderNotFound) {
		t.Fatalf("foreign seller order allowed: %v", err)
	}
	if _, err := service.ReplySellerWithMessageID(ctx, fixture.userIDs[1], ticket.ID, "Foreign reply"); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("foreign seller reply allowed: %v", err)
	}
	if _, err := service.ReplySellerWithMessageID(ctx, fixture.userIDs[0], ticket.ID, "Customer without seller role"); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("customer allowed seller reply: %v", err)
	}
}
