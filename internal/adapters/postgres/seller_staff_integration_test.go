package postgres

import (
	"context"
	"testing"
)

func TestSellerStaffCRUDPersistsPermissionsAndRemoval(t *testing.T) {
	pool := openIntegrationPool(t)
	fixture := newCommerceFixture(t, pool, 1, 0)
	var staffEmail string
	if err := pool.QueryRow(context.Background(), `SELECT email FROM users WHERE id=$1`, fixture.userIDs[0]).Scan(&staffEmail); err != nil {
		t.Fatalf("load staff fixture email: %v", err)
	}
	repository := NewSellerStaffRepository(pool)

	member, err := repository.AddStaff(context.Background(), fixture.ownerID, staffEmail, []string{"PRODUCT_READ"})
	if err != nil {
		t.Fatalf("add staff: %v", err)
	}
	if member.UserID != fixture.userIDs[0] || member.Status != "active" {
		t.Fatalf("added staff was not returned: %+v", member)
	}
	member, err = repository.UpdateStaff(context.Background(), fixture.ownerID, member.UserID, []string{"PRODUCT_READ", "PRODUCT_WRITE"})
	if err != nil {
		t.Fatalf("update staff permissions: %v", err)
	}
	if len(member.Permissions) != 2 {
		t.Fatalf("updated permissions did not persist: %+v", member.Permissions)
	}
	if err := repository.RemoveStaff(context.Background(), fixture.ownerID, member.UserID); err != nil {
		t.Fatalf("remove staff: %v", err)
	}
	staff, err := repository.ListStaff(context.Background(), fixture.ownerID)
	if err != nil {
		t.Fatalf("list staff after removal: %v", err)
	}
	for _, listed := range staff {
		if listed.UserID == member.UserID {
			t.Fatalf("removed staff remains active: %+v", listed)
		}
	}
	var membershipStatus string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM seller_users WHERE seller_id=(SELECT id FROM sellers WHERE owner_user_id=$1) AND user_id=$2`, fixture.ownerID, member.UserID).Scan(&membershipStatus); err != nil {
		t.Fatalf("load removed membership state: %v", err)
	}
	if membershipStatus != "revoked" {
		t.Fatalf("membership state = %q, want revoked", membershipStatus)
	}
	var staffRoleExists bool
	if err := pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM user_roles WHERE user_id=$1 AND role_slug='seller_staff')`, member.UserID).Scan(&staffRoleExists); err != nil {
		t.Fatalf("check removed staff role: %v", err)
	}
	if staffRoleExists {
		t.Fatal("seller_staff role remains after the user's only seller membership was removed")
	}
}
