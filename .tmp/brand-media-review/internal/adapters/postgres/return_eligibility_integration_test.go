package postgres

import (
	"context"
	"errors"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

func TestReturnRequiresDeliveredOrderAndOwner(t *testing.T) {
	pool := openIntegrationPool(t)
	f := newCommerceFixture(t, pool, 2, 5)
	r := NewCommerceRepository(pool)
	ctx := context.Background()
	order, err := r.CreateOrder(ctx, f.userIDs[0], f.cartIDs[0], "return-eligibility-qa", domaincommerce.AddressInput{RecipientName: "QA", Phone: "9000000000", Line1: "Test lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"paid", "processing", "shipped"} {
		if _, err = pool.Exec(ctx, `UPDATE orders SET status=$1 WHERE id=$2`, status, order.ID); err != nil {
			t.Fatal(err)
		}
		if _, err = r.CreateReturnRequest(ctx, f.userIDs[0], order.OrderNumber, "QA eligibility verification"); !errors.Is(err, ports.ErrReturnNotFound) {
			t.Fatalf("return at status %s error=%v; want ineligible", status, err)
		}
	}
	if _, err = pool.Exec(ctx, `UPDATE orders SET status='delivered' WHERE id=$1`, order.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CreateReturnRequest(ctx, f.userIDs[1], order.OrderNumber, "QA ownership verification"); !errors.Is(err, ports.ErrReturnNotFound) {
		t.Fatalf("foreign return error=%v; want not found", err)
	}
	if _, err = r.CreateReturnRequest(ctx, f.userIDs[0], order.OrderNumber, "QA eligible delivered return"); err != nil {
		t.Fatalf("delivered owner return error=%v", err)
	}
}
