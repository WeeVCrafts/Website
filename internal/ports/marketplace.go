package ports

import (
	"context"
	"encoding/json"
	"time"
)

type MarketplaceLine struct {
	ID, SellerID                  int64
	Name, SKU                     string
	Quantity, Cancelled, Refunded int
	PaidCents                     int64
	CanCancel, CanReturn          bool
}
type MarketplacePackage struct {
	ID int64
	Shipment
	Status         string
	DeliveryMethod string
	DeliveredAt    *time.Time
	Items          []MarketplacePackageItem
}
type MarketplacePackageItem struct {
	OrderItemID int64 `json:"-"`
	Name, SKU   string
	Quantity    int
}
type MarketplaceSellerOrder struct {
	SellerID, Revision           int64
	Name, Status, DeliveryMethod string
	SettlementStatus             string
}
type MarketplaceRefund struct {
	ID, SellerID, ItemID, AmountCents, ShippingCents int64
	Quantity                                         int
	Kind, Status, Reason, ProviderRefundID           string
}
type MarketplaceCase struct {
	ID, OrderID, SellerID, PackageID int64
	Number, Kind, Status, Reason     string
	Assignee, NextAction             string
	CreatedAt                        time.Time
}

type FulfillmentQueueItem struct {
	Number, Seller, Issue string
}
type MarketplaceOrderSummary struct {
	ID                          int64
	Number, Status, SellerStage string
}
type MarketplaceJobSummary struct {
	ID                      int64
	Kind, Status, LastError string
	Attempts                int
}
type MarketplaceReturn struct {
	ID, ItemID, SellerID              int64
	Quantity                          int
	Status, Reason, Carrier, Tracking string
}
type MarketplaceInvoice struct {
	ID, SellerID int64
	Number       string
	Snapshot     json.RawMessage
	IssuedAt     time.Time
	PDF          []byte
	SHA256       string
}
type SellerInvoiceProfile struct {
	SellerID                                                   int64
	LegalName, Address, StateCode, GSTIN, Registration, Prefix string
}
type InvoiceLine struct {
	ItemID                            int64
	Name, SKU, HSN                    string
	Quantity                          int
	RateBasisPoints                   int64
	PaidCents, TaxableCents, TaxCents int64
}
type InvoiceDocument struct {
	RelatedNumber                                                       string
	Number, Date, Currency, Kind                                        string
	Seller                                                              SellerInvoiceProfile
	CustomerName, CustomerAddress, CustomerStateCode                    string
	Lines                                                               []InvoiceLine
	TotalCents, TaxableCents, TaxCents, CGSTCents, SGSTCents, IGSTCents int64
}
type MarketplaceView struct {
	NewReturnPolicy                         bool
	Claims                                  []PolicyClaim
	CreditAvailableCents                    int64
	CreditGrants                            []StoreCreditGrant
	Policy                                  PolicySettings
	OrderID                                 int64
	Number, Status, Payment, Role, Currency string
	WorkflowVersion                         int
	Lines                                   []MarketplaceLine
	Sellers                                 []MarketplaceSellerOrder
	Packages                                []MarketplacePackage
	Refunds                                 []MarketplaceRefund
	Cases                                   []MarketplaceCase
	Queue                                   []FulfillmentQueueItem
	Invoices                                []MarketplaceInvoice
	CreditNotes                             []MarketplaceInvoice
	Orders                                  []MarketplaceOrderSummary
	NextOrderID, NextCaseID                 int64
	Jobs                                    []MarketplaceJobSummary
	Returns                                 []MarketplaceReturn
}
type MarketplaceCommand struct {
	InspectionPassed                                             bool
	Kind, Resolution, EvidenceException, ReturnClass, QCEvidence string
	Action, Number, Reason, Key, Status                          string
	SellerID, ItemID, PackageID, RefundID, CaseID, Revision      int64
	Quantity                                                     int
	Parcel
	Items             map[int64]int
	Restock           bool
	Carrier, Tracking string
	PreferredDate     string
}
type MarketplaceJob struct {
	ID, ResourceID int64
	Kind           string
	Attempts       int
}
type MarketplaceStore interface {
	View(context.Context, int64, string) (MarketplaceView, error)
	Act(context.Context, int64, MarketplaceCommand) error
	DueJobs(context.Context, int) ([]MarketplaceJob, error)
	JobDispatched(context.Context, int64) error
	ClaimJob(context.Context, int64) (MarketplaceJob, bool, error)
	FinishJob(context.Context, MarketplaceJob, error) error
	ShippingOrder(context.Context, int64) (ShippingOrder, error)
	SavePackage(context.Context, int64, Shipment, string) error
	StartPackageAction(context.Context, int64, string) error
	RejectPackageAction(context.Context, int64, string, int64) error
	QueueMaintenance(context.Context) error
	IssueInvoices(context.Context, int64) error
	IssueCreditNote(context.Context, int64) error
	NotifySellers(context.Context, int64) error
	RefundRecord(context.Context, int64) (PaymentRefundRecord, bool, error)
	SavePartialRefund(context.Context, int64, PaymentRefund) error
	CancellationPackages(context.Context, int64) ([]MarketplacePackage, error)
	CompleteCancellation(context.Context, int64) error
	NDRAction(context.Context, int64) (Shipment, string, string, string, bool, error)
	ConfirmNDRAction(context.Context, int64) error
}
type NDRGateway interface {
	ActNDR(context.Context, Shipment, string, string, string) error
}
