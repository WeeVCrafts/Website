package components

import (
	"strings"

	"github.com/wecratfs/commerce/internal/ports"
)

func marketplaceQueueLabel(value string) string {
	if value == "" {
		return "All exceptions"
	}
	return strings.ReplaceAll(value, "_", " ")
}

func sellerFulfillmentNextAction(order ports.MarketplaceOrderSummary) string {
	if order.Status == "cancelled" {
		return "Order cancelled"
	}
	switch order.SellerStage {
	case "awaiting_payment":
		return "Waiting for payment"
	case "new":
		return "Review and accept order"
	case "accepted":
		return "Start packing"
	case "packing":
		return "Confirm parcels and mark ready"
	case "ready_to_ship":
		return "Review booking and pickup status"
	case "shipped":
		return "Track delivery"
	case "delivered":
		return "View delivery record"
	case "cancelled":
		return "Your fulfillment was cancelled"
	default:
		return "View saved fulfillment details"
	}
}
