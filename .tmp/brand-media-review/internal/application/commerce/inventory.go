package commerce

import (
	"context"
	"errors"
	"strings"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

var ErrInvalidInventoryAdjustment = errors.New("inventory adjustment is invalid")

type InventoryService struct {
	repository  ports.InventoryRepository
	roles       ports.RoleChecker
	permissions ports.SellerPermissionChecker
}

func NewInventoryService(repository ports.InventoryRepository, roles ports.RoleChecker) *InventoryService {
	service := &InventoryService{repository: repository, roles: roles}
	if checker, ok := roles.(ports.SellerPermissionChecker); ok {
		service.permissions = checker
	}
	return service
}

func (s *InventoryService) List(ctx context.Context, userID int64) ([]domaincommerce.InventoryItem, error) {
	if !s.sellerAllowed(ctx, userID, domaincommerce.SellerPermissionInventoryRead) {
		return nil, ports.ErrForbidden
	}
	return s.repository.ListSellerInventory(ctx, userID)
}

func (s *InventoryService) Adjust(ctx context.Context, userID int64, adjustment domaincommerce.InventoryAdjustment) error {
	if !s.sellerAllowed(ctx, userID, domaincommerce.SellerPermissionInventoryWrite) {
		return ports.ErrForbidden
	}
	if adjustment.VariantID <= 0 || adjustment.Delta == 0 || adjustment.Delta < -100000 || adjustment.Delta > 100000 {
		return ErrInvalidInventoryAdjustment
	}
	adjustment.Reason = strings.TrimSpace(adjustment.Reason)
	if len(adjustment.Reason) < 3 || len(adjustment.Reason) > 500 {
		return ErrInvalidInventoryAdjustment
	}
	return s.repository.AdjustSellerInventory(ctx, userID, adjustment)
}

func (s *InventoryService) sellerAllowed(ctx context.Context, userID int64, permission string) bool {
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

func (s *InventoryService) AdjustAdmin(ctx context.Context, userID int64, a domaincommerce.InventoryAdjustment) error {
	if s.roles == nil {
		return ports.ErrForbidden
	}
	allowed, err := s.roles.HasAnyRole(ctx, userID, domainidentity.RoleSuperAdmin)
	if err != nil {
		return err
	}
	if !allowed {
		return ports.ErrForbidden
	}
	a.Reason = strings.TrimSpace(a.Reason)
	if a.VariantID <= 0 || a.Delta == 0 || a.Delta < -100000 || a.Delta > 100000 || len(a.Reason) < 3 || len(a.Reason) > 500 {
		return ErrInvalidInventoryAdjustment
	}
	repository, ok := s.repository.(interface {
		AdjustAdminInventory(context.Context, int64, domaincommerce.InventoryAdjustment) error
	})
	if !ok {
		return ports.ErrForbidden
	}
	return repository.AdjustAdminInventory(ctx, userID, a)
}
