package handlers

import "testing"

func TestNotificationRedirectsRemainOnSite(t *testing.T) {
	for _, link := range []string{"https://example.com", "//example.com", "/\\example.com", "/foo\r\nLocation: https://example.com", ""} {
		if got := safeNotificationLink(link); got != "/notifications" {
			t.Errorf("unsafe link %q accepted as %q", link, got)
		}
	}
	for _, link := range []string{"/admin/support?ticket_id=42", "/seller-admin/support?ticket_id=1#confirmation-2", "/orders/WC-1"} {
		if got := safeNotificationLink(link); got != link {
			t.Errorf("safe link %q changed to %q", link, got)
		}
	}
}

func TestNotificationDestinationResolvesRelevantOrderTab(t *testing.T) {
	for _, tc := range []struct{ title, link, want string }{
		{"Refund processed", "/orders/WC-1/fulfillment", "/orders/WC-1/fulfillment?tab=returns"},
		{"Items cancelled", "/seller-admin/orders/WC-1/fulfillment", "/seller-admin/orders/WC-1/fulfillment?tab=returns"},
		{"Package delivered", "/orders/WC-1/fulfillment", "/orders/WC-1/fulfillment"},
		{"Refund processed", "/orders/WC-1/fulfillment?tab=returns", "/orders/WC-1/fulfillment?tab=returns"},
		{"Reply", "", "/notifications?filter=unread"},
	} {
		if got := notificationDestination(tc.title, tc.link, "unread"); got != tc.want {
			t.Errorf("destination for %q and %q = %q, want %q", tc.title, tc.link, got, tc.want)
		}
	}
}
