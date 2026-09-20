package supportportal

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestLiveSupportPortalUsesCurrentIdentityAndPersistentCaseAction(t *testing.T) {
	page := viewmodels.SupportPortalPage{
		Active: "cases", Title: "Support cases", DisplayName: "Leela Agent", RoleLabel: "Support Agent", Initials: "LA", CSRFToken: "csrf-value",
		Cases: []viewmodels.SupportPortalCase{{DatabaseID: 91, ID: "CASE-91", Subject: "Order is delayed", Customer: "R*** K.", Priority: "normal", Created: "20 Sep 2026", Status: "open", Order: "WC-91"}},
	}
	recorder := httptest.NewRecorder()
	if err := LiveCases(page).Render(t.Context(), recorder); err != nil {
		t.Fatalf("render support case portal: %v", err)
	}
	body := recorder.Body.String()
	for _, visible := range []string{"Leela Agent", "Support Agent", "CASE-91", "Order is delayed", `action="/support-portal/cases/status"`, `name="ticket_id" value="91"`, `name="csrf_token" value="csrf-value"`} {
		if !strings.Contains(body, visible) {
			t.Errorf("live support portal is missing %q", visible)
		}
	}
	if strings.Contains(body, "Aditi Rao") || strings.Contains(body, "Priya Sharma") {
		t.Fatal("support portal rendered a sample identity instead of live case identity")
	}
}
