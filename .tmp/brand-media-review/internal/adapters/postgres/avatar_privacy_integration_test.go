package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
	"time"
)

func TestPrivacyDeletionClearsPhoneAndQueuesPrivatePhoto(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,phone,status) VALUES($1,'Photo fixture','+919876543210','active') RETURNING id`, fmt.Sprintf("avatar-%d@example.invalid", suffix)).Scan(&id); err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("avatars/%d/%032x", id, suffix)
	if _, err := pool.Exec(ctx, `INSERT INTO user_avatars(user_id,object_key) VALUES($1,$2)`, id, key); err != nil {
		t.Fatal(err)
	}
	repository := NewPrivacyRepository(pool)
	request, err := repository.CreatePrivacyRequest(ctx, id, "deletion", "Delete profile fixture")
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.ExecuteDeletion(ctx, id, request.ID); err != nil {
		t.Fatal(err)
	}
	var phone string
	var avatars, queued int
	if err := pool.QueryRow(ctx, `SELECT phone,(SELECT count(*) FROM user_avatars WHERE user_id=$1),(SELECT count(*) FROM avatar_cleanup WHERE object_key=$2) FROM users WHERE id=$1`, id, key).Scan(&phone, &avatars, &queued); err != nil {
		t.Fatal(err)
	}
	if phone != "" || avatars != 0 || queued != 1 {
		t.Fatalf("personal data retained: phone=%t avatars=%d cleanup=%d", phone != "", avatars, queued)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM privacy_requests WHERE user_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE actor_id=$1`, fmt.Sprint(id))
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM avatar_cleanup WHERE object_key=$1`, key)
	})
}

func TestPrivacyDeletionCannotBypassProtectedAdministrator(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status) VALUES($1,'Protected fixture','active') RETURNING id`, fmt.Sprintf("protected-privacy-%d@example.invalid", time.Now().UnixNano())).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'super_admin')`, id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM privacy_requests WHERE user_id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	})
	repository := NewPrivacyRepository(pool)
	request, err := repository.CreatePrivacyRequest(ctx, id, "deletion", "Verify protected administrator safeguard")
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.ExecuteDeletion(ctx, id, request.ID); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("protected deletion error=%v", err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM users WHERE id=$1`, id).Scan(&status); err != nil || status != "active" {
		t.Fatal("protected account was changed")
	}
}
