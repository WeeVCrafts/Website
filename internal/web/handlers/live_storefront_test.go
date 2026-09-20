package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/pages"
)

func TestLiveTotalsUsesIntegerCents(t *testing.T) {
	items, shipping, discount, total := liveTotals([]viewmodels.CartItem{{
		Product: viewmodels.Product{PriceCents: 24900}, Quantity: 2,
	}})
	if items != "INR 498" || shipping != "INR 99" || discount != "INR 0" || total != "INR 597" {
		t.Fatalf("liveTotals() = (%q, %q, %q, %q)", items, shipping, discount, total)
	}
}

func TestLiveSlugNormalizesSellerNames(t *testing.T) {
	if got := liveSlug("Weaver's Touch"); got != "weavers-touch" {
		t.Fatalf("liveSlug() = %q, want weavers-touch", got)
	}
	if got := liveSlug("Earth & Fire"); got != "earth-and-fire" {
		t.Fatalf("liveSlug() = %q, want earth-and-fire", got)
	}
}

func TestCustomerPagesUseTheSharedStorefront(t *testing.T) {
	handler := &LiveStorefrontHandler{}
	for _, path := range []string{
		"/", "/login", "/register", "/account", "/account/profile", "/account/addresses",
		"/products", "/products/example", "/cart", "/wishlist", "/checkout", "/orders",
		"/orders/WC123", "/tracking", "/returns", "/payments", "/contact", "/faq",
	} {
		if !handler.ownsRoute(path) {
			t.Errorf("ownsRoute(%q) = false, want customer page to use shared storefront", path)
		}
	}
	for _, path := range []string{"/admin", "/seller-admin", "/support-portal", "/api/v1/products", "/health/ready"} {
		if handler.ownsRoute(path) {
			t.Errorf("ownsRoute(%q) = true, want non-storefront route to remain outside customer shell", path)
		}
	}
}

func TestCustomerPagesRequireAuthentication(t *testing.T) {
	for _, path := range []string{"/account", "/account/profile", "/cart", "/wishlist", "/checkout", "/orders", "/returns", "/payments"} {
		if !requiresCustomerSession(path) {
			t.Errorf("requiresCustomerSession(%q) = false", path)
		}
	}
	for _, path := range []string{"/", "/login", "/register", "/products", "/faq"} {
		if requiresCustomerSession(path) {
			t.Errorf("requiresCustomerSession(%q) = true", path)
		}
	}
}

func TestSafeCustomerNextBlocksExternalRedirects(t *testing.T) {
	for _, candidate := range []string{"", "https://example.com", "//example.com", "/\\example.com"} {
		if got := safeCustomerNext(candidate, "/account"); got != "/account" {
			t.Errorf("safeCustomerNext(%q) = %q, want /account", candidate, got)
		}
	}
	if got := safeCustomerNext("/checkout?step=address", "/account"); got != "/checkout?step=address" {
		t.Errorf("safeCustomerNext() = %q, want internal path", got)
	}
}

func TestLiveOrderStatusIsCustomerReadable(t *testing.T) {
	if got := displayOrderStatus("pending_payment"); got != "Awaiting payment" {
		t.Fatalf("displayOrderStatus(pending_payment) = %q", got)
	}
	if got := displayPaymentStatus("", "payment_failed"); got != "Failed" {
		t.Fatalf("displayPaymentStatus() = %q", got)
	}
}

func TestPaymentReadinessFollowsConfiguredMode(t *testing.T) {
	for _, test := range []struct {
		mode       string
		configured bool
		want       bool
	}{
		{mode: "local", want: true},
		{mode: "razorpay", configured: true, want: true},
		{mode: "razorpay", want: false},
		{mode: "unknown", configured: true, want: false},
	} {
		if got := paymentReady(test.mode, test.configured); got != test.want {
			t.Errorf("paymentReady(%q, %v) = %v, want %v", test.mode, test.configured, got, test.want)
		}
	}
}

func TestCheckoutPlaceOrderButtonTracksPaymentReadiness(t *testing.T) {
	render := func(paymentReady bool) string {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/checkout", nil)
		page := viewmodels.CustomerPage{Route: "/checkout", Title: "Checkout", PaymentReady: paymentReady}
		if err := pages.Checkout(page).Render(request.Context(), recorder); err != nil {
			t.Fatalf("render checkout: %v", err)
		}
		return recorder.Body.String()
	}
	for _, test := range []struct {
		ready        bool
		wantDisabled bool
	}{
		{ready: true, wantDisabled: false},
		{ready: false, wantDisabled: true},
	} {
		body := render(test.ready)
		start := strings.Index(body, `data-backend-action="checkout"`)
		if start < 0 {
			t.Fatal("checkout action button was not rendered")
		}
		buttonStart := strings.LastIndex(body[:start], "<button")
		buttonEnd := strings.Index(body[start:], "</button>")
		if buttonStart < 0 || buttonEnd < 0 {
			t.Fatal("checkout action button markup is incomplete")
		}
		button := body[buttonStart : start+buttonEnd]
		if got := strings.Contains(button, " disabled"); got != test.wantDisabled {
			t.Errorf("PaymentReady=%v disabled=%v, want %v", test.ready, got, test.wantDisabled)
		}
	}
}
