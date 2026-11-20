package components

import (
	"encoding/json"
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
	return p.Selected != nil && (p.Selected.Status == "submitted" || p.Selected.Status == "approved" || p.Selected.Status == "attested" || p.Selected.Status == "superseded") && p.Error == ""
}

func commerceSetupStatus(status string) string {
	if status == "attested" {
		return "Confirmed by seller"
	}
	return commerceLabel(status)
}
func commerceReadinessMissing(x ports.SellerReadinessOption) string {
	var missing []string
	if x.ProductStatus != "approved" {
		missing = append(missing, "product approval")
	}
	if !x.ParcelReady() {
		missing = append(missing, "packed measurements")
	}
	if !x.TaxReady || !x.Classified {
		missing = append(missing, "tax and return details")
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
	return []struct{ Label, State string }{{"Business details", state(p.Shared.InvoiceReady)}, {"Pickup location", state(p.Shared.PickupVerified)}, {"Packed measurements", state(x.ParcelReady())}, {"Tax & return details", state(x.TaxReady && x.Classified)}}
}

type matchingTaxGroup struct {
	Details     ports.SetupSubmissionData
	Submissions []ports.SetupSubmission
}

func matchingTaxGroups(submissions []ports.SetupSubmission) []matchingTaxGroup {
	type key struct {
		Seller           int64
		HSN, ReturnClass string
		Rate             int
	}
	var groups []matchingTaxGroup
	index := make(map[key]int)
	for _, submission := range submissions {
		if submission.Kind != "tax" || submission.Status != "submitted" {
			continue
		}
		var details ports.SetupSubmissionData
		if json.Unmarshal(submission.Payload, &details) != nil {
			continue
		}
		k := key{submission.SellerID, details.HSN, details.ReturnClass, details.RateBasisPoints}
		i, ok := index[k]
		if !ok {
			i = len(groups)
			index[k] = i
			groups = append(groups, matchingTaxGroup{Details: details})
		}
		groups[i].Submissions = append(groups[i].Submissions, submission)
	}
	var matching []matchingTaxGroup
	for _, group := range groups {
		if len(group.Submissions) > 1 {
			matching = append(matching, group)
		}
	}
	return matching
}

func sellerInvoiceNext(setup ports.SellerFulfillmentSetup) string {
	switch setup.InvoiceSubmissionStatus {
	case "submitted":
		return "Waiting for Super Admin verification."
	case "changes_requested":
		return "Review the requested changes and resubmit."
	case "draft":
		return "Continue your draft."
	default:
		if setup.InvoiceReady && setup.InvoiceAttested {
			return "Confirmed by the seller for all product options."
		}
		if setup.InvoiceReady {
			return "Verified for this seller."
		}
		return "Confirm your business details once."
	}
}
