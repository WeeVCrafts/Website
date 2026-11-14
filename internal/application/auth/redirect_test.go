package auth

import (
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"testing"
)

func TestCustomerRedirectAllowsOnlyCustomerPaths(t *testing.T) {
	roles := []domainidentity.Role{domainidentity.RoleCustomer}
	for _, input := range []string{"", "https://evil.test", "//evil.test", "/super-admin", "/account/../super-admin", "/%5c%5cevil.test", "/login"} {
		if got := RedirectDestination(roles, input); got != "/" {
			t.Fatalf("unsafe next %q: %s", input, got)
		}
	}
	for _, next := range []string{"/", "/cart", "/wishlist", "/orders/ORDER-1", "/checkout?step=address&intent=selected"} {
		if got := RedirectDestination(roles, next); got != next {
			t.Fatalf("customer continuation %q became %q", next, got)
		}
	}
	if got := RedirectDestination([]domainidentity.Role{domainidentity.RoleAdmin}, "/checkout"); got != "/admin" {
		t.Fatal(got)
	}
}
