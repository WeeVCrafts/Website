package handlers

import "testing"

func TestManagementRouteMatching(t *testing.T) {
	for _, path := range []string{"/super-admin/customers", "/marketplace-admin/brands/create", "/super-admin/category-management/update", "/super-admin/commission-rules/delete", "/super-admin/settings/save", "/finance-portal/commission-rules", "/finance-portal/commission-rules/update"} {
		if _, _, _, ok := managementPortalPath(path); !ok {
			t.Errorf("live management route rejected: %s", path)
		}
	}
	for _, path := range []string{"/seller-admin/settings", "/super-admin/customers/fake", "/super-admin/brands/update/extra", "/super-admin/products", "/super-admin/settings/", "/finance-portal/settings", "/finance-portal/brands/create"} {
		if _, _, _, ok := managementPortalPath(path); ok {
			t.Errorf("unowned route intercepted: %s", path)
		}
	}
}
