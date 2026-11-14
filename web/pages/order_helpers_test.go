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
	for _, order := range []viewmodels.Order{
		{State: "pending_payment", PaymentState: "pending"},
		{State: "pending_payment", PaymentState: "authorized", PaymentInProgress: true},
		{State: "payment_failed", PaymentState: "failed", PaymentExpired: true},
		{State: "paid", PaymentState: "captured", PaymentReview: true},
		{State: "cancelled", PaymentState: "captured"},
	} {
		var html bytes.Buffer
		order.Number = "QA-ORDER"
		if err := OrderConfirmation(viewmodels.CustomerPage{SelectedOrder: order, PaymentReady: true}).Render(context.Background(), &html); err != nil {
			t.Fatal(err)
		}
		for _, marker := range []string{"Order confirmed", "check_circle", "Download receipt", "data-retry-payment"} {
			if strings.Contains(html.String(), marker) {
				t.Errorf("unexpected %q for state %s/%s (review: %v)", marker, order.State, order.PaymentState, order.PaymentReview)
			}
		}
	}
}
