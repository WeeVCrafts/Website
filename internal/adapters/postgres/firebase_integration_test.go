package postgres

import (
	"context"
	"errors"
	"fmt"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
	"time"
)

func TestFirebaseAccountOwnershipAndExplicitLink(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	r := NewFirebaseRepository(pool)
	suffix := time.Now().UnixNano()
	email := fmt.Sprintf("firebase-%d@example.invalid", suffix)
	uid := fmt.Sprintf("firebase-%d", suffix)
	proof := ports.FirebaseIdentity{UID: uid, Email: email, Name: "Firebase fixture", Provider: "google.com", EmailVerified: true}
	if _, err := r.ResolveFirebase(ctx, proof, "", false, 0); !errors.Is(err, applicationauth.ErrFirebaseTerms) {
		t.Fatalf("unconsented registration: %v", err)
	}
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status) VALUES($1,'Existing customer','active') RETURNING id`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'customer');`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO credentials(user_id,password_hash) VALUES($1,'fixture-hash')`, id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM firebase_identities WHERE user_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE actor_id=$1`, fmt.Sprint(id))
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	})
	for _, linkID := range []int64{0, id + 1} {
		if _, err := r.ResolveFirebase(ctx, proof, "", true, linkID); !errors.Is(err, applicationauth.ErrFirebaseLinkRequired) {
			t.Fatalf("email matching bypassed ownership: %v", err)
		}
	}
	user, err := r.ResolveFirebase(ctx, proof, "", true, id)
	if err != nil || user.ID != id {
		t.Fatalf("explicit link failed: %v", err)
	}
	if recipient, found, err := r.FirebaseEmailRecipient(ctx, 0, email); err != nil || !found || recipient.UID != uid {
		t.Fatal("bound customer recovery recipient missing")
	}
	if allowed, err := r.FirebaseVerificationAllowed(ctx, email); err != nil || !allowed {
		t.Fatal("customer verification rejected")
	}
	if recipient, found, err := r.FirebaseEmailRecipient(ctx, id+100000, email); err != nil || found || recipient.UID != "" {
		t.Fatal("guessed owner identifier recovered another account")
	}
	var retained bool
	if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM credentials WHERE user_id=$1)`, id).Scan(&retained); err != nil || retained {
		t.Fatal("old password remains usable")
	}
	proof.UID = "different-fixture"
	if _, err = r.ResolveFirebase(ctx, proof, "", true, id); !errors.Is(err, applicationauth.ErrFirebaseInvalid) {
		t.Fatalf("second identity took over account: %v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE user_roles SET role_slug='seller_owner' WHERE user_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	proof.UID = uid
	if _, found, err := r.FirebaseEmailRecipient(ctx, 0, email); err != nil || found {
		t.Fatal("seller received customer recovery")
	}
	if allowed, err := r.FirebaseVerificationAllowed(ctx, email); err != nil || allowed {
		t.Fatal("seller received customer verification")
	}
	if _, err = r.ResolveFirebase(ctx, proof, "", true, id); !errors.Is(err, applicationauth.ErrFirebaseRole) {
		t.Fatalf("seller granted customer Firebase session: %v", err)
	}
}
func TestFirebaseRegistrationAndPrivacyDeletion(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	r := NewFirebaseRepository(pool)
	suffix := time.Now().UnixNano()
	uid := fmt.Sprintf("firebase-delete-%d", suffix)
	u, err := r.ResolveFirebase(ctx, ports.FirebaseIdentity{UID: uid, Email: fmt.Sprintf("firebase-delete-%d@example.invalid", suffix), Name: "Private fixture", EmailVerified: true}, "+919876543210", true, 0)
	if err != nil {
		t.Fatal(err)
	}
	privacy := NewPrivacyRepository(pool)
	request, err := privacy.CreatePrivacyRequest(ctx, u.ID, "deletion", "Delete Firebase fixture")
	if err != nil {
		t.Fatal(err)
	}
	if err = privacy.ExecuteDeletion(ctx, u.ID, request.ID); err != nil {
		t.Fatal(err)
	}
	var queue, bindings int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM firebase_deletion_queue WHERE firebase_uid=$1),(SELECT count(*) FROM firebase_identities WHERE user_id=$2)`, uid, u.ID).Scan(&queue, &bindings); err != nil || queue != 1 || bindings != 0 {
		t.Fatalf("identity deletion not reconciled: queued=%d retained=%d err=%v", queue, bindings, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM firebase_deletion_queue WHERE firebase_uid=$1`, uid)
		_, _ = pool.Exec(ctx, `DELETE FROM privacy_requests WHERE user_id=$1`, u.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM consent_records WHERE user_id=$1`, u.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE actor_id=$1`, fmt.Sprint(u.ID))
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, u.ID)
	})
}
