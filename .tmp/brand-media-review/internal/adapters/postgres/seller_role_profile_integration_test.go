package postgres

import (
	"context"
	"errors"
	"strconv"
	"testing"

	identity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

// A Seller role without an owned business cannot enter /seller-admin. Reapplying
// the role is the existing, authorized repair path for incomplete account data.
func TestSellerRoleAssignmentCreatesAndRepairsOwnedProfile(t *testing.T) {
	pool := isolatedRolePool(t, false)
	ctx := context.Background()
	admin := roleFixture(t, pool, "super_admin")
	repo := NewRoleAdministrationRepository(pool)
	for _, originalRole := range []string{"customer", "seller"} {
		t.Run(originalRole, func(t *testing.T) {
			userID := roleFixture(t, pool, originalRole)
			if err := repo.UpdateRoleAssignment(ctx, userID, userID, 0, identity.RoleSeller, true, "Unauthorized profile repair"); !errors.Is(err, ports.ErrForbidden) {
				t.Fatalf("non-admin repair: %v", err)
			}
			var count int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM sellers WHERE owner_user_id=$1`, userID).Scan(&count); err != nil || count != 0 {
				t.Fatalf("unauthorized repair created a profile: %d, %v", count, err)
			}
			if err := repo.UpdateRoleAssignment(ctx, admin, userID, 0, identity.RoleSeller, true, "Repair missing seller business profile"); err != nil {
				t.Fatal(err)
			}
			var sellerID int64
			var status, name, role string
			var revoked bool
			if err := pool.QueryRow(ctx, `SELECT s.id,s.status,s.display_name,r.role_slug,u.sessions_revoked_at IS NOT NULL FROM sellers s JOIN users u ON u.id=s.owner_user_id JOIN user_roles r ON r.user_id=u.id WHERE u.id=$1`, userID).Scan(&sellerID, &status, &name, &role, &revoked); err != nil || sellerID <= 0 || status != "pending" || name != originalRole || role != "seller" || !revoked {
				t.Fatalf("incomplete repaired seller: id=%d status=%s name=%s role=%s revoked=%v err=%v", sellerID, status, name, role, revoked, err)
			}
			// Repeated repair must retain the same business and its approval state.
			if _, err := pool.Exec(ctx, `UPDATE sellers SET display_name='Existing shop',status='suspended',public_story='Saved maker story' WHERE id=$1`, sellerID); err != nil {
				t.Fatal(err)
			}
			if err := repo.UpdateRoleAssignment(ctx, admin, userID, 0, identity.RoleSeller, true, "Verify repair preserves existing business"); err != nil {
				t.Fatal(err)
			}
			var preserved bool
			if err := pool.QueryRow(ctx, `SELECT count(*)=1 AND bool_and(id=$2 AND display_name='Existing shop' AND status='suspended' AND public_story='Saved maker story') FROM sellers WHERE owner_user_id=$1`, userID, sellerID).Scan(&preserved); err != nil || !preserved {
				t.Fatalf("repair replaced or activated existing shop: %v, %v", preserved, err)
			}
			var audits int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action='identity.role_replaced' AND resource_type='user' AND resource_id=$1`, strconv.FormatInt(userID, 10)).Scan(&audits); err != nil || audits != 2 {
				t.Fatalf("repair audit count: %d, %v", audits, err)
			}
		})
	}
}
