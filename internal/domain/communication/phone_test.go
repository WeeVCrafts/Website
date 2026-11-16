package communication

import "testing"

func TestIndianMobileKeepsOnlyValidAccountNumbers(t *testing.T) {
	for _, raw := range []string{"9876543210", "+91 98765-43210", "09876543210", "91(98765)43210"} {
		if n, ok := IndianMobile(raw); !ok || n != "919876543210" {
			t.Fatalf("valid display format rejected: %q", raw)
		}
	}
	for _, raw := range []string{"", "1234567890", "+12025550123", "+919876543210,+919876543211", "98765\n43210", "919876543210123456789012345", "98765<script>"} {
		if _, ok := IndianMobile(raw); ok {
			t.Fatalf("invalid recipient accepted: %q", raw)
		}
	}
}
