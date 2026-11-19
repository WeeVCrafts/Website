package commerce

type OrderFinancialSummary struct {
	OriginalTotalCents    int64  `json:"original_total_cents"`
	CapturedCents         int64  `json:"captured_cents"`
	ProviderRefundedCents int64  `json:"provider_refunded_cents"`
	PendingRefundCents    int64  `json:"pending_refund_cents"`
	CreditAppliedCents    int64  `json:"credit_applied_cents"`
	CreditRestoredCents   int64  `json:"credit_restored_cents"`
	NetProviderPaidCents  int64  `json:"net_provider_paid_cents"`
	SellerScoped          bool   `json:"seller_scoped"`
	DisplayStatus         string `json:"display_status"`
}

type OrderItemProgress struct {
	ID             int64  `json:"id"`
	SellerID       int64  `json:"seller_id"`
	Name           string `json:"name"`
	SKU            string `json:"sku"`
	Seller         string `json:"seller"`
	Ordered        int    `json:"ordered"`
	Cancelled      int    `json:"cancelled"`
	Refunded       int    `json:"refunded"`
	OriginalCents  int64  `json:"original_cents"`
	PaidCents      int64  `json:"paid_cents"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}
