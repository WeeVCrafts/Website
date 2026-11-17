package handlers

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

var setupKeys = []string{"seller_id", "variant_id", "seller_q", "product_q", "seller_before", "variant_before", "customer_q", "customer_before", "launch_q", "launch_before", "setup_before", "pickup_choices"}

func setupContext(values url.Values) url.Values {
	q := url.Values{"tab": {"setup"}}
	for _, k := range setupKeys {
		if v := values.Get(k); v != "" {
			q.Set(k, v)
		}
	}
	return q
}

func setupPostContext(values url.Values) url.Values {
	q := setupContext(url.Values{})
	// Only allowlisted dataset state enters URLs. Legal names, GSTINs and
	// addresses stay in POST bodies and the private rendered document.
	if encoded := values.Get("setup_context"); len(encoded) < 4096 {
		if v, err := url.ParseQuery(encoded); err == nil {
			q = setupContext(v)
		}
	}
	if values.Get("action") != "save_shipping_profile" {
		q.Del("pickup_choices")
	}
	return q
}

func isMarketplaceSetupAction(action string) bool {
	switch action {
	case "save_shipping_profile", "save_invoice_profile", "save_tax_profile", "publish_policy", "pause_shipping", "configure_pilot", "classify_variant", "configure_live_catalog":
		return true
	}
	return false
}

func parseTaxPercent(values url.Values) (int, error) {
	percent, present := values["tax_rate_percent"]
	if !present {
		rate, err := strconv.Atoi(values.Get("rate_basis_points"))
		if err != nil || rate < 0 || rate > 10000 {
			return 0, fmt.Errorf("Enter a tax rate from 0 to 100%%")
		}
		return rate, nil
	}
	if len(percent) != 1 {
		return 0, fmt.Errorf("Enter one tax rate")
	}
	s := strings.TrimSpace(percent[0])
	if !regexp.MustCompile(`^\d{1,3}(\.\d{1,2})?$`).MatchString(s) {
		return 0, fmt.Errorf("Enter a percentage with up to two decimal places")
	}
	parts := strings.SplitN(s, ".", 2)
	whole, _ := strconv.Atoi(parts[0])
	fraction := 0
	if len(parts) == 2 {
		fraction, _ = strconv.Atoi((parts[1] + "00")[:2])
	}
	rate := whole*100 + fraction
	if rate > 10000 {
		return 0, fmt.Errorf("Enter a tax rate from 0 to 100%%")
	}
	if legacy, ok := values["rate_basis_points"]; ok {
		if len(legacy) != 1 {
			return 0, fmt.Errorf("Conflicting tax rates")
		}
		n, err := strconv.Atoi(legacy[0])
		if err != nil || n != rate {
			return 0, fmt.Errorf("Conflicting tax rates; enter the percentage again")
		}
	}
	return rate, nil
}

func validateSetup(action string, f url.Values) map[string]string {
	e := map[string]string{}
	minimum := 4
	if action == "publish_policy" || action == "pause_shipping" || action == "configure_pilot" || action == "configure_live_catalog" {
		minimum = 10
	}
	if action != "classify_variant" && len(strings.TrimSpace(f.Get("reason"))) < minimum {
		e["reason"] = fmt.Sprintf("Enter a reason of at least %d characters.", minimum)
	}
	if action == "publish_policy" || action == "pause_shipping" || action == "configure_pilot" || action == "configure_live_catalog" || action == "classify_variant" {
		if f.Get("confirmed") != "yes" {
			e["confirmed"] = "Confirm the policy or shipping change before saving."
		}
	}
	positive := func(key, label string) {
		n, err := strconv.ParseInt(f.Get(key), 10, 64)
		if err != nil || n <= 0 {
			e[key] = "Choose " + label + "."
		}
	}
	switch action {
	case "save_invoice_profile":
		positive("seller_id", "a seller")
		if n := len(strings.TrimSpace(f.Get("legal_name"))); n < 2 || n > 160 {
			e["legal_name"] = "Enter the seller’s legal name (2–160 characters)."
		}
		if n := len(strings.TrimSpace(f.Get("address"))); n < 10 || n > 1000 {
			e["address"] = "Enter the registered address (10–1,000 characters)."
		}
		if !domain.ValidIndianStateCode(f.Get("state_code")) {
			e["state_code"] = "Choose a state or union territory."
		}
		if !regexp.MustCompile(`^[A-Z0-9-]{1,6}$`).MatchString(f.Get("prefix")) {
			e["prefix"] = "Use 1–6 uppercase letters, numbers or hyphens."
		}
		switch f.Get("registration") {
		case "registered":
			if !regexp.MustCompile(`^[0-9]{2}[A-Z0-9]{13}$`).MatchString(strings.ToUpper(strings.TrimSpace(f.Get("gstin")))) {
				e["gstin"] = "Enter the 15-character GSTIN from the seller’s registration."
			}
		case "unregistered":
			if strings.TrimSpace(f.Get("gstin")) != "" {
				e["gstin"] = "Clear GSTIN when selecting Unregistered."
			}
		default:
			e["registration"] = "Choose the seller’s registration status."
		}
	case "save_tax_profile":
		positive("variant_id", "a product option")
		if !regexp.MustCompile(`^[0-9]{4,8}$`).MatchString(strings.TrimSpace(f.Get("hsn"))) {
			e["hsn"] = "Enter the verified HSN (4–8 digits)."
		}
		if _, err := parseTaxPercent(f); err != nil {
			e["tax_rate_percent"] = err.Error()
		}
	case "save_shipping_profile":
		positive("seller_id", "a seller")
		if f.Get("pickup_location") == "" {
			e["pickup_location"] = "Choose a verified Shiprocket pickup location."
		}
	case "classify_variant":
		positive("variant_id", "a product option")
		if f.Get("return_class") != "textile_craft" && f.Get("return_class") != "fragile_art" {
			e["return_class"] = "Choose the verified return classification."
		}
	case "configure_pilot":
		positive("variant_id", "a product option")
		positive("customer_id", "an active customer")
	case "configure_live_catalog":
		positive("variant_id", "the first ready option")
		positive("second_variant_id", "a different seller’s ready option")
		if f.Get("variant_id") == f.Get("second_variant_id") {
			e["second_variant_id"] = "Choose a different seller’s ready option."
		}
	}
	return e
}

func (h *MarketplaceHandler) loadSetup(ctx context.Context, actor int64, p *viewmodels.MarketplacePage) error {
	q := p.SetupContext
	num := func(k string) int64 { v, _ := strconv.ParseInt(q.Get(k), 10, 64); return v }
	d, err := h.Repository.FulfillmentSetup(ctx, actor, ports.MarketplaceSetupQuery{SellerID: num("seller_id"), VariantID: num("variant_id"), SellerBefore: num("seller_before"), VariantBefore: num("variant_before"), CustomerBefore: num("customer_before"), LaunchBefore: num("launch_before"), SellerSearch: q.Get("seller_q"), ProductSearch: q.Get("product_q"), CustomerSearch: q.Get("customer_q"), LaunchSearch: q.Get("launch_q")})
	if err != nil {
		return err
	}
	p.Setup = &d
	p.States = domain.IndianStates()
	// A live read is requested explicitly; routine invoice/tax editing does not
	// wait on Shiprocket. Saving always revalidates with the provider.
	if q.Get("seller_id") != "" && q.Get("pickup_choices") == "1" && h.PickupChecker != nil {
		locations, e := h.PickupChecker.PickupPostcodes(ctx)
		if e != nil {
			p.PickupError = "Shiprocket pickup locations could not be loaded. Retry when the provider is available."
		} else {
			for name, pin := range locations {
				p.Pickups = append(p.Pickups, viewmodels.MarketplacePickupChoice{Name: name, PIN: pin})
			}
			sort.Slice(p.Pickups, func(i, j int) bool { return p.Pickups[i].Name < p.Pickups[j].Name })
		}
	}
	if q.Get("pickup_choices") == "1" && h.PickupChecker == nil {
		p.PickupError = "Live Shiprocket configuration is required to load verified pickup locations."
	}
	if p.SetupAction == "save_shipping_profile" && len(p.Pickups) == 0 && p.SetupValues.Get("pickup_location") != "" {
		p.Pickups = []viewmodels.MarketplacePickupChoice{{Name: p.SetupValues.Get("pickup_location")}}
	}
	return nil
}
