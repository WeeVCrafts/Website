package commerce

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

type ShippingService struct {
	repository      ports.ShippingRepository
	gateway         ports.ShippingGateway
	environment     string
	pickups         map[int64]string
	bookingsEnabled bool
}

func NewShippingService(repository ports.ShippingRepository, gateway ports.ShippingGateway, environment string, pickups map[int64]string) *ShippingService {
	return &ShippingService{repository: repository, gateway: gateway, environment: environment, pickups: pickups, bookingsEnabled: true}
}

func (s *ShippingService) SetBookingsEnabled(enabled bool) { s.bookingsEnabled = enabled }
func (s *ShippingService) BookingsEnabled() bool           { return s.bookingsEnabled }

func (s *ShippingService) PickupLocation(seller int64) string { return s.pickups[seller] }
func (s *ShippingService) Environment() string                { return s.environment }

func (s *ShippingService) ScheduleCheck(ctx context.Context, orderID int64, reference, awb string) error {
	return s.repository.ScheduleShipmentCheck(ctx, s.environment, orderID, reference, awb)
}

func (s *ShippingService) Get(ctx context.Context, actor, seller int64, number string) (ports.Shipment, error) {
	return s.repository.SellerShipment(ctx, actor, seller, number, s.environment)
}

func (s *ShippingService) CourierOptions(ctx context.Context, shipment ports.Shipment) ([]ports.CourierOption, error) {
	if shipment.ProviderOrderID <= 0 || shipment.PendingAction != "" || shipment.AWB != "" {
		return nil, ports.ErrInvalidState
	}
	gateway, ok := s.gateway.(ports.CourierAvailabilityGateway)
	if !ok {
		return nil, ports.ErrShippingUnavailable
	}
	return gateway.CourierOptions(ctx, shipment)
}

func (s *ShippingService) Act(ctx context.Context, actor, seller int64, number, action string, parcel ports.Parcel, courier int64) error {
	if len(number) == 0 || len(number) > 120 || seller <= 0 || actor <= 0 {
		return ports.ErrShippingInput
	}
	if !s.bookingsEnabled && (action == "create" || action == "assign" || action == "pickup") {
		return ports.ErrInvalidState
	}
	if action == "refresh" {
		current, err := s.Get(ctx, actor, seller, number)
		if err != nil {
			return err
		}
		if current.PendingAction != "" && !current.CanReconcile {
			return ports.ErrShippingPending
		}
		return s.refresh(ctx, current)
	}
	switch action {
	case "create":
		if parcel.WeightGrams < 100 || parcel.WeightGrams > 50000 || parcel.LengthCM < 1 || parcel.LengthCM > 200 || parcel.WidthCM < 1 || parcel.WidthCM > 200 || parcel.HeightCM < 1 || parcel.HeightCM > 200 || s.pickups[seller] == "" {
			return ports.ErrShippingInput
		}
	case "assign":
		if courier < 0 || courier > 100000000 {
			return ports.ErrShippingInput
		}
		if _, supported := s.gateway.(ports.CourierAvailabilityGateway); supported && courier > 0 {
			shipment, err := s.Get(ctx, actor, seller, number)
			if err != nil {
				return err
			}
			if shipment.AWB == "" {
				options, err := s.CourierOptions(ctx, shipment)
				if err != nil {
					if errors.Is(err, ports.ErrShippingPending) {
						return ports.ErrShippingUnavailable
					}
					return err
				}
				available := false
				for _, option := range options {
					if option.ID == courier {
						available = true
						break
					}
				}
				if !available {
					return ports.ErrShippingInput
				}
			}
		}
	case "pickup", "label", "manifest":
	default:
		return ports.ErrShippingInput
	}
	order, err := s.repository.ClaimShipment(ctx, actor, seller, number, s.environment, action, s.pickups[seller], parcel)
	if err != nil || order.PendingAction == "" {
		return err
	}
	current := order.Shipment
	switch action {
	case "create":
		current, err = s.gateway.Create(ctx, order)
	case "assign":
		current, err = s.gateway.Assign(ctx, current, courier)
	case "pickup":
		current, err = s.gateway.Pickup(ctx, current)
	case "label":
		current, err = s.gateway.Label(ctx, current)
	case "manifest":
		current, err = s.gateway.Manifest(ctx, current)
	}
	if err != nil && !errors.Is(err, ports.ErrShippingRejected) && !errors.Is(err, ports.ErrShippingInput) && !errors.Is(err, ports.ErrShippingUnavailable) {
		return err
	}
	confirm, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err != nil && action == "create" {
		if clearErr := s.repository.RejectShipmentCreation(confirm, s.environment, order.Shipment); clearErr != nil {
			return ports.ErrShippingPending
		}
		return err
	}
	if err != nil {
		if action == "label" || action == "manifest" {
			s.recordException(ctx, order.Shipment, "document_failed")
		}
		current = order.Shipment
	}
	if saveErr := s.repository.SaveShipment(confirm, s.environment, current, action, true); saveErr != nil {
		return ports.ErrShippingPending
	}
	return err
}

func (s *ShippingService) Reconcile(ctx context.Context) error {
	rows, err := s.repository.DueShipments(ctx, s.environment, 4)
	if err != nil {
		return err
	}
	var failures error
	for _, row := range rows {
		if ctx.Err() != nil {
			return errors.Join(failures, ctx.Err())
		}
		if err := s.refresh(ctx, row); err != nil && !errors.Is(err, ports.ErrShippingPending) {
			failures = errors.Join(failures, err)
		}
	}
	return failures
}

func (s *ShippingService) refresh(ctx context.Context, current ports.Shipment) error {
	var fresh ports.Shipment
	var err error
	if current.ProviderOrderID == 0 {
		fresh, err = s.gateway.Find(ctx, current.Reference)
	} else {
		fresh, err = s.gateway.Status(ctx, current)
	}
	if err != nil {
		return err
	}
	if fresh.Reference != current.Reference || fresh.ProviderOrderID <= 0 || fresh.ProviderShipmentID <= 0 {
		return ports.ErrShippingPending
	}
	fresh.OrderID, fresh.SellerID, fresh.RequestedBy, fresh.OrderNumber = current.OrderID, current.SellerID, current.RequestedBy, current.OrderNumber
	if s.environment == "live" && current.ProviderShipmentID == 0 && shiprocketStage(fresh.ProviderStatus) != "" {
		// Attach recovered IDs first. A later authenticated status read can apply the delivery stages.
		fresh.ProviderStatus = ""
	}
	complete := current.PendingAction == "create" && fresh.ProviderShipmentID > 0 || current.PendingAction == "assign" && fresh.AWB != "" || current.PendingAction == "pickup" && fresh.PickupReference != ""
	// A document can be requested again for the same shipment after reconciliation.
	if current.PendingAction == "cancel" {
		complete = ports.ShipmentCancelled(fresh.ProviderStatus)
		if !complete && shiprocketStage(fresh.ProviderStatus) != "" {
			s.recordException(ctx, current, "provider_exception")
			complete = true
		}
	}
	if ports.ShipmentCancelled(fresh.ProviderStatus) {
		complete = true
	}
	if current.PendingAction == "label" || current.PendingAction == "manifest" {
		s.recordException(ctx, current, "document_failed")
		complete = true
	}
	if s.environment == "live" && current.ProviderShipmentID > 0 && len(fresh.AWB) >= 3 && len(fresh.AWB) <= 120 && len(fresh.Courier) >= 2 && len(fresh.Courier) <= 80 {
		stage := shiprocketStage(fresh.ProviderStatus)
		for step := 0; stage != "" && step < 3; step++ {
			if err := s.repository.ApplyShipmentStage(ctx, s.environment, fresh, stage); err != nil {
				return err
			}
		}
	}
	if err := s.repository.SaveShipment(ctx, s.environment, fresh, current.PendingAction, complete); err != nil {
		return err
	}
	if current.PendingAction != "" && !complete {
		return ports.ErrShippingPending
	}
	return nil
}

func (s *ShippingService) recordException(ctx context.Context, shipment ports.Shipment, kind string) {
	if repo, ok := s.repository.(ports.ShippingOperationsRepository); ok {
		confirm, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = repo.RecordShipmentException(confirm, shipment, s.environment, kind)
	}
}

func (s *ShippingService) Cancel(ctx context.Context, actor, order, seller int64) error {
	repo, ok := s.repository.(ports.ShippingOperationsRepository)
	gateway, supported := s.gateway.(ports.ShipmentCancellationGateway)
	if !ok || !supported {
		return ports.ErrShippingUnavailable
	}
	current, err := repo.ShipmentForCancellation(ctx, actor, order, seller, s.environment)
	if err != nil {
		return err
	}
	fresh, err := s.gateway.Status(ctx, current)
	if err != nil {
		return err
	}
	if fresh.ProviderOrderID != current.ProviderOrderID || fresh.ProviderShipmentID != current.ProviderShipmentID || fresh.Reference != current.Reference {
		return ports.ErrShippingPending
	}
	if ports.ShipmentCancelled(fresh.ProviderStatus) {
		return s.repository.SaveShipment(ctx, s.environment, fresh, "", true)
	}
	if !ports.ShippingPrePickup(fresh.ProviderStatus) {
		return ports.ErrInvalidState
	}
	fresh.PreviousProviderStatus = current.ProviderStatus
	if err := repo.ClaimShipmentCancellation(ctx, actor, fresh, s.environment); err != nil {
		return err
	}
	if err := gateway.Cancel(ctx, fresh); err != nil {
		if errors.Is(err, ports.ErrShippingRejected) {
			confirm, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if saveErr := s.repository.SaveShipment(confirm, s.environment, fresh, "cancel", true); saveErr == nil {
				return err
			}
		}
		return ports.ErrShippingPending
	}
	fresh.PendingAction = "cancel"
	if err := s.refresh(ctx, fresh); err != nil {
		return ports.ErrShippingPending
	}
	return nil
}

func shiprocketStage(value string) domain.FulfillmentStage {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "SHIPPED", "IN TRANSIT", "IN-TRANSIT", "PICKED UP", "REACHED AT DESTINATION HUB":
		return domain.FulfillmentShipped
	case "OUT FOR DELIVERY":
		return domain.FulfillmentOutForDelivery
	case "DELIVERED":
		return domain.FulfillmentDelivered
	default:
		return ""
	}
}
