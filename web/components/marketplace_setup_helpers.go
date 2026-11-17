package components

import (
	"fmt"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"sort"
	"strconv"
)

func tabCurrent(active bool) string {
	if active {
		return "page"
	}
	return "false"
}

func setupSwitch(enabled bool) string {
	if enabled {
		return "Enabled"
	}
	return "Paused"
}

func setupFieldHint(name string) string {
	switch name {
	case "tax_rate_percent":
		return "Enter a percentage from 0 to 100 with up to two decimal places, for example 5 or 5.50."
	case "hsn":
		return "Enter the verified HSN using 4–8 digits."
	case "prefix":
		return "Use 1–6 uppercase letters, numbers or hyphens."
	case "gstin":
		return "Enter the 15-character GSTIN using uppercase letters and numbers."
	case "legal_name":
		return "Enter the seller’s legal name using 2–160 characters."
	case "reason":
		return "Enter a verification reason using at least four characters."
	}
	return "Check this value before saving."
}

func setupInputMode(name string) string {
	if name == "tax_rate_percent" {
		return "decimal"
	}
	if name == "hsn" {
		return "numeric"
	}
	return "text"
}
func invalidField(message string) string {
	if message != "" {
		return "true"
	}
	return "false"
}
func setupErrorKeys(errors map[string]string) []string {
	keys := make([]string, 0, len(errors))
	for k := range errors {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func setupTaxValue(p viewmodels.MarketplacePage) string {
	if p.Setup == nil || !p.Setup.TaxSaved {
		return ""
	}
	n := p.Setup.TaxBasisPoints
	return fmt.Sprintf("%d.%02d", n/100, n%100)
}
func setupFormID(p viewmodels.MarketplacePage, action, name string) int64 {
	v, _ := strconv.ParseInt(p.FormValue(action, name, ""), 10, 64)
	return v
}
