package commerce

import "testing"

func TestInvoiceStateChoicesMatchValidator(t *testing.T) {
	seen := map[string]bool{}
	telangana := false
	for _, s := range IndianStates() {
		if seen[s.Code] || !ValidIndianStateCode(s.Code) || IndianStateCode(s.Name) != s.Code {
			t.Fatalf("invalid selector state: %+v", s)
		}
		seen[s.Code] = true
		if s.Name == "Telangana" && s.Code == "36" {
			telangana = true
		}
	}
	if !telangana {
		t.Fatal("Telangana option missing")
	}
}
