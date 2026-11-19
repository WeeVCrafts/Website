package ports

// Setup choices contain display labels, never provider credentials or customer contacts.
type MarketplaceSetupOption struct {
	ID, SellerID int64
	Label        string
}

type MarketplaceSetupQuery struct {
	SellerID, VariantID, SellerBefore, VariantBefore, CustomerBefore, LaunchBefore int64
	SellerSearch, ProductSearch, CustomerSearch, LaunchSearch                      string
}

type MarketplaceSetup struct {
	Query                                             MarketplaceSetupQuery
	Sellers, Variants, Customers, LaunchVariants      []MarketplaceSetupOption
	NextSeller, NextVariant, NextCustomer, NextLaunch int64
	Invoice                                           SellerInvoiceProfile
	InvoiceSaved, InvoiceReady, PickupReady, TaxSaved bool
	HSN, ReturnClass, PickupLocation, PickupPIN       string
	TaxBasisPoints                                    int
	Product, Seller, SKU                              string
	ProductID                                         int64
	Selected                                          *ShippingReadiness
}
