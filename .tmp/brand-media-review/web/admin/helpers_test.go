package admin

import "testing"

func TestDisplayInitialsUsesNameParts(t *testing.T) {
	if got := displayInitials("Vinay Kumar"); got != "VK" {
		t.Fatalf("displayInitials() = %q, want VK", got)
	}
	if got := displayInitials("Vinay"); got != "V" {
		t.Fatalf("displayInitials(single name) = %q, want V", got)
	}
}
