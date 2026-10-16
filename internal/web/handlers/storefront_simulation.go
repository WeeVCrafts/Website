package handlers

import (
	"fmt"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"net/http"
	"strings"
	"time"
)

// Simulation reads definitions and current counters without creating a cart,
// consuming a coupon, reserving inventory or creating an order.
func (h *LivePortalHandler) simulateStorefront(r *http.Request, c storefront.Config) string {
	subtotal, e := parseStorefrontMoney(r.URL.Query().Get("subtotal"))
	if e != nil || subtotal < 0 || subtotal > 100000000 {
		return "Enter an example subtotal from ₹0 to ₹1,000,000 with up to two decimal places."
	}
	pin := r.URL.Query().Get("pin")
	shipping := c.Settings.Policy.Shipping(subtotal)
	discount := int64(0)
	couponNotice := "No coupon entered."
	code := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("coupon")))
	if code != "" {
		couponNotice = "Coupon unavailable in this draft."
		if !c.Settings.Features.Coupons {
			couponNotice = "Coupon entry is disabled in this draft."
		} else {
			now := time.Now()
			for _, p := range c.Promotions {
				for _, coupon := range p.Coupons {
					if coupon.Code != code {
						continue
					}
					if !coupon.Active || p.Status != "active" || p.StartsAt != nil && now.Before(*p.StartsAt) || p.EndsAt != nil && !now.Before(*p.EndsAt) {
						couponNotice = "Coupon is inactive or outside its date range."
						continue
					}
					var campaignUses, couponUses int64
					if e := h.pool.QueryRow(r.Context(), `SELECT COALESCE((SELECT usage_count FROM promotions WHERE slug=$1),0),COALESCE((SELECT usage_count FROM coupons WHERE code=$2),0)`, p.Key, coupon.Code).Scan(&campaignUses, &couponUses); e != nil {
						return "Usage counts could not be read. Try the simulation again."
					}
					if p.UsageLimit > 0 && campaignUses >= p.UsageLimit || coupon.UsageLimit > 0 && couponUses >= coupon.UsageLimit {
						couponNotice = "Coupon usage limit reached."
						continue
					}
					if p.TargetMode != "all" {
						couponNotice = "This coupon is limited to selected products. Select those items at checkout to calculate their eligible amount."
						continue
					}
					switch p.Kind {
					case "percentage":
						discount = subtotal * p.Value / 100
					case "fixed":
						discount = min(subtotal, p.Value)
					case "free_shipping":
						discount = shipping
					}
					couponNotice = "Coupon definition applies to this example. Per-customer redemption history is checked at checkout."
				}
			}
		}
	}
	coverage := "Serviceable"
	if !c.Settings.Policy.Serves(pin) {
		coverage = "Not serviceable"
	}
	return fmt.Sprintf("Example cart: %s · shipping %s · discount %s · total %s. PIN %s: %s. %s", storefrontMoney(subtotal), storefrontMoney(shipping), storefrontMoney(discount), storefrontMoney(subtotal+shipping-discount), pin, coverage, couponNotice)
}
