package selleradmin

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestSellerNavigationIncludesOwnedBusinessModules(t *testing.T) {
	page := viewmodels.SellerAdminPage{Active: "dashboard", Title: "Dashboard", DisplayName: "Asha Rao", RoleLabel: "Seller", Initials: "AS", SellerName: "Asha's Studio", UserEmail: "asha@example.test", CanProducts: true, CanProductWrite: true, CanInventory: true, CanOrders: true, CanFulfill: true, CanReturns: true, CanReports: true}
	recorder := httptest.NewRecorder()
	if err := PortalDashboard(page).Render(t.Context(), recorder); err != nil {
		t.Fatalf("render seller portal: %v", err)
	}
	body := recorder.Body.String()
	for _, visible := range []string{"Asha Rao", "Seller", "Asha&#39;s Studio", "/seller-admin/products", "/seller-admin/inventory", "/seller-admin/orders"} {
		if !strings.Contains(body, visible) {
			t.Errorf("seller portal is missing %q", visible)
		}
	}
	for _, hidden := range []string{"Switch workspace", "/workspaces", "Team &amp; Permissions", "/seller-admin/team"} {
		if strings.Contains(body, hidden) {
			t.Errorf("seller portal exposes retired action %q", hidden)
		}
	}
}
