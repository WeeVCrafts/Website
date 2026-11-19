package components

import (
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

func TestCommerceTaxDraftDoesNotInventZeroRate(t *testing.T) {
	empty := ""
	for _, tc := range []struct {
		name string
		data ports.SetupSubmissionData
		want string
	}{
		{"missing profile", ports.SetupSubmissionData{}, ""},
		{"incomplete draft", ports.SetupSubmissionData{HSN: "4420", DraftRatePercent: &empty}, ""},
		{"recorded zero", ports.SetupSubmissionData{HSN: "4420", RateBasisPoints: 0}, "0.00"},
		{"recorded rate", ports.SetupSubmissionData{HSN: "4420", RateBasisPoints: 525}, "5.25"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := commerceDraftPercent(tc.data); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
