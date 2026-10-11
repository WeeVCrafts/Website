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
		name               string
		page               viewmodels.AdminPage
		visible, invisible []string
	}{
		{name: "admin", page: viewmodels.AdminPage{PortalRoot: "/admin", PortalLabel: "Admin", RoleLabel: "Admin", DisplayName: "Maya Admin", CanFinance: true}, visible: []string{"Maya Admin", `href="/admin/orders"`, `href="/admin/support"`, `href="/admin/finance"`}, invisible: []string{`href="/admin/users"`, `href="/admin/security"`, `href="/admin/settings"`}},
		{name: "super admin", page: viewmodels.AdminPage{PortalRoot: "/super-admin", PortalLabel: "Super Admin", RoleLabel: "Super Admin", DisplayName: "Ravi Admin", CanFinance: true, CanSecurity: true}, visible: []string{`href="/super-admin/users"`, `href="/super-admin/roles"`, `href="/super-admin/security"`, `href="/super-admin/settings"`}},
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
		RoleAssignments: []domainidentity.RoleAssignment{{ID: 47, DisplayName: "Maya Seller", Email: "maya@example.test", Status: "active", Roles: []string{"seller"}}},
	}
	recorder := httptest.NewRecorder()
	if err := PortalSecurity(page).Render(t.Context(), recorder); err != nil {
		t.Fatalf("render security portal: %v", err)
	}
	body := recorder.Body.String()
	for _, expected := range []string{"Maya Seller", "maya@example.test", "seller", `action="/super-admin/roles/update"`, `name="csrf_token" value="csrf-test"`, `name="reason"`, `name="action"`} {
		if !strings.Contains(body, expected) {
			t.Errorf("role assignment form missing %q", expected)
		}
	}
}
