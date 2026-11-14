package pages

import (
	"fmt"
	"strconv"

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

// Confirmation uses the recorded parcel stages, including partial shipments.
func confirmationProcessing(order viewmodels.Order) bool {
	if !confirmationPaymentSettled(order) || order.State == "cancelled" {
		return false
	}
	if len(order.Shipments) > 0 {
		return domaincommerce.AggregateFulfillmentStage(order.Shipments).Index() >= domaincommerce.FulfillmentProcessing.Index()
	}
	switch order.State {
	case "processing", "packing", "packed", "shipped", "out_for_delivery", "delivered":
		return true
	}
	return false
}
func confirmationDelivered(order viewmodels.Order) bool {
	if !confirmationPaymentSettled(order) || order.State == "cancelled" {
		return false
	}
	if len(order.Shipments) > 0 {
		return domaincommerce.AggregateFulfillmentStage(order.Shipments) == domaincommerce.FulfillmentDelivered
	}
	return order.State == "delivered"
}

// A recorded refund does not erase the completed shipment history.
func confirmationPaymentSettled(order viewmodels.Order) bool {
	return order.PaymentState == "captured" || order.PaymentState == "refunded" || order.State == "refunded" || order.RefundStatus == "processed"
}

func shoppingShell(p viewmodels.CustomerPage) viewmodels.CustomerPage {
	if p.CheckoutIntentID != "" {
		p.Cart = p.ShoppingCart
		p.CartCount = p.ShoppingCartCount
		p.CheckoutIntentID = ""
	}
	return p
}

func productInitialQuantity(p viewmodels.CustomerPage) int {
	return min(productQuantityLimit(p.Product), max(1, p.ProductQuantity))
}

func canCheckPayment(p viewmodels.CustomerPage) bool {
	order := p.SelectedOrder
	return !p.NoPaymentDemo && !order.PaymentReview &&
		(order.State == "pending_payment" || order.State == "payment_failed")
}

func canContinuePayment(p viewmodels.CustomerPage) bool {
	order := p.SelectedOrder
	return canCheckPayment(p) && p.PaymentReady && order.CanPay &&
		!order.PaymentExpired && !order.PaymentInProgress &&
		!order.PaymentCreationPending && !order.PaymentCreationReview
}

func productQuantityLimit(product viewmodels.Product) int {
	for _, variant := range product.Variants {
		if selectedProductVariant(variant, product) {
			return variantQuantityLimit(variant)
		}
	}
	return purchaseQuantityLimit(product.Available, product.InStock)
}

func variantQuantityLimit(variant viewmodels.ProductVariant) int {
	available, _ := strconv.Atoi(variant.Available)
	return purchaseQuantityLimit(available, variant.InStock)
}

func purchaseQuantityLimit(available int, inStock bool) int {
	if !inStock {
		return 0
	}
	// Isolated mock storefront products may provide only an in-stock flag.
	return min(99, max(1, available))
}
