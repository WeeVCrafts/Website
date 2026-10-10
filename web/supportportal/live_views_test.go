package supportportal

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestLiveSupportPortalUsesCurrentIdentityAndCaseLink(t *testing.T) {
	page := viewmodels.SupportPortalPage{
		Active: "cases", Title: "Support cases", DisplayName: "Leela Agent", RoleLabel: "Support Agent", Initials: "LA", CSRFToken: "csrf-value",
		Cases: []viewmodels.SupportPortalCase{{DatabaseID: 91, ID: "CASE-91", Subject: "Order is delayed", Customer: "R*** K.", Priority: "normal", Created: "20 Sep 2026", Status: "open", Order: "WC-91"}},
	}
	recorder := httptest.NewRecorder()
	if err := LiveCases(page).Render(t.Context(), recorder); err != nil {
		t.Fatalf("render support case portal: %v", err)
	}
	body := recorder.Body.String()
	for _, visible := range []string{"Leela Agent", "Support Agent", "CASE-91", "Order is delayed", `href="/support-portal/cases?ticket_id=91&amp;q="`, `<table class="ui-table ui-table--records">`} {
		if !strings.Contains(body, visible) {
			t.Errorf("live support portal is missing %q", visible)
		}
	}
	if strings.Contains(body, "Aditi Rao") || strings.Contains(body, "Priya Sharma") {
		t.Fatal("support portal rendered a sample identity instead of live case identity")
	}
}

func TestLiveSupportCaseShowsStoredConversationAndCSRFReplyForm(t *testing.T) {
	page := viewmodels.SupportPortalPage{
		Active: "cases", Title: "Support cases", DisplayName: "Leela Agent", RoleLabel: "Support Agent", Initials: "LA", CSRFToken: "csrf-value",
		SelectedCase: viewmodels.SupportPortalCase{DatabaseID: 91, ID: "CASE-91", Subject: "Order is delayed", Customer: "R*** K.", Created: "20 Sep 2026", Status: "in_progress", InitialMessage: "My parcel has not arrived.", AgentNote: "Checking the carrier scan."},
		Messages:     []viewmodels.SupportPortalConversation{{Author: "Leela Agent", Initials: "LA", Role: "Support Agent", Message: "We are checking with the carrier.", Time: "20 Sep 2026 10:30", Tone: "agent"}},
		Cases:        []viewmodels.SupportPortalCase{{DatabaseID: 91, ID: "CASE-91", Subject: "Order is delayed", Customer: "R*** K.", Priority: "normal", Created: "20 Sep 2026", Status: "in_progress", Order: "WC-91", LatestReply: "We are checking with the carrier."}},
	}
	recorder := httptest.NewRecorder()
	if err := LiveCases(page).Render(t.Context(), recorder); err != nil {
		t.Fatalf("render selected support conversation: %v", err)
	}
	body := recorder.Body.String()
	for _, visible := range []string{"My parcel has not arrived.", "We are checking with the carrier.", "Checking the carrier scan.", `action="/support-portal/cases/reply"`, `name="ticket_id" value="91"`, `name="csrf_token" value="csrf-value"`, `name="message"`} {
		if !strings.Contains(body, visible) {
			t.Errorf("selected support conversation is missing %q", visible)
		}
	}
}

func TestCaseDetailSelectHasOnlyCurrentStatusSelected(t *testing.T) {
	page := viewmodels.SupportPortalPage{SelectedCase: viewmodels.SupportPortalCase{DatabaseID: 91, ID: "CASE-91", Status: "in_progress"}}
	r := httptest.NewRecorder()
	if err := LiveCases(page).Render(t.Context(), r); err != nil {
		t.Fatal(err)
	}
	body := r.Body.String()
	if strings.Count(body, " selected") != 1 || !strings.Contains(body, `value="in_progress" selected`) {
		t.Fatalf("incorrect selected status: %s", body)
	}
	if !strings.Contains(body, `action="/support-portal/cases/status"`) {
		t.Fatal("detail must retain status action")
	}
}
