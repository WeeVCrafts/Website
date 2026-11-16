package ports

import (
	"context"
	"time"
)

type ReturnShippingOrder struct {
	ShippingOrder
	ClaimID       int64
	PendingAction string
}
type ReturnShippingGateway interface {
	CreateReturn(context.Context, ReturnShippingOrder) (Shipment, error)
	FindReturn(context.Context, string) (Shipment, error)
}
type ReturnShippingStore interface {
	ReturnShippingOrder(context.Context, int64) (ReturnShippingOrder, error)
	StartReturnShipping(context.Context, int64) error
	RejectReturnShipping(context.Context, int64) error
	SaveReturnShipping(context.Context, int64, Shipment) error
}

type PolicySettings struct {
	PublishedAt                               *time.Time
	Version, ShippingMode                     string
	PilotUserID, PilotVariantID, PilotOrderID int64
}
type PolicyClaim struct {
	ID, ItemID, SellerID, AmountCents, FeeCents, ReplacementPackageID                                           int64
	Quantity                                                                                                    int
	Kind, Resolution, Status, Reason, EvidenceException, InspectionNote, ReturnAWB, ReturnCourier, ReturnStatus string
	SubmittedAt                                                                                                 time.Time
	Evidence                                                                                                    []ClaimEvidence
}
type ClaimEvidence struct {
	ID                    int64
	Filename, ContentType string
}
type StoreCreditGrant struct {
	ID, AmountCents, RemainingCents int64
	ExpiresAt                       time.Time
}

type ShippingReadiness struct {
	VariantID, SellerID                             int64
	Product, SKU, Seller                            string
	WeightGrams, LengthCM, WidthCM, HeightCM        int
	PickupReady, InvoiceReady, TaxReady, Classified bool
}
