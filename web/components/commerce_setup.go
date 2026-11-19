package components

import (
	"fmt"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"strings"
	"time"
)

func commerceSetupValue(p viewmodels.CommerceSetupPage, key, fallback string) string {
	if p.Values != nil {
		if v, ok := p.Values[key]; ok && len(v) > 0 {
			return v[0]
		}
	}
	return fallback
}
func commerceSetupError(p viewmodels.CommerceSetupPage, key string) string { return p.Errors[key] }
func commerceInvalid(p viewmodels.CommerceSetupPage, key string) string {
	if p.Errors[key] != "" {
		return "true"
	}
	return "false"
}
func commercePercent(v int) string { return fmt.Sprintf("%d.%02d", v/100, v%100) }
func commerceDraftPercent(d ports.SetupSubmissionData) string {
	if d.DraftRatePercent != nil {
		return *d.DraftRatePercent
	}
	if d.HSN == "" {
		return ""
	}
	return commercePercent(d.RateBasisPoints)
}
func commerceDate(t time.Time) string {
	return t.In(time.FixedZone("IST", 19800)).Format("2 Jan 2006, 3:04 PM MST")
}
func commerceSetupReadOnly(p viewmodels.CommerceSetupPage) bool {
	return p.Selected != nil && (p.Selected.Status == "submitted" || p.Selected.Status == "approved" || p.Selected.Status == "superseded") && p.Error == ""
}

func commerceReadinessMissing(x ports.SellerReadinessOption) string {
	var missing []string
	if x.ProductStatus != "approved" {
		missing = append(missing, "product approval")
	}
	if !x.InvoiceReady {
		missing = append(missing, "seller invoice verification")
	}
	if !x.PickupReady {
		missing = append(missing, "verified pickup")
	}
	if !x.ParcelReady() {
		missing = append(missing, "packed measurements")
	}
	if !x.TaxReady || !x.Classified {
		missing = append(missing, "tax and return verification")
	}
	if x.Stock <= 0 {
		missing = append(missing, "stock")
	}
	if len(missing) == 0 {
		return "Setup complete; platform launch controls still apply"
	}
	return "Missing: " + strings.Join(missing, ", ")
}

func commerceReviewRows(p viewmodels.CommerceSetupPage) []struct{ Label, Current, Submitted string } {
	c, d := p.CurrentData, p.Data
	if p.Kind == "tax" {
		currentRate := "Not recorded"
		if c.HSN != "" {
			currentRate = commercePercent(c.RateBasisPoints)
		}
		return []struct{ Label, Current, Submitted string }{{"HSN", c.HSN, d.HSN}, {"Tax rate (%)", currentRate, commercePercent(d.RateBasisPoints)}, {"Return category", commerceLabel(c.ReturnClass), commerceLabel(d.ReturnClass)}}
	}
	return []struct{ Label, Current, Submitted string }{{"Legal name", c.LegalName, d.LegalName}, {"Registration", commerceLabel(c.Registration), commerceLabel(d.Registration)}, {"Invoice address", c.Address, d.Address}, {"State", c.StateCode, d.StateCode}, {"GSTIN", c.GSTIN, d.GSTIN}}
}

func commerceSetupSteps(p viewmodels.CommerceSetupPage) []struct{ Label, State string } {
	var x ports.SellerReadinessOption
	for _, r := range p.Rows {
		if p.VariantID == 0 || p.VariantID == r.VariantID {
			x = r
			break
		}
	}
	state := func(ready bool) string {
		if ready {
			return "Complete"
		}
		return "Missing"
	}
	review := "Not submitted"
	if p.Selected != nil {
		review = commerceLabel(p.Selected.Status)
	}
	return []struct{ Label, State string }{{"Seller invoice details", state(x.InvoiceReady)}, {"Pickup location", state(x.PickupReady)}, {"Packed measurements", state(x.ParcelReady())}, {"Tax & return details", state(x.TaxReady && x.Classified)}, {"Admin review", review}}
}
