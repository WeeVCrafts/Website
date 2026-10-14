package pages

import (
	"bytes"
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestAuthSwitchKeepsOnlySafeReturnPaths(t *testing.T) {
	for _, next := range []string{"", "https://example.test", "//example.test", `/\\example.test`} {
		if got := authRouteURL("/register", url.Values{"next": {next}}); got != "/register" {
			t.Errorf("unsafe return path propagated: %q", got)
		}
	}
	next := "/products/blue-tote?category=textiles&sort=price-asc"
	got, err := url.Parse(authRouteURL("/register", url.Values{"next": {next}}))
	if err != nil || got.Query().Get("next") != next {
		t.Fatalf("return path lost: %v %v", got, err)
	}
}

func TestAccountScreensUseCompactAccessiblePresentation(t *testing.T) {
	for _, route := range []string{"/login", "/register"} {
		var output bytes.Buffer
		if err := Auth(viewmodels.CustomerPage{Route: route, Filters: url.Values{"next": {"/products/blue-tote"}}}).Render(context.Background(), &output); err != nil {
			t.Fatal(err)
		}
		body := output.String()
		for _, want := range []string{`class="auth-browse" href="/"`, `aria-label="Account access"`, `aria-label="Account help and policies"`, `novalidate`, `data-backend-form-message`, `aria-controls="` + strings.TrimPrefix(route, "/") + `-password"`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s missing %s", route, want)
			}
		}
		if strings.Contains(body, `class="site-footer"`) {
			t.Errorf("%s uses the full storefront footer", route)
		}
	}
	if isAuthPage("/products") || !isAuthPage("/forgot-password") {
		t.Fatal("auth presentation leaked to discovery routes")
	}
}
