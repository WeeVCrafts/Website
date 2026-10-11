package commerce

import (
	"context"
	"errors"
	"testing"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

type sellerStaffRepository struct {
	addedEmail       string
	addedPermissions []string
	removedUserID    int64
}

func (r *sellerStaffRepository) ListStaff(context.Context, int64) ([]domaincommerce.SellerStaffMember, error) {
	return []domaincommerce.SellerStaffMember{{UserID: 7, Email: "staff@example.com", Status: "active"}}, nil
}

func (r *sellerStaffRepository) AddStaff(_ context.Context, _ int64, email string, permissions []string) (domaincommerce.SellerStaffMember, error) {
	r.addedEmail = email
	r.addedPermissions = permissions
	return domaincommerce.SellerStaffMember{UserID: 7, Email: email, Permissions: permissions, Status: "active"}, nil
}

func (r *sellerStaffRepository) UpdateStaff(_ context.Context, _ int64, staffUserID int64, permissions []string) (domaincommerce.SellerStaffMember, error) {
	r.addedPermissions = permissions
	return domaincommerce.SellerStaffMember{UserID: staffUserID, Status: "active", Permissions: permissions}, nil
}

func (r *sellerStaffRepository) RemoveStaff(_ context.Context, _ int64, staffUserID int64) error {
	r.removedUserID = staffUserID
	return nil
}

func TestRetiredStaffServiceAlwaysDenies(t *testing.T) {
	repo := &sellerStaffRepository{}
	service := NewSellerStaffService(repo, roleChecker{roles: map[int64]map[domainidentity.Role]bool{10: {domainidentity.RoleSeller: true}}})
	for _, actor := range []int64{10, 11} {
		if _, err := service.List(context.Background(), actor); !errors.Is(err, ports.ErrForbidden) {
			t.Fatal(err)
		}
		if _, err := service.Add(context.Background(), actor, "staff@example.com", []string{"PRODUCT_WRITE"}); !errors.Is(err, ports.ErrForbidden) {
			t.Fatal(err)
		}
		if _, err := service.Update(context.Background(), actor, 17, []string{"PRODUCT_WRITE"}); !errors.Is(err, ports.ErrForbidden) {
			t.Fatal(err)
		}
		if err := service.Remove(context.Background(), actor, 17); !errors.Is(err, ports.ErrForbidden) {
			t.Fatal(err)
		}
	}
	if repo.addedEmail != "" || repo.removedUserID != 0 {
		t.Fatal("retired mutation reached repository")
	}
}
