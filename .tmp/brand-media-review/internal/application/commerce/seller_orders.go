package commerce

import (
	"context"
	"errors"
	"strings"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

var ErrInvalidSellerFulfillment = errors.New("seller fulfillment is invalid")
var ErrStaleSellerFulfillment = errors.New("seller fulfillment has changed")

type SellerOrderService struct {
	repository  ports.SellerOrderRepository
	roles       ports.RoleChecker
	permissions ports.SellerPermissionChecker
}

func NewSellerOrderService(repository ports.SellerOrderRepository, roles ports.RoleChecker) *SellerOrderService {
	service := &SellerOrderService{repository: repository, roles: roles}
	if checker, ok := roles.(ports.SellerPermissionChecker); ok {
		service.permissions = checker
	}
	return service
}

func (s *SellerOrderService) List(ctx context.Context, userID int64) ([]domaincommerce.SellerOrder, error) {
	if !s.sellerAllowed(ctx, userID, domaincommerce.SellerPermissionOrderRead) {
		return nil, ports.ErrForbidden
	}
	return s.repository.ListSellerOrders(ctx, userID)
}

func (s *SellerOrderService) UpdateFulfillment(ctx context.Context, userID, sellerID int64, orderNumber, status, carrier, trackingNumber, note string) error {
	return s.UpdateFulfillmentWithRevision(ctx, userID, sellerID, orderNumber, status, carrier, trackingNumber, note, -1)
}

func (s *SellerOrderService) UpdateFulfillmentWithRevision(ctx context.Context, userID, sellerID int64, orderNumber, status, carrier, trackingNumber, note string, expectedRevision int64) error {
	if !s.sellerAllowed(ctx, userID, domaincommerce.SellerPermissionOrderFulfill) {
		return ports.ErrForbidden
	}
	if sellerID <= 0 {
		return ErrInvalidSellerFulfillment
	}
	orderNumber = strings.TrimSpace(orderNumber)
	status = strings.ToLower(strings.TrimSpace(status))
	carrier = strings.TrimSpace(carrier)
	trackingNumber = strings.TrimSpace(trackingNumber)
	note = strings.TrimSpace(note)
	if orderNumber == "" || len(orderNumber) > 120 || len(carrier) > 80 || len(trackingNumber) > 120 || len(note) > 500 {
		return ErrInvalidSellerFulfillment
	}
	stage, ok := domaincommerce.ParseFulfillmentStage(status)
	if !ok || stage == domaincommerce.FulfillmentPending {
		return ErrInvalidSellerFulfillment
	}
	if stage.Index() >= domaincommerce.FulfillmentShipped.Index() && (len(carrier) < 2 || len(trackingNumber) < 3) {
		return ErrInvalidSellerFulfillment
	}
	if revised, ok := s.repository.(ports.SellerFulfillmentRevisionRepository); ok && expectedRevision >= 0 {
		return revised.UpdateSellerFulfillmentWithRevision(ctx, userID, sellerID, orderNumber, status, carrier, trackingNumber, note, expectedRevision)
	}
	return s.repository.UpdateSellerFulfillment(ctx, userID, sellerID, orderNumber, status, carrier, trackingNumber, note)
}

func (s *SellerOrderService) sellerAllowed(ctx context.Context, userID int64, permission string) bool {
	if userID <= 0 || s.repository == nil {
		return false
	}
	if s.roles != nil {
		allowed, err := s.roles.HasAnyRole(ctx, userID, domainidentity.RoleSeller)
		if err == nil && allowed {
			return true
		}
	}
	return false
}
