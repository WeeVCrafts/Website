package admin

import (
	"bytes"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"strings"
	"testing"
)

func TestFailedReviewRefundRequiresSeparateBoundAuthorization(t *testing.T) {
	var out bytes.Buffer
	page := viewmodels.AdminPage{PortalRoot: "/super-admin", Title: "Payment reviews", CanFinance: true}
	cases := []ports.PaymentReviewCase{{ID: 7, OrderNumber: "QA-7", Status: "refund_pending", RefundSubmissionState: "failed", ProviderRefundID: "rfnd_failed_7", Currency: "INR", AmountCents: 1000}}
	if err := PaymentReviews(page, cases, 0, "").Render(t.Context(), &out); err != nil {
		t.Fatal(err)
	}
	html := out.String()
	if !strings.Contains(html, `action="/super-admin/payment-reviews/refund-retry"`) || !strings.Contains(html, `name="expected_refund_id" value="rfnd_failed_7"`) || !strings.Contains(html, "New refund authorization reason") {
		t.Fatal("failed refund lacks separate authorization bound to its prior provider refund")
	}
	if strings.Contains(html, `action="/super-admin/payment-reviews/approve"`) {
		t.Fatal("a pending refund must not offer stock fulfilment")
	}
}
