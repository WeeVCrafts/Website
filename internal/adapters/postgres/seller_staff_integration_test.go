package postgres

import (
	"context"
	"errors"
	identity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

func TestRetiredStaffGrantsCannotAuthorize(t *testing.T) {
	pool := openIntegrationPool(t)
	fixture := newCommerceFixture(t, pool, 1, 0)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'seller'),($2,'customer')`, fixture.ownerID, fixture.userIDs[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'seller_staff')`, fixture.userIDs[0]); err == nil {
		t.Fatal("retired role accepted")
	}
	repo := NewSellerStaffRepository(pool)
	if _, err := repo.AddStaff(ctx, fixture.ownerID, "staff@test.invalid", []string{"PRODUCT_WRITE"}); !errors.Is(err, ports.ErrForbidden) {
		t.Fatal(err)
	}
	if allowed, err := NewRoleAuthorizer(pool).HasSellerPermission(ctx, fixture.userIDs[0], "PRODUCT_WRITE"); err != nil || allowed {
		t.Fatal(allowed, err)
	}
	if allowed, err := NewRoleAuthorizer(pool).HasAnyRole(ctx, fixture.userIDs[0], identity.RoleSeller); err != nil || allowed {
		t.Fatal(allowed, err)
	}
}
