package pages

import (
	"fmt"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func checkoutItemsLabel(count int) string {
	if count == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", count)
}

func checkoutMakersLabel(items []viewmodels.CartItem) string {
	count := checkoutSellerCount(items)
	if count == 1 {
		return "1 maker"
	}
	return fmt.Sprintf("%d makers", count)
}

func checkoutSellerCount(items []viewmodels.CartItem) int {
	makers := make(map[string]struct{})
	for _, item := range items {
		makers[item.Product.SellerSlug+":"+item.Product.Seller] = struct{}{}
	}
	return len(makers)
}

type shipmentStep struct{ Label, Class, Icon, State string }

func shipmentSteps(status string) []shipmentStep {
	stage, ok := domaincommerce.ParseFulfillmentStage(status)
	if !ok {
		stage = domaincommerce.FulfillmentPending
	}
	current := stage.Index()
	labels := []string{"Order confirmed", "Processing", "Packing", "Shipped", "Ready to deliver", "Delivered"}
	steps := make([]shipmentStep, len(labels))
	for i, label := range labels {
		steps[i] = shipmentStep{Label: label, Icon: "radio_button_unchecked", State: "Next"}
		if i < current || stage == domaincommerce.FulfillmentDelivered {
			steps[i].Class = "done"
			steps[i].Icon = "check_circle"
			steps[i].State = "Complete"
		} else if i == current {
			steps[i].Class = "current"
			steps[i].Icon = "radio_button_checked"
			steps[i].State = "Current"
		}
	}
	return steps
}
