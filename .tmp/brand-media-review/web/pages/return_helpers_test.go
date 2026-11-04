package pages

import (
	"testing"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestReturnTabsMatchPersistedStatuses(t *testing.T) {
	returns := []viewmodels.Return{
		{ID: "1", Status: "Pending review"},
		{ID: "2", Status: "Approved"},
		{ID: "3", Status: "Item received"},
		{ID: "4", Status: "Refunded"},
		{ID: "5", Status: "Declined"},
	}

	if got := returnCount(returns, ""); got != 3 {
		t.Fatalf("current return count = %d, want 3", got)
	}
	if got := returnCount(returns, "refunds"); got != 1 {
		t.Fatalf("refunded return count = %d, want 1", got)
	}
	if got := returnCount(returns, "closed"); got != 1 {
		t.Fatalf("closed return count = %d, want 1", got)
	}
	if got := len(returnSubset(returns, "")); got != 3 {
		t.Fatalf("current return rows = %d, want 3", got)
	}
}

func TestReturnStatusLabel(t *testing.T) {
	for status, want := range map[string]string{
		"pending":  "Pending review",
		"approved": "Approved",
		"received": "Item received",
		"rejected": "Declined",
	} {
		if got := returnStatusLabel(status); got != want {
			t.Errorf("returnStatusLabel(%q) = %q, want %q", status, got, want)
		}
	}
}
