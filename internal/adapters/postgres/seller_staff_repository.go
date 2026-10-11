package postgres

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

type SellerStaffRepository struct {
	pool *pgxpool.Pool
}

func NewSellerStaffRepository(pool *pgxpool.Pool) *SellerStaffRepository {
	return &SellerStaffRepository{pool: pool}
}

// Retained interfaces fail closed; the historical membership table is read-only.
func (r *SellerStaffRepository) ListStaff(context.Context, int64) ([]domaincommerce.SellerStaffMember, error) {
	return nil, ports.ErrForbidden
}
func (r *SellerStaffRepository) AddStaff(context.Context, int64, string, []string) (domaincommerce.SellerStaffMember, error) {
	return domaincommerce.SellerStaffMember{}, ports.ErrForbidden
}
func (r *SellerStaffRepository) UpdateStaff(context.Context, int64, int64, []string) (domaincommerce.SellerStaffMember, error) {
	return domaincommerce.SellerStaffMember{}, ports.ErrForbidden
}
func (r *SellerStaffRepository) RemoveStaff(context.Context, int64, int64) error {
	return ports.ErrForbidden
}
