package admin

import (
	"net/http/httptest"
	"strings"
	"testing"

	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestRolePortalsRenderDistinctNavigationAndSignedInIdentity(t *testing.T) {
	tests := []struct {
		name      string
		page      viewmodels.AdminPage
		visible   []string
		invisible []string
	}{
		{name: "marketplace", page: viewmodels.AdminPage{PortalRoot: "/marketplace-admin", PortalLabel: "Marketplace Portal", RoleLabel: "Marketplace Admin", DisplayName: "Maya Admin"}, visible: []string{"Marketplace Portal", "Maya Admin", `href="/marketplace-admin/products"`, `href="/marketplace-admin/category-management"`, `href="/marketplace-admin/brands"`, `href="/marketplace-admin/customers"`}, invisible: []string{"Security &amp; Roles", "Payments &amp; Finance", `href="/marketplace-admin/commission-rules"`}},
		{name: "super admin", page: viewmodels.AdminPage{PortalRoot: "/super-admin", PortalLabel: "Super Admin Portal", RoleLabel: "Super Admin", DisplayName: "Ravi Admin", CanFinance: true, CanSecurity: true}, visible: []string{"Super Admin Portal", "Ravi Admin", "Security &amp; Roles", "Finance"}},
		{name: "finance", page: viewmodels.AdminPage{PortalRoot: "/finance-portal", PortalLabel: "Finance Portal", RoleLabel: "Finance Operator", DisplayName: "Nina Finance", IsFinance: true}, visible: []string{"Finance Portal", "Nina Finance", "Failed Payments", "Reconciliation"}, invisible: []string{"Security &amp; Roles", `href="/finance-portal/products"`}},
		{name: "operations", page: viewmodels.AdminPage{PortalRoot: "/operations-portal", PortalLabel: "Operations Portal", RoleLabel: "Returns Operations", DisplayName: "Omar Ops", IsOperations: true}, visible: []string{"Operations Portal", "Omar Ops", "Returns", "Orders"}, invisible: []string{"Security &amp; Roles", "Payments &amp; Finance", "Disputes"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			if err := PortalDashboard(test.page).Render(t.Context(), recorder); err != nil {
				t.Fatalf("render portal: %v", err)
			}
			body := recorder.Body.String()
			for _, value := range test.visible {
				if !strings.Contains(body, value) {
					t.Errorf("portal is missing %q", value)
				}
			}
			for _, value := range test.invisible {
				if strings.Contains(body, value) {
					t.Errorf("portal exposes %q", value)
				}
			}
		})
	}
}

func TestSecurityPortalRendersPersistentRoleAssignmentControls(t *testing.T) {
	page := viewmodels.AdminPage{
		PortalRoot: "/super-admin", PortalLabel: "Super Admin Portal", RoleLabel: "Super Admin",
		DisplayName: "Ravi Admin", CSRFToken: "csrf-test",
		RoleAssignments: []domainidentity.RoleAssignment{{ID: 47, DisplayName: "Maya Seller", Email: "maya@example.test", Status: "active", Roles: []string{"seller_owner"}}},
	}
	recorder := httptest.NewRecorder()
	if err := PortalSecurity(page).Render(t.Context(), recorder); err != nil {
		t.Fatalf("render security portal: %v", err)
	}
	body := recorder.Body.String()
	for _, expected := range []string{"Maya Seller", "maya@example.test", "seller_owner", `action="/super-admin/roles/update"`, `name="csrf_token" value="csrf-test"`, `name="reason"`, `name="action"`} {
		if !strings.Contains(body, expected) {
			t.Errorf("role assignment form missing %q", expected)
		}
	}
}
