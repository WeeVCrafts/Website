package viewmodels

// LegacyPage carries transport data for routes that still use the older
// application handlers. It exists only as a Templ adapter during migration;
// business rules remain in the application services.
type LegacyPage struct {
	Mode, Brand, Tagline, Description, Title, Error, Notice, FormEmail, CSRFToken string
	Customer                                                                  CustomerPage
	Products                                                                  []Product
	Product                                                                   Product
	Lines                                                                     []string
	CartCount                                                                 int
	ManagedProduct                                                            LegacyProductDraft
	Staff                                                                     []LegacyStaffMember
}

type LegacyProductDraft struct {
	ID int64
	Name, Description, SKU, DeliveryLabel string
	PriceCents, CompareAtCents int64
}

type LegacyStaffMember struct {
	Email, DisplayName, Status string
	Permissions []string
}
