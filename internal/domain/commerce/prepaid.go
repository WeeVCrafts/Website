package commerce

// PrepaidOrderConfirmed reports whether the captured order can enter fulfillment.
func PrepaidOrderConfirmed(orderStatus, paymentStatus string, paymentReview bool) bool {
	if paymentReview || paymentStatus != "captured" {
		return false
	}
	switch orderStatus {
	case "paid", "processing", "shipped", "delivered":
		return true
	default:
		return false
	}
}
