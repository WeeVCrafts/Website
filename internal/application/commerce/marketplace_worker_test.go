package commerce

import (
	"context"
	"errors"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

type marketplaceMemory struct {
	ports.MarketplaceStore
	order    ports.ShippingOrder
	attempts int
}

func (s *marketplaceMemory) ClaimJob(context.Context, int64) (ports.MarketplaceJob, bool, error) {
	s.attempts++
	return ports.MarketplaceJob{ID: 1, ResourceID: 1, Kind: "book_package", Attempts: s.attempts}, true, nil
}
func (s *marketplaceMemory) FinishJob(context.Context, ports.MarketplaceJob, error) error { return nil }
func (s *marketplaceMemory) ShippingOrder(context.Context, int64) (ports.ShippingOrder, error) {
	return s.order, nil
}
func (s *marketplaceMemory) StartPackageAction(_ context.Context, _ int64, action string) error {
	if s.order.PendingAction != "" {
		return ports.ErrShippingPending
	}
	s.order.PendingAction = action
	return nil
}
func (s *marketplaceMemory) SavePackage(_ context.Context, _ int64, v ports.Shipment, pending string) error {
	s.order.Shipment = v
	s.order.PendingAction = pending
	return nil
}
func (s *marketplaceMemory) RejectPackageAction(_ context.Context, _ int64, _ string, courier int64) error {
	s.order.PendingAction = ""
	if courier > 0 {
		s.order.RejectedCourierIDs = append(s.order.RejectedCourierIDs, courier)
	}
	return nil
}

type marketplaceCourier struct {
	creates, pickups int
	assigned         []int64
	rejectFirst      bool
}

func (g *marketplaceCourier) Create(_ context.Context, o ports.ShippingOrder) (ports.Shipment, error) {
	g.creates++
	s := o.Shipment
	s.ProviderOrderID = 10
	s.ProviderShipmentID = 20
	s.ProviderStatus = "NEW"
	return s, nil
}
func (g *marketplaceCourier) Find(context.Context, string) (ports.Shipment, error) {
	return ports.Shipment{}, ports.ErrShippingPending
}
func (g *marketplaceCourier) Status(_ context.Context, s ports.Shipment) (ports.Shipment, error) {
	return s, nil
}
func (g *marketplaceCourier) CourierOptions(context.Context, ports.Shipment) ([]ports.CourierOption, error) {
	return []ports.CourierOption{{ID: 10, Name: "Recommended", RateCents: 1000, Recommended: true}, {ID: 11, Name: "Alternative", RateCents: 500}}, nil
}
func (g *marketplaceCourier) Assign(_ context.Context, s ports.Shipment, id int64) (ports.Shipment, error) {
	g.assigned = append(g.assigned, id)
	if g.rejectFirst && id == 10 {
		return s, ports.ErrShippingRejected
	}
	s.AWB = "confirmed-waybill"
	return s, nil
}
func (g *marketplaceCourier) Pickup(_ context.Context, s ports.Shipment) (ports.Shipment, error) {
	g.pickups++
	s.PickupReference = "confirmed-pickup"
	return s, nil
}
func (g *marketplaceCourier) Label(_ context.Context, s ports.Shipment) (ports.Shipment, error) {
	s.LabelURL = "https://shiprocket.in/fixture-label.pdf"
	return s, nil
}
func (g *marketplaceCourier) Manifest(_ context.Context, s ports.Shipment) (ports.Shipment, error) {
	s.ManifestURL = "https://shiprocket.in/fixture-manifest.pdf"
	return s, nil
}
func TestMarketplaceCourierRejectionTriesAlternativeWithoutNewShipment(t *testing.T) {
	store := &marketplaceMemory{order: ports.ShippingOrder{Shipment: ports.Shipment{Reference: "stable-reference"}}}
	gateway := &marketplaceCourier{rejectFirst: true}
	processor := &MarketplaceProcessor{Store: store, Shipping: gateway, BookingsEnabled: true}
	if err := processor.Process(t.Context(), 1); !errors.Is(err, ports.ErrShippingRejected) {
		t.Fatal(err)
	}
	if err := processor.Process(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	if gateway.creates != 1 || len(gateway.assigned) != 2 || gateway.assigned[0] != 10 || gateway.assigned[1] != 11 || gateway.pickups != 1 {
		t.Fatalf("provider side effects %+v", gateway)
	}
}
func TestMarketplaceDoesNotSchedulePickupAfterCollection(t *testing.T) {
	store := &marketplaceMemory{order: ports.ShippingOrder{Shipment: ports.Shipment{Reference: "stable-reference", ProviderOrderID: 10, ProviderShipmentID: 20, ProviderStatus: "IN TRANSIT", AWB: "collected-waybill"}}}
	gateway := &marketplaceCourier{}
	processor := &MarketplaceProcessor{Store: store, Shipping: gateway, BookingsEnabled: true}
	if err := processor.Process(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	if gateway.pickups != 0 || gateway.creates != 0 || len(gateway.assigned) != 0 {
		t.Fatalf("collected parcel was booked again %+v", gateway)
	}
}
