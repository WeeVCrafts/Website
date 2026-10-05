package workspaces

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestWorkspacePageShowsOnlyProvidedRoleDestinations(t *testing.T) {
	page := viewmodels.WorkspacePage{
		DisplayName: "Vinay QA",
		Workspaces: []viewmodels.WorkspaceChoice{
			{Role: "finance_operator", Label: "Finance", Path: "/finance-portal"},
		},
	}
	recorder := httptest.NewRecorder()
	if err := Document(page).Render(t.Context(), recorder); err != nil {
		t.Fatalf("render workspace chooser: %v", err)
	}
	body := recorder.Body.String()
	for _, expected := range []string{"Vinay QA", "Finance", `href="/finance-portal"`} {
		if !strings.Contains(body, expected) {
			t.Errorf("workspace chooser does not contain %q: %s", expected, body)
		}
	}
	if strings.Contains(body, `href="/super-admin"`) {
		t.Fatalf("workspace chooser displayed a role not assigned in the view model: %s", body)
	}
}

func TestWorkspacePageShowsUnassignedStateWithoutPortalLinks(t *testing.T) {
	recorder := httptest.NewRecorder()
	if err := Document(viewmodels.WorkspacePage{}).Render(t.Context(), recorder); err != nil {
		t.Fatalf("render empty workspace chooser: %v", err)
	}
	if !strings.Contains(recorder.Body.String(), "No workspace is assigned") || strings.Contains(recorder.Body.String(), `class="choice"`) {
		t.Fatalf("empty workspace chooser should explain missing access without links: %s", recorder.Body.String())
	}
}
