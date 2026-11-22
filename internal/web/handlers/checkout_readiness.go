package handlers

import (
	"context"
	"slices"
	"time"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func (h *MarketplaceHandler) loadCheckoutChecks(ctx context.Context, p *viewmodels.MarketplacePage) error {
	result, err := h.Repository.CheckoutHealth(ctx)
	if err != nil || !result.Enabled {
		return err
	}
	p.AutomaticCheckout = true
	p.CheckoutPaused = result.Paused
	if !slices.Contains(result.Blockers, "unchecked") {
		p.CheckoutCheckedAt = result.CheckedAt
	}
	fresh := result.ValidUntil.After(time.Now())
	status := func(ok bool) string {
		if ok {
			return "Passed"
		}
		return "Action needed"
	}
	appendCheck := func(label, next string, ready bool) {
		p.CheckoutChecks = append(p.CheckoutChecks, viewmodels.CheckoutCheck{Label: label, Status: status(ready), Next: next})
	}
	appendCheck("New checkout switch", "Enable WECRATFS_CHECKOUT_ENABLED in the website runtime settings.", h.CheckoutEnabled)
	appendCheck("Published returns policy", "Publish the approved returns policy in Fulfillment Setup.", p.Policy.PublishedAt != nil)
	appendCheck("Emergency pause", "Resume checkout after resolving the issue that caused the pause.", !result.Paused)
	if !fresh {
		p.CheckoutChecks = append(p.CheckoutChecks, viewmodels.CheckoutCheck{Label: "Integration results", Status: "Awaiting verification", Next: "Request a new check and confirm the readiness worker is running."})
		return nil
	}
	missing := func(codes ...string) bool {
		return slices.ContainsFunc(codes, func(code string) bool { return slices.Contains(result.Blockers, code) })
	}
	appendCheck("Public content approval", "Review and approve production storefront information.", !missing("public_content"))
	appendCheck("Razorpay API access", "Check the live Razorpay API credentials and connection.", !missing("razorpay_config", "razorpay_access"))
	appendCheck("Razorpay webhook configuration", "Configure the signing secret and verify callback delivery in staging.", !missing("razorpay_webhook"))
	appendCheck("Shiprocket booking configuration", "Verify the live credentials and pickup configuration, then enable bookings.", !missing("shiprocket_config", "shiprocket_bookings"))
	appendCheck("Shiprocket pickup access", "Check access to the provider pickup list. Verify individual sellers in Pickup location.", !missing("shiprocket_config", "shiprocket_pickups"))
	appendCheck("Shiprocket wallet", "Check that the API user can read the wallet and that the account has available funds.", !missing("shiprocket_config", "shiprocket_wallet", "shiprocket_wallet_access"))
	return nil
}
