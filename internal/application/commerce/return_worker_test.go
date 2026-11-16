package commerce

import (
	"context"
	"errors"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

type reverseMemory struct {
	marketplaceMemory
	parcel     ports.ReturnShippingOrder
	rejections int
}

func (s *reverseMemory) ReturnShippingOrder(context.Context, int64) (ports.ReturnShippingOrder, error) {
	return s.parcel, nil
}
func (s *reverseMemory) StartReturnShipping(context.Context, int64) error {
	s.parcel.PendingAction = "create"
	return nil
}
func (s *reverseMemory) RejectReturnShipping(context.Context, int64) error {
	s.parcel.PendingAction = ""
	s.rejections++
	return nil
}
func (s *reverseMemory) SaveReturnShipping(_ context.Context, _ int64, v ports.Shipment) error {
	s.parcel.Shipment = v
	s.parcel.PendingAction = ""
	return nil
}

type reverseCourier struct {
	marketplaceCourier
	result       ports.Shipment
	failure      error
	posts, reads int
}

func (g *reverseCourier) CreateReturn(_ context.Context, o ports.ReturnShippingOrder) (ports.Shipment, error) {
	g.posts++
	g.result = o.Shipment
	g.result.ProviderOrderID = 10
	g.result.ProviderShipmentID = 20
	return g.result, g.failure
}
func (g *reverseCourier) FindReturn(context.Context, string) (ports.Shipment, error) {
	g.reads++
	return g.result, nil
}
func TestReversePickupLostResponseReconcilesWithoutAnotherPOST(t *testing.T) {
	s := &reverseMemory{parcel: ports.ReturnShippingOrder{ShippingOrder: ports.ShippingOrder{Shipment: ports.Shipment{Reference: "WC-RET-1"}}}}
	g := &reverseCourier{failure: ports.ErrShippingPending}
	p := &MarketplaceProcessor{Store: s, Shipping: g, BookingsEnabled: true}
	if err := p.returnPickup(t.Context(), 1); !errors.Is(err, ports.ErrShippingPending) {
		t.Fatal(err)
	}
	if s.parcel.PendingAction != "create" {
		t.Fatal("uncertain intent was cleared")
	}
	if err := p.returnPickup(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	if g.posts != 1 || g.reads != 1 || s.parcel.ProviderOrderID != 10 {
		t.Fatalf("duplicate return: %+v", g)
	}
}
func TestReversePickupDefinitiveRejectionAllowsCorrectedRetry(t *testing.T) {
	s := &reverseMemory{}
	g := &reverseCourier{failure: ports.ErrShippingRejected}
	p := &MarketplaceProcessor{Store: s, Shipping: g, BookingsEnabled: true}
	if err := p.returnPickup(t.Context(), 1); !errors.Is(err, ports.ErrShippingRejected) {
		t.Fatal(err)
	}
	if s.parcel.PendingAction != "" || s.rejections != 1 {
		t.Fatal("definitively rejected intent stuck")
	}
}
