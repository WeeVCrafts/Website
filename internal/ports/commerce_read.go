package ports

import (
	"context"
	"encoding/json"
	"github.com/wecratfs/commerce/internal/domain/commerce"
	"time"
)

// These are authorized read projections. They do not replace financial or
// fulfillment state machines, and never grant permission to perform an action.
type OrderFinancialSummary = commerce.OrderFinancialSummary
type OrderItemProgress = commerce.OrderItemProgress

type CommerceRequest struct {
	ID, OrderID, SellerID                              int64
	Kind, Number, Item, Seller, Status, Bucket, Reason string
	AmountCents                                        int64
	AmountKnown                                        bool
	CreatedAt                                          time.Time
}

func (r CommerceRequest) Key() string { return r.Kind + ":" + formatRecordID(r.ID) }

type RequestQuery struct {
	OrderNumber, Search, Bucket, Focus string
	Page                               int
}
type RequestPage struct {
	Rows               []CommerceRequest
	Total, Page, Pages int
	Counts             map[string]int
	Selected           *CommerceRequest
}

type OrderSupportReference struct{ Kind, Reference, Status, Owner, NextAction, URL string }

type CommerceOrderSummary struct {
	ID                                           int64
	Number, Role, Status, LegacyStatus, Currency string
	WorkflowVersion                              int
	CreatedAt                                    time.Time
	Financial                                    OrderFinancialSummary
	Items                                        []OrderItemProgress
	Support                                      []OrderSupportReference
	LegacyShipments                              []commerce.OrderFulfillment
}

type CommerceReadRepository interface {
	OrderForActor(context.Context, int64, string) (CommerceOrderSummary, error)
	RequestsForActor(context.Context, int64, RequestQuery) (RequestPage, error)
}

type CommerceActor struct {
	Role, Name, Email, Seller string
	SellerID                  int64
}
type SellerFulfillmentSetup struct {
	InvoiceReady, InvoiceAttested, PickupVerified bool
	InvoiceSubmissionID                           int64
	InvoiceSubmissionStatus                       string
}

type SellerReadinessOption struct {
	ShippingReadiness
	ProductStatus string
	ReviewID      int64
	ReviewStatus  string
	ImageURL      string
	HasCustomTax  bool
	ReturnClass   string
}

type SetupSubmission struct {
	ID, SellerID, VariantID, Revision               int64
	Kind, Status, Seller, Product, Note, ReviewNote string
	Payload                                         json.RawMessage
	SubmittedAt                                     *time.Time
	CreatedAt                                       time.Time
}

type SetupSubmissionInput struct {
	ID, SellerID, VariantID, ExpectedRevision int64
	ExpectedProfileRevision                   int64
	Kind, Note                                string
	Payload                                   json.RawMessage
	Submit                                    bool
	Attest                                    bool
}

type SetupConfirmation struct {
	ProfileRevision int64
	ReviewRequired  bool
	Registration    string
}

type SetupSubmissionData struct {
	LegalName        string  `json:"legal_name"`
	Address          string  `json:"address"`
	StateCode        string  `json:"state_code"`
	Registration     string  `json:"registration"`
	GSTIN            string  `json:"gstin"`
	HSN              string  `json:"hsn"`
	RateBasisPoints  int     `json:"rate_basis_points"`
	DraftRatePercent *string `json:"draft_rate_percent,omitempty"`
	ReturnClass      string  `json:"return_class"`
}
