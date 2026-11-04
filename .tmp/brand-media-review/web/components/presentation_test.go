package components

import (
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"testing"
)

func TestPresentationUsesAuthoredContentAndMeaningfulStates(t *testing.T) {
	if StatusTone("payment_failed") != "danger" || StatusLabel("payment_failed") != "Payment failed" || StatusTone("delivered") != "success" {
		t.Fatal("order status presentation does not match its meaning")
	}
	if DeliveryLocation(viewmodels.CustomerPage{}) != "Choose delivery location" || ProductImage("") != "/assets/images/product-unavailable.svg" || CustomerOption("Default variant") != "Standard" {
		t.Fatal("missing content or technical defaults leaked into customer copy")
	}
	description := "Made in small batches.\nMaterials: Cotton\nDimensions: 20 x 30 cm\nCare: Hand wash\nOther: Seller's description"
	if len(ProductSpecifications(description)) != 3 || SpecificationValue(description, "Care") != "Hand wash" || ProductStory(description) != "Made in small batches.\nOther: Seller's description" {
		t.Fatal("authored specifications and story were mixed or lost")
	}
	if len(ProductSpecifications("An object with no supplied specifications.")) != 0 {
		t.Fatal("missing specifications were invented")
	}
}
