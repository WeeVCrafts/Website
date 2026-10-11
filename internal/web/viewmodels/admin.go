package viewmodels

import domainidentity "github.com/wecratfs/commerce/internal/domain/identity"

// AdminPage is the view model for the live marketplace operations workspace.
type AdminPage struct {
	Pagination                                                                       PortalPagination
	ActorID                                                                          int64
	Active, NavActive, Workspace, Title, Subtitle, Query, Range, Notice, DataWarning string
	PortalRoot, PortalLabel, RoleLabel, DisplayName                                  string
	SettlementRoot                                                                   string
	CSRFToken                                                                        string
	CanFinance, CanSecurity, IsFinance, IsOperations, CanManageCatalog               bool
	Stats                                                                            []AdminStat
	Bars                                                                             []AdminBar
	Donut                                                                            []AdminDonut
	Sellers                                                                          []AdminSeller
	Products                                                                         []AdminProduct
	Inventory                                                                        []AdminInventory
	Orders                                                                           []AdminOrder
	Returns                                                                          []AdminReturn
	Settlements                                                                      []AdminSettlement
	Transactions                                                                     []AdminTransaction
	SupportCases                                                                     []AdminSupportCase
	Reviews                                                                          []AdminReview
	Campaigns                                                                        []AdminCampaign
	AuditEntries                                                                     []AdminAuditEntry
	Configs                                                                          []AdminConfig
	RoleAssignments                                                                  []domainidentity.RoleAssignment
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
	SearchSKUs                                                       string
}

type AdminProductEditor struct {
	ID, SellerID, SellerName, Name, Slug, BrandSlug, BrandName, CategorySlug, CategoryName   string
	Description, SKU, PriceCents, CompareAtCents, InitialStock, DeliveryLabel, Action, Image string
	Variants                                                                                 []ProductVariant
	Media                                                                                    []ProductMedia
}

type ProductVariant struct {
	ID, Name, SKU, PriceCents, CompareAtCents, Available, Reserved, Status string
	Price, CompareAt, Discount, StockMessage                               string
	HasCompareAt, InStock                                                  bool
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
	ID, Seller, Reference, Orders, Gross, Commission, Amount, Status, Date, Period, Image string
}

type AdminTransaction struct {
	Date, Type, Reference, Description, Amount, Status string
}

type AdminSupportCase struct {
	Case, Customer, Subject, Priority, Date, Status string
}

type AdminReview struct {
	ID, ProductID, Product, Seller, Customer, Title, Body, Status, Date string
	Rating                                                              int
	VerifiedPurchase                                                    bool
}

type AdminCampaign struct {
	ID, Name, Description, Code, Type, Value, UsageLimit, UsageCount string
	Start, End, StartsOn, EndsOn, Status, Channel, Reach, Revenue    string
}

type AdminAuditEntry struct {
	Date, User, Action, Entity, Details, IP string
}

type AdminConfig struct {
	Icon, Name, Value, Detail string
}

// SellerAdminPage is the view model for the live seller-scoped workspace.
type SellerAdminPage struct {
	LocalDeliverySimulation                                                          bool
	Pagination                                                                       PortalPagination
	LedgerPagination, SettlementPagination                                           PortalPagination
	Active, Workspace, Title, Subtitle, Query, Range, Tab, Notice, DataWarning       string
	DisplayName, UserEmail, SellerName, SellerStatus, RoleLabel, Initials, CSRFToken string
	RangeLabel                                                                       string
	PublicLocation, PublicStory, PublicTechnique, PublicPortrait                     string
	LowStockThreshold                                                                int
	CanProducts, CanProductWrite, CanInventory, CanInventoryWrite                    bool
	CanOrders, CanFulfill, CanReturns, CanReports, CanTeam                           bool
	Stats                                                                            []SellerAdminStat
	TopProducts                                                                      []SellerAdminTopProduct
	LowStock                                                                         []SellerAdminInventory
	Products                                                                         []SellerAdminProduct
	Inventory                                                                        []SellerAdminInventory
	Orders                                                                           []SellerAdminOrder
	OrderDetail                                                                      *SellerAdminOrderDetail
	Returns                                                                          []SellerAdminReturn
	Ledger                                                                           []SellerAdminLedger
	Settlements                                                                      []SellerAdminSettlement
	AnalyticsBars                                                                    []SellerAdminBar
	Team                                                                             []SellerTeamMember
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
	SearchSKUs                                                                   string
	UnitCost, UnitProfit, Margin                                                 string
	HasCost                                                                      bool
}

type SellerAdminInventory struct {
	VariantID, Name, SKU, Variant, Location, Available, Reserved, Reorder, Status, Image string
	IsLowStock                                                                           bool
}

type SellerAdminOrder struct {
	SellerID, Number, Date, Customer, Items, Value, Type, Payment, Status, Destination, Image, NextStatus string
}

type SellerAdminOrderDetail struct {
	SellerID, Number, Date, Customer, Status, Payment, Carrier, TrackingNumber, Note string
	SellerName, RecipientName, Line1, Line2, City, State, PostalCode, CountryCode    string
	Subtotal, Currency                                                               string
	Items                                                                            []SellerAdminOrderItem
}

type SellerAdminOrderItem struct {
	Name, SKU, Quantity, UnitPrice, LineTotal string
}

type SellerAdminTopProduct struct {
	Name, SKU, Units, Revenue, UnitCost, UnitProfit, Margin, Image string
	HasCost                                                        bool
}

type SellerCategoryChoice struct {
	Slug, Name string
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
	ProductRead, ProductWrite, InventoryRead, InventoryWrite          bool
	OrderRead, OrderFulfill, ReturnRead, ReportRead                   bool
}

// SupportPortalPage is the support-agent-scoped live workspace. Customer
// contact and transaction fields are intentionally minimized or masked to
// reflect the PRD's support data boundary.
type SupportPortalPage struct {
	Pagination                                         PortalPagination
	DataWarning                                        string
	Active, Title, Subtitle, Query, Range, Tab, Notice string
	DisplayName, RoleLabel, Initials                   string
	CSRFToken                                          string
	Stats                                              []SupportPortalStat
	Cases                                              []SupportPortalCase
	Conversations                                      []SupportPortalConversation
	SelectedCase                                       SupportPortalCase
	Messages                                           []SupportPortalConversation
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
	ID, Name, Slug, BrandSlug, BrandName, CategorySlug, CategoryName                                         string
	Description, PriceCents, CostPriceCents, CompareAtCents, SKU, InitialStock, DeliveryLabel, Action, Image string
	Categories                                                                                               []SellerCategoryChoice
	Variants                                                                                                 []ProductVariant
	Media                                                                                                    []ProductMedia
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
	LatestReply, InitialMessage, AgentNote                                                                                      string
	Attachments                                                                                                                 []SupportPortalAttachment
}

type SupportPortalConversation struct {
	Author, Initials, Role, Message, Time, Tone, Image, TicketNumber string
	TicketID                                                         int64
	Attachments                                                      []SupportPortalAttachment
}

type SupportPortalAttachment struct {
	ID                         int64
	FileName, ContentType, URL string
	Size                       int64
}

type SupportPortalDirectoryEntry struct {
	Name, EmailPhone, Type, Location, Orders, Cases, Status, LastActive, Image string
}

type SupportPortalTimelineEntry struct {
	Time, Title, Detail, Tone string
}
