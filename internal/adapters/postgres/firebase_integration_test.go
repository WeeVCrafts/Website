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
	if _, err := r.ResolveFirebase(ctx, proof, "", "", false, 0); !errors.Is(err, applicationauth.ErrFirebaseTerms) {
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
		if _, err := r.ResolveFirebase(ctx, proof, "", "", true, linkID); !errors.Is(err, applicationauth.ErrFirebaseLinkRequired) {
			t.Fatalf("email matching bypassed ownership: %v", err)
		}
	}
	user, err := r.ResolveFirebase(ctx, proof, "", "", true, id)
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
	if _, err = r.ResolveFirebase(ctx, proof, "", "", true, id); !errors.Is(err, applicationauth.ErrFirebaseInvalid) {
		t.Fatalf("second identity took over account: %v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE user_roles SET role_slug='seller' WHERE user_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	proof.UID = uid
	if recipient, found, err := r.FirebaseEmailRecipient(ctx, 0, email); err != nil || !found || recipient.UID != uid {
		t.Fatal("existing Firebase seller cannot recover their password")
	}
	if _, found, err := r.FirebaseEmailRecipient(ctx, id, ""); err != nil || found {
		t.Fatal("seller received customer email verification")
	}
	if account, boundUID, err := r.FirebasePasswordAccount(ctx, email); err != nil || boundUID != uid || len(account.Roles) != 1 || account.Roles[0] != "seller" {
		t.Fatalf("seller password sign-in lookup failed: %v", err)
	}
	if allowed, err := r.FirebaseVerificationAllowed(ctx, email); err != nil || allowed {
		t.Fatal("seller received customer verification")
	}
	if seller, err := r.ResolveFirebase(ctx, proof, "", "", false, 0); err != nil || seller.ID != id || len(seller.Roles) != 1 || seller.Roles[0] != "seller" {
		t.Fatalf("linked seller did not keep the same account and role: %v", err)
	}
	proof.Provider = "password"
	if _, err = r.ResolveFirebase(ctx, proof, "", "", false, 0); !errors.Is(err, applicationauth.ErrFirebaseRole) {
		t.Fatalf("Firebase password proof entered the seller Google flow: %v", err)
	}
}

func TestExistingSellerGoogleLinkPreservesNativeCredentials(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := t.Context()
	repo := NewFirebaseRepository(pool)
	proof := ports.FirebaseIdentity{Email: fmt.Sprintf("seller-google-%d@example.invalid", time.Now().UnixNano()), UID: fmt.Sprintf("seller-google-%d", time.Now().UnixNano()), Provider: "google.com", EmailVerified: true}
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status) VALUES($1,'Existing seller','active') RETURNING id`, proof.Email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE actor_id=$1`, fmt.Sprint(id))
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, id)
	})
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'seller')`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO credentials(user_id,password_hash) VALUES($1,'fixture-native-hash')`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ResolveFirebase(ctx, proof, "", "", false, 0); !errors.Is(err, applicationauth.ErrFirebaseLinkRequired) {
		t.Fatalf("email matching bypassed seller password proof: %v", err)
	}
	for _, linkID := range []int64{id, 0} {
		u, err := repo.ResolveFirebase(ctx, proof, "", "", false, linkID)
		if err != nil || u.ID != id || len(u.Roles) != 1 || u.Roles[0] != "seller" {
			t.Fatalf("existing seller could not sign in without new-account consent: %v", err)
		}
	}
	var hash string
	if err := pool.QueryRow(ctx, `SELECT password_hash FROM credentials WHERE user_id=$1`, id).Scan(&hash); err != nil || hash != "fixture-native-hash" {
		t.Fatalf("native credentials changed: %v", err)
	}
	if _, found, err := repo.FirebaseEmailRecipient(ctx, 0, proof.Email); err != nil || found {
		t.Fatalf("native password reset was redirected to Firebase: %v", err)
	}
	if _, _, err := repo.FirebasePasswordAccount(ctx, proof.Email); err == nil {
		t.Fatal("Firebase password fallback bypassed native credentials")
	}
	proof.UID += "-different"
	if _, err := repo.ResolveFirebase(ctx, proof, "", "", true, id); !errors.Is(err, applicationauth.ErrFirebaseInvalid) {
		t.Fatalf("different Google identity replaced the seller binding: %v", err)
	}
}
func TestFirebaseRegistrationAndPrivacyDeletion(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	r := NewFirebaseRepository(pool)
	suffix := time.Now().UnixNano()
	uid := fmt.Sprintf("firebase-delete-%d", suffix)
	u, err := r.ResolveFirebase(ctx, ports.FirebaseIdentity{UID: uid, Email: fmt.Sprintf("firebase-delete-%d@example.invalid", suffix), Name: "Private fixture", EmailVerified: true}, "+919876543210", "firebase_delete", true, 0)
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
