package ports

import (
	"context"
	"errors"
	"time"

	"github.com/wecratfs/commerce/internal/domain/commerce"
)

var (
	ErrShippingInput             = errors.New("invalid shipment details")
	ErrShippingPending           = errors.New("shipment operation is awaiting provider confirmation")
	ErrShippingRejected          = errors.New("shipping provider rejected the request")
	ErrCancellationAfterHandover = errors.New("courier handover was confirmed before cancellation")
	ErrShippingUnavailable       = errors.New("shipping provider unavailable")
	ErrShippingParcelMissing     = errors.New("shipping parcel data is missing or exceeds limits")
)

type Parcel = commerce.ParcelDimensions

type Shipment struct {
	RejectedCourierIDs                                                     []int64
	OrderID, SellerID, RequestedBy, ProviderOrderID, ProviderShipmentID    int64
	OrderNumber, Reference, PickupLocation                                 string
	PendingAction, ProviderStatus, AWB, Courier, PickupReference, LabelURL string
	ManifestURL, PreviousProviderStatus                                    string
	CanReconcile, ManualDispatchAllowed                                    bool
	DeliveryPostalCode                                                     string
	Parcel
}

type CourierOption struct {
	ID            int64
	Name          string
	EstimatedRate string
	EstimatedDays string
	RateCents     int64
	Recommended   bool
}

type ShippingOrder struct {
	Shipment
	PlacedAt      time.Time
	Email         string
	Address       commerce.AddressInput
	Items         []commerce.SellerOrderItem
	DiscountCents int64
}

type ShippingGateway interface {
	Create(context.Context, ShippingOrder) (Shipment, error)
	Find(context.Context, string) (Shipment, error)
	Status(context.Context, Shipment) (Shipment, error)
	Assign(context.Context, Shipment, int64) (Shipment, error)
	Pickup(context.Context, Shipment) (Shipment, error)
	Label(context.Context, Shipment) (Shipment, error)
	Manifest(context.Context, Shipment) (Shipment, error)
}

type CourierAvailabilityGateway interface {
	CourierOptions(context.Context, Shipment) ([]CourierOption, error)
}

type DeliveryEstimateGateway interface {
	DeliveryEstimate(context.Context, string, string, Parcel) (available bool, days int, err error)
}

type PricedDeliveryGateway interface {
	DeliveryRate(context.Context, string, string, Parcel) (CourierOption, error)
}

type ProductDeliveryResult struct {
	State       string `json:"state"`
	TransitDays int    `json:"transit_days,omitempty"`
}

type ProductDeliveryReader interface {
	ProductDelivery(ctx context.Context, slug string, variant int64, quantity int, pin string) (ProductDeliveryResult, error)
}

type CheckoutDeliveryChecker interface {
	DeliveryAvailable(context.Context, string, string, Parcel) (bool, error)
}

type ShippingRepository interface {
	SellerShipment(context.Context, int64, int64, string, string) (Shipment, error)
	ClaimShipment(context.Context, int64, int64, string, string, string, string, Parcel) (ShippingOrder, error)
	SaveShipment(context.Context, string, Shipment, string, bool) error
	RejectShipmentCreation(context.Context, string, Shipment) error
	ApplyShipmentStage(context.Context, string, Shipment, commerce.FulfillmentStage) error
	DueShipments(context.Context, string, int) ([]Shipment, error)
	ScheduleShipmentCheck(context.Context, string, int64, string, string) error
}

type ShipmentCancellationGateway interface {
	Cancel(context.Context, Shipment) error
}

type ShippingOperationsRepository interface {
	ShipmentForCancellation(context.Context, int64, int64, int64, string) (Shipment, error)
	ClaimShipmentCancellation(context.Context, int64, Shipment, string) error
	RecordShipmentException(context.Context, Shipment, string, string) error
}

type DeliveryException struct {
	OrderID, SellerID, AssigneeID     int64
	OrderNumber, Kind, ShipmentStatus string
	OpenedAt                          time.Time
	CanCancel, CanResolve             bool
}

func ShipmentCancelled(status string) bool {
	return status == "CANCELLED" || status == "CANCELED"
}

func ShippingPrePickup(status string) bool {
	switch status {
	case "NEW", "PROCESSING", "PENDING", "READY TO SHIP", "AWB ASSIGNED", "LABEL GENERATED", "PICKUP SCHEDULED", "PICKUP QUEUED", "MANIFEST GENERATED", "PICKUP RESCHEDULED", "SHIPMENT BOOKED":
		return true
	}
	return false
}
