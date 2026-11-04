package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
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

func TestConfirmationReflectsRecordedPaymentAndEveryParcelStage(t *testing.T) {
	for _, stage := range domaincommerce.FulfillmentStages {
		order := viewmodels.Order{Number: "QA-ORDER", State: "paid", PaymentState: "captured", Shipments: []domaincommerce.OrderFulfillment{{Status: string(stage)}}}
		if confirmationProcessing(order) != (stage.Index() >= domaincommerce.FulfillmentProcessing.Index()) {
			t.Errorf("processing: %s", stage)
		}
		if confirmationDelivered(order) != (stage == domaincommerce.FulfillmentDelivered) {
			t.Errorf("delivered: %s", stage)
		}
	}
	mixed := viewmodels.Order{State: "paid", PaymentState: "captured", Shipments: []domaincommerce.OrderFulfillment{{Status: "delivered"}, {Status: "packing"}}}
	if !confirmationProcessing(mixed) || confirmationDelivered(mixed) {
		t.Fatal("partial delivery reported as complete")
	}
	refunded := viewmodels.Order{State: "refunded", PaymentState: "refunded", Shipments: []domaincommerce.OrderFulfillment{{Status: "delivered"}}}
	if !confirmationPaymentSettled(refunded) || !confirmationProcessing(refunded) || !confirmationDelivered(refunded) {
		t.Fatal("refund erased recorded delivery history")
	}
	for _, state := range []string{"pending_payment", "payment_failed", "cancelled", "refunded"} {
		var html bytes.Buffer
		if err := OrderConfirmation(viewmodels.CustomerPage{SelectedOrder: viewmodels.Order{Number: "QA-ORDER", State: state}}).Render(context.Background(), &html); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(html.String(), "Order placed successfully!") {
			t.Errorf("unpaid or closed order presented as success: %s", state)
		}
	}
}
