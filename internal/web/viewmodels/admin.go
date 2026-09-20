package viewmodels

import domainidentity "github.com/wecratfs/commerce/internal/domain/identity"

// AdminPage is the view model for the live marketplace operations workspace.
type AdminPage struct {
	Active, NavActive, Workspace, Title, Subtitle, Query, Range, Notice string
	PortalRoot, PortalLabel, RoleLabel, DisplayName                     string
	CSRFToken                                                           string
	CanFinance, CanSecurity, IsFinance, IsOperations                    bool
	Stats                                                               []AdminStat
	Bars                                                                []AdminBar
	Donut                                                               []AdminDonut
	Sellers                                                             []AdminSeller
	Products                                                            []AdminProduct
	Inventory                                                           []AdminInventory
	Orders                                                              []AdminOrder
	Returns                                                             []AdminReturn
	Settlements                                                         []AdminSettlement
	Transactions                                                        []AdminTransaction
	SupportCases                                                        []AdminSupportCase
	Campaigns                                                           []AdminCampaign
	AuditEntries                                                        []AdminAuditEntry
	Configs                                                             []AdminConfig
	RoleAssignments                                                     []domainidentity.RoleAssignment
}

type AdminStat struct {
	Icon, Value, Label, Delta, Period string
	Positive                          bool
}

type AdminBar struct {
	Label, Value, Height, Color string
}

type AdminDonut struct {
	Label, Value, Color string
}

type AdminSeller struct {
	ID                                               string
	Name, Location, Category, Applied, Status, Image string
	Products, Orders, Revenue, Rating                string
}

type AdminProduct struct {
	ID, Name, SKU, Seller, Category, Price, Submitted, Status, Image string
}

type AdminInventory struct {
	Name, SKU, Stock, Reorder, Reserved, Available, Location, Value, Status, Image string
}

type AdminOrder struct {
	Number, Date, Customer, Items, Value, Type, Destination, Fulfillment, Delivery, RTO string
}

type AdminReturn struct {
	ID, Number, Customer, Product, Reason, Date, Status, Amount, Image string
}

type AdminSettlement struct {
	Seller, Orders, Gross, Commission, Amount, Status, Date, Image string
}

type AdminTransaction struct {
	Date, Type, Reference, Description, Amount, Status string
}

type AdminSupportCase struct {
	Case, Customer, Subject, Priority, Date, Status string
}

type AdminCampaign struct {
	Name, Type, Start, End, Status, Channel, Reach, Revenue string
}

type AdminAuditEntry struct {
	Date, User, Action, Entity, Details, IP string
}

type AdminConfig struct {
	Icon, Name, Value, Detail string
}

// SellerAdminPage is the view model for the live seller-scoped workspace.
type SellerAdminPage struct {
	Active, Workspace, Title, Subtitle, Query, Range, Tab, Notice      string
	DisplayName, UserEmail, SellerName, RoleLabel, Initials, CSRFToken string
	CanProducts, CanProductWrite, CanInventory, CanInventoryWrite      bool
	CanOrders, CanFulfill, CanReturns, CanReports, CanTeam             bool
	Stats                                                              []SellerAdminStat
	Products                                                           []SellerAdminProduct
	Inventory                                                          []SellerAdminInventory
	Orders                                                             []SellerAdminOrder
	Returns                                                            []SellerAdminReturn
	Ledger                                                             []SellerAdminLedger
	Settlements                                                        []SellerAdminSettlement
	AnalyticsBars                                                      []SellerAdminBar
	Team                                                               []SellerTeamMember
}

type SellerAdminStat struct {
	Icon, Value, Label, Delta, Period, Tone string
	Positive                                bool
}

type SellerAdminBar struct {
	Label, Current, Previous string
}

type SellerAdminProduct struct {
	ID, Name, SKU, Category, Price, Variants, Stock, Status, Availability, Image string
}

type SellerAdminInventory struct {
	VariantID, Name, SKU, Variant, Location, Available, Reserved, Reorder, Status, Image string
}

type SellerAdminOrder struct {
	SellerID, Number, Date, Customer, Items, Value, Type, Payment, Status, Destination, Image string
}

type SellerAdminReturn struct {
	Number, Order, Customer, Product, Reason, Date, Status, Amount, Image string
}

type SellerAdminLedger struct {
	Number, Date, Sale, Commission, Adjustments, Status, Net string
}

type SellerAdminSettlement struct {
	Number, Amount, Orders, Status, Date string
}

type SellerTeamMember struct {
	UserID, Name, Email, Role, Status, LastActive, Permissions, Image string
}

// SupportPortalPage is the support-agent-scoped live workspace. Customer
// contact and transaction fields are intentionally minimized or masked to
// reflect the PRD's support data boundary.
type SupportPortalPage struct {
	Active, Title, Subtitle, Query, Range, Tab, Notice string
	DisplayName, RoleLabel, Initials                   string
	CSRFToken                                          string
	Stats                                              []SupportPortalStat
	Cases                                              []SupportPortalCase
	Conversations                                      []SupportPortalConversation
	Directory                                          []SupportPortalDirectoryEntry
	Timeline                                           []SupportPortalTimelineEntry
}

type WorkspaceChoice struct {
	Role, Label, Path string
}

type WorkspacePage struct {
	DisplayName string
	CSRFToken   string
	Workspaces  []WorkspaceChoice
}

type SellerProductForm struct {
	ID, Name, Slug, BrandSlug, BrandName, CategorySlug, CategoryName  string
	Description, PriceCents, SKU, InitialStock, DeliveryLabel, Action string
}

type SupportPortalStat struct {
	Icon, Value, Label, Delta, Period, Tone string
	Positive                                bool
}

type SupportPortalCase struct {
	DatabaseID                                                                                                                  int64
	CustomerID                                                                                                                  int64
	ID, Subject, Customer, CustomerInitials, Category, Priority, Created, Status, Age, Seller, Order, Avatar, Assignee, Channel string
	OrderStatus, OrderValue, PaymentStatus                                                                                      string
}

type SupportPortalConversation struct {
	Author, Initials, Role, Message, Time, Tone, Image string
}

type SupportPortalDirectoryEntry struct {
	Name, EmailPhone, Type, Location, Orders, Cases, Status, LastActive, Image string
}

type SupportPortalTimelineEntry struct {
	Time, Title, Detail, Tone string
}
