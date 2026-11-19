package components

import (
	"fmt"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"net/url"
	"strings"
)

func commerceMoney(v int64) string {
	sign := ""
	if v < 0 {
		sign = "−"
		v = -v
	}
	return fmt.Sprintf("%s₹%d.%02d", sign, v/100, v%100)
}
func commerceLabel(s string) string {
	if s == "" {
		return "Not recorded"
	}
	return strings.ToUpper(s[:1]) + strings.ReplaceAll(s[1:], "_", " ")
}
func commerceTabURL(p viewmodels.MarketplacePage, tab string) string {
	return p.OrderURL + "?tab=" + url.QueryEscape(tab)
}
func commerceRequestURL(p viewmodels.MarketplacePage, number, key string) string {
	return p.Back + "/orders/" + url.PathEscape(number) + "?tab=returns&focus=" + url.QueryEscape(key)
}
func commerceCustomerOrderURL(number, key string) string {
	return "/orders/" + url.PathEscape(number) + "?tab=returns&focus=" + url.QueryEscape(key)
}

func commercePageURL(p viewmodels.MarketplacePage, page int) string {
	q := url.Values{"q": {p.RequestSearch}, "status": {p.RequestBucket}, "page": {fmt.Sprint(page)}}
	if p.Number != "" {
		q.Set("tab", "returns")
	}
	return p.OrderURL + "?" + q.Encode()
}

func commercePackRemaining(p viewmodels.MarketplacePage, itemID int64) int {
	for _, item := range p.Lines {
		if item.ID == itemID {
			remaining := item.Quantity - item.Cancelled
			for _, pkg := range p.Packages {
				if pkg.Status == "cancelled" || strings.HasPrefix(pkg.Reference, "WC-RPL-") {
					continue
				}
				for _, piece := range pkg.Items {
					if piece.SKU == item.SKU {
						remaining -= piece.Quantity
					}
				}
			}
			if remaining < 0 {
				return 0
			}
			return remaining
		}
	}
	return 0
}
func commerceNeedsPacking(p viewmodels.MarketplacePage, seller int64) bool {
	for _, item := range p.Lines {
		if item.SellerID == seller && commercePackRemaining(p, item.ID) > 0 {
			return true
		}
	}
	return false
}

func commerceLegacyRefundExists(p viewmodels.MarketplacePage) bool {
	for _, r := range p.Requests.Rows {
		if r.Kind == "legacy_refund" {
			return true
		}
	}
	return false
}

func commerceClearURL(p viewmodels.MarketplacePage) string {
	if p.Number != "" {
		return commerceTabURL(p, "returns")
	}
	return p.OrderURL
}

func commerceReturnStages(current string) []string {
	var out []string
	for _, next := range []string{"approved", "pickup_pending", "received", "inspected", "rejected", "completed"} {
		if domain.CanTransition("return", current, next) {
			out = append(out, next)
		}
	}
	return out
}

func commerceNextAction(p viewmodels.MarketplacePage) (string, string, string) {
	if p.Status == "cancelled" {
		return "This order is cancelled. Recorded refund outcomes remain available.", "View requests", commerceTabURL(p, "returns")
	}
	if p.Payment == "under_review" {
		return "Payment needs reconciliation. Fulfillment must wait for verification.", "Review support", commerceTabURL(p, "support")
	}
	if p.Payment == "pending" || p.Payment == "authorized" || p.Payment == "failed" {
		if p.Role == "customer" {
			return "Payment confirmation is required before the seller can fulfill this order.", "Check payment", "/orders/" + url.PathEscape(p.Number) + "/confirmation"
		}
		return "Payment confirmation is still pending. Do not dispatch this order.", "Review payment", commerceTabURL(p, "overview")
	}
	for _, r := range p.Requests.Rows {
		if r.Bucket == "needs_review" || r.Bucket == "in_progress" {
			return "A return, claim or refund is awaiting resolution.", "View requests", commerceTabURL(p, "returns")
		}
	}
	for _, c := range p.Cases {
		if c.Status == "open" {
			return "A delivery or order exception needs an assigned action.", "Review support", commerceTabURL(p, "support")
		}
	}
	if p.Status == "completed" || p.Status == "delivered" {
		return "Delivery is recorded. Documents and eligible support options remain available.", "View documents", commerceTabURL(p, "documents")
	}
	if p.Role == "seller" {
		for _, s := range p.Sellers {
			if s.Status == "new" {
				return "Confirm availability before starting fulfillment.", "Review new order", commerceTabURL(p, "packages")
			}
			if s.Status == "accepted" || s.Status == "packing" {
				return "Pack the available items and record the actual parcel measurements.", "Continue packing", commerceTabURL(p, "packages")
			}
		}
	}
	return "Follow each package independently. Carrier updates confirm delivery progress.", "View packages", commerceTabURL(p, "packages")
}

func commerceFocusedRequests(p viewmodels.MarketplacePage) []ports.CommerceRequest {
	if p.Requests.Selected != nil {
		return []ports.CommerceRequest{*p.Requests.Selected}
	}
	return p.Requests.Rows
}
