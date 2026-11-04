package main

import (
	"github.com/wecratfs/commerce/internal/domain/identity"
	"testing"
)

func TestDesignatedInputRequiresFourUniqueCanonicalIdentities(t *testing.T) {
	accounts := []account{
		{Username: "customer", Email: "customer@example.test", Role: identity.RoleCustomer, Password: "bootstrap-pass"},
		{Username: "seller", Email: "seller@example.test", Role: identity.RoleSeller, Password: "bootstrap-pass"},
		{Username: "admin", Email: "admin@example.test", Role: identity.RoleAdmin, Password: "bootstrap-pass"},
		{Username: "superadmin", Email: "superadmin@example.test", Role: identity.RoleSuperAdmin, Password: "bootstrap-pass"},
	}
	if err := validate(accounts); err != nil {
		t.Fatal(err)
	}
	if err := validate(accounts[:3]); err == nil {
		t.Fatal("partial set accepted")
	}
	accounts[3].Role = identity.RoleAdmin
	if err := validate(accounts); err == nil {
		t.Fatal("duplicate role accepted")
	}
	accounts[3].Role = "seller_staff"
	if err := validate(accounts); err == nil {
		t.Fatal("retired role accepted")
	}
}
