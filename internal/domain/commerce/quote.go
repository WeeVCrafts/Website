package commerce

import "context"

type CheckoutDelivery struct {
	State       string `json:"delivery_state,omitempty"`
	TransitDays int    `json:"delivery_transit_days,omitempty"`
}

type CheckoutQuote struct {
	CartItemsKey string `json:"cart_items_key,omitempty"`
	CheckoutDelivery
	ShippingPending      bool  `json:"shipping_pending,omitempty"`
	CreditAvailableCents int64 `json:"credit_available_cents"`
	CreditCents          int64 `json:"credit_cents"`
	CashDueCents         int64 `json:"cash_due_cents"`
	PolicyRevision       int64 `json:"policy_revision"`
	CartID               int64 `json:"cart_id"`
	SubtotalCents        int64 `json:"subtotal_cents"`
	ShippingCents        int64 `json:"shipping_cents"`
	DiscountCents        int64 `json:"discount_cents"`
	TotalCents           int64 `json:"total_cents"`
	ItemCount            int   `json:"item_count"`
	Available            bool  `json:"available"`
}

func CheckoutShipping(subtotal int64) int64 {
	if subtotal > 0 && subtotal < 150000 {
		return 9900
	}
	return 0
}

type expectedCartItemsKey struct{}

func WithExpectedCartItems(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, expectedCartItemsKey{}, key)
}

func ExpectedCartItems(ctx context.Context) string {
	key, _ := ctx.Value(expectedCartItemsKey{}).(string)
	return key
}

type expectedShippingKey struct{}

func WithExpectedShipping(ctx context.Context, cents int64) context.Context {
	return context.WithValue(ctx, expectedShippingKey{}, cents)
}

func ExpectedShipping(ctx context.Context) (int64, bool) {
	cents, ok := ctx.Value(expectedShippingKey{}).(int64)
	return cents, ok
}
