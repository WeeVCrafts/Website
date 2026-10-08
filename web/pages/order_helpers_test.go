package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestOrderHistoryUsesActualStateAndPurchasedQuantities(t *testing.T) {
	orders := []viewmodels.Order{{State: "paid"}, {State: "shipped"}, {State: "pending_payment"}, {State: "payment_failed"}, {State: "refunded"}, {State: "cancelled"}, {State: "delivered"}}
	if got := orderCount(orders, "active"); got != 3 {
		t.Fatalf("active order count=%d", got)
	}
	order := viewmodels.Order{State: "paid", Status: "Confirmed", Items: []viewmodels.CartItem{{Quantity: 2}, {Quantity: 1}}}
	var rendered bytes.Buffer
	if err := PrimaryOrder(order, "csrf").Render(context.Background(), &rendered); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "3 items") {
		t.Fatal("history does not show purchased quantity")
	}
	if strings.Contains(rendered.String(), "<b>Packed</b>") || strings.Contains(rendered.String(), "Maker update") {
		t.Fatal("history invents a packing update")
	}
}
