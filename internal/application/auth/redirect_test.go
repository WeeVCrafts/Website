package auth

import (
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"testing"
)

func TestCustomerRedirectAllowsOnlyCustomerPaths(t *testing.T) {
	roles := []domainidentity.Role{domainidentity.RoleCustomer}
	for _, input := range []string{"", "https://evil.test", "//evil.test", "/super-admin", "/account/../super-admin", "/%5c%5cevil.test", "/login"} {
		if got := RedirectDestination(roles, input); got != "/products" {
			t.Fatalf("unsafe next %q: %s", input, got)
		}
	}
	if got := RedirectDestination(roles, "/checkout?step=address"); got != "/checkout?step=address" {
		t.Fatal(got)
	}
	if got := RedirectDestination([]domainidentity.Role{domainidentity.RoleAdmin}, "/checkout"); got != "/admin" {
		t.Fatal(got)
	}
}
