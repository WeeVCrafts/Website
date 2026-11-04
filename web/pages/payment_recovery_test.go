package pages

import (
	"bytes"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"strings"
	"testing"
)

func TestNoPaymentDemoDoesNotOfferProviderPaymentOrPoll(t *testing.T) {
	var out bytes.Buffer
	page := viewmodels.CustomerPage{NoPaymentDemo: true, PaymentReady: true, PaymentMode: "local", SelectedOrder: viewmodels.Order{Number: "QA-DEMO", State: "pending_payment", PaymentState: "created", CanPay: true}}
	if err := OrderConfirmationContent(page).Render(t.Context(), &out); err != nil {
		t.Fatal(err)
	}
	html := out.String()
	if strings.Contains(html, "data-retry-payment") || !strings.Contains(html, `data-payment-pending="false"`) || !strings.Contains(html, "No money has been charged") {
		t.Fatal("no-payment demo must remain an explicitly unpaid saved order")
	}
}
