package handlers

import "testing"

func TestNextFulfillmentStatusFollowsOrder(t *testing.T) {
	tests := map[string]string{
		"pending":    "processing",
		"processing": "shipped",
		"shipped":    "delivered",
		"delivered":  "",
		"cancelled":  "",
	}
	for current, want := range tests {
		if got := nextFulfillmentStatus(current); got != want {
			t.Errorf("nextFulfillmentStatus(%q) = %q, want %q", current, got, want)
		}
	}
}
