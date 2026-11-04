package identity

import "testing"

func TestPreferredBrandPresentation(t *testing.T) {
	if Name != "Weevcrafts" {
		t.Fatalf("preferred brand: %q", Name)
	}
	for _, legacy := range []string{"WeevCrafts", "WeeVCrafts", "WeeVcrafts", "WEEVCRAFTS", "Weevcrafts"} {
		if got := DisplayName("Discover " + legacy + " today"); got != "Discover Weevcrafts today" {
			t.Fatalf("%q became %q", legacy, got)
		}
	}
	if DisplayName("https://weevcrafts.com/") != "https://weevcrafts.com/" {
		t.Fatal("public domain changed")
	}
}
