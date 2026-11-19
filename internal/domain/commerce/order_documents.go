package commerce

type OrderPackageItem struct {
	Name     string `json:"name"`
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}
type OrderPackageSummary struct {
	ID        int64              `json:"id"`
	SellerID  int64              `json:"seller_id"`
	Reference string             `json:"reference"`
	Status    string             `json:"status"`
	Courier   string             `json:"courier"`
	AWB       string             `json:"awb"`
	Items     []OrderPackageItem `json:"items"`
}
type OrderDocumentReference struct {
	ID       int64  `json:"id"`
	SellerID int64  `json:"seller_id"`
	Kind     string `json:"kind"`
	Number   string `json:"number"`
	URL      string `json:"url"`
}
type OrderRequestSummary struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	AmountCents int64  `json:"amount_cents"`
}
