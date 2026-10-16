package commerce

type CheckoutQuote struct {
	PolicyRevision int64 `json:"policy_revision"`
	CartID         int64 `json:"cart_id"`
	SubtotalCents  int64 `json:"subtotal_cents"`
	ShippingCents  int64 `json:"shipping_cents"`
	DiscountCents  int64 `json:"discount_cents"`
	TotalCents     int64 `json:"total_cents"`
	ItemCount      int   `json:"item_count"`
	Available      bool  `json:"available"`
}

func CheckoutShipping(subtotal int64) int64 {
	if subtotal > 0 && subtotal < 150000 {
		return 9900
	}
	return 0
}
