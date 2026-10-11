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
