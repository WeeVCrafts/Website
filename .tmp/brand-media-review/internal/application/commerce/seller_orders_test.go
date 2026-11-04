package commerce

import (
	"context"
	"errors"
	"testing"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

type sellerOrderRepository struct {
	updated bool
}

func (r *sellerOrderRepository) ListSellerOrders(context.Context, int64) ([]domaincommerce.SellerOrder, error) {
	return []domaincommerce.SellerOrder{{OrderNumber: "WC-1"}}, nil
}

func (r *sellerOrderRepository) UpdateSellerFulfillment(context.Context, int64, int64, string, string, string, string, string) error {
	r.updated = true
	return nil
}

func TestFulfilmentOnlySellerOwnerCanReadAndWrite(t *testing.T) {
	repository := &sellerOrderRepository{}
	checker := roleChecker{roles: map[int64]map[domainidentity.Role]bool{10: {domainidentity.RoleSeller: true}, 11: {domainidentity.Role("seller_staff"): true}}, permissions: map[int64]map[string]bool{11: {"ORDER_READ": true, "ORDER_FULFILL": true}}}
	service := NewSellerOrderService(repository, checker)
	if _, err := service.List(context.Background(), 11); !errors.Is(err, ports.ErrForbidden) {
		t.Fatal("staff read authorized", err)
	}
	if err := service.UpdateFulfillment(context.Background(), 11, 3, "WC-1", "processing", "", "", "picked"); !errors.Is(err, ports.ErrForbidden) {
		t.Fatal("staff write authorized", err)
	}
	if items, err := service.List(context.Background(), 10); err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	if err := service.UpdateFulfillment(context.Background(), 10, 3, "WC-1", "processing", "", "", "picked"); err != nil {
		t.Fatal(err)
	}
}
func TestSellerOrderServiceValidatesFulfillment(t *testing.T) {
	service := NewSellerOrderService(&sellerOrderRepository{}, roleChecker{roles: map[int64]map[domainidentity.Role]bool{10: {domainidentity.RoleSeller: true}}})
	for _, status := range []string{"", "pending", "cancelled"} {
		if err := service.UpdateFulfillment(context.Background(), 10, 3, "WC-1", status, "", "", "note"); !errors.Is(err, ErrInvalidSellerFulfillment) {
			t.Fatalf("status %q returned %v", status, err)
		}
	}
	if err := service.UpdateFulfillment(context.Background(), 10, 3, "WC-1", "shipped", "", "", "packed"); !errors.Is(err, ErrInvalidSellerFulfillment) {
		t.Fatalf("shipped without tracking returned %v", err)
	}
}
