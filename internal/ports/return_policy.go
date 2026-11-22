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
	Version, ShippingMode, LiveCatalogScope   string
	LiveCatalogSellerID                       int64
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
	ProductID                                       int64
	Product, SKU, Seller                            string
	WeightGrams, LengthCM, WidthCM, HeightCM        int
	PickupReady, InvoiceReady, TaxReady, Classified bool
	Stock                                           int
	Selected                                        bool
	LiveCheckout                                    bool
	LiveBlocker                                     string
}

func (r ShippingReadiness) Ready() bool {
	return r.PickupReady && r.InvoiceReady && r.TaxReady && r.Classified && r.Stock > 0 && r.ParcelReady()
}

func (r ShippingReadiness) ParcelReady() bool {
	return r.WeightGrams >= 100 && r.WeightGrams <= 50000 &&
		r.LengthCM >= 1 && r.LengthCM <= 200 && r.WidthCM >= 1 && r.WidthCM <= 200 &&
		r.HeightCM >= 1 && r.HeightCM <= 200
}
