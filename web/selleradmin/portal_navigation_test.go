package selleradmin

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestSellerStaffNavigationReflectsAssignedPermissions(t *testing.T) {
	page := viewmodels.SellerAdminPage{Active: "dashboard", Title: "Dashboard", DisplayName: "Asha Staff", RoleLabel: "Seller Staff", Initials: "AS", SellerName: "Asha's Studio", UserEmail: "asha@example.test", CanProducts: true, CanInventory: true}
	recorder := httptest.NewRecorder()
	if err := PortalDashboard(page).Render(t.Context(), recorder); err != nil {
		t.Fatalf("render seller portal: %v", err)
	}
	body := recorder.Body.String()
	for _, visible := range []string{"Asha Staff", "Seller Staff", "Asha&#39;s Studio", "/seller-admin/products", "/seller-admin/inventory"} {
		if !strings.Contains(body, visible) {
			t.Errorf("seller staff portal is missing %q", visible)
		}
	}
	for _, hidden := range []string{"Switch workspace", "/workspaces", "Team &amp; Permissions", "Create product draft", "/seller-admin/orders"} {
		if strings.Contains(body, hidden) {
			t.Errorf("seller staff portal exposes unassigned action %q", hidden)
		}
	}
}
