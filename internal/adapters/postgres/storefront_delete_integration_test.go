package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestStorefrontDeleteDraftProtectsReleasesMediaAndState(t *testing.T) {
	pool := isolatedRolePool(t, false)
	ctx := context.Background()
	repo := NewStorefrontRepository(pool)
	super, admin, seller := roleFixture(t, pool, "super_admin"), roleFixture(t, pool, "admin"), roleFixture(t, pool, "seller")
	if err := repo.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	live, err := ReadPublishedStorefront(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO storefront_media(id,object_key,content_type,byte_size,alt,created_by) VALUES('retained-photo','retained-object','image/jpeg',100,'Retained photo',$1)`, super); err != nil {
		t.Fatal(err)
	}
	id, err := repo.NewDraft(ctx, admin, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, actor := range []int64{admin, seller} {
		if err = repo.DeleteDraft(ctx, actor, id, 1); !errors.Is(err, ports.ErrForbidden) {
			t.Fatalf("actor %d deletion: %v", actor, err)
		}
	}
	if err = repo.DeleteDraft(ctx, super, id, 99); !errors.Is(err, storefront.ErrConflict) {
		t.Fatalf("stale version: %v", err)
	}
	if err = repo.Transition(ctx, admin, id, 1, "submit", ""); err != nil {
		t.Fatal(err)
	}
	if err = repo.DeleteDraft(ctx, super, id, 2); !errors.Is(err, storefront.ErrLocked) {
		t.Fatalf("submitted draft: %v", err)
	}
	if err = repo.Transition(ctx, super, id, 2, "request-changes", "Return for revision"); err != nil {
		t.Fatal(err)
	}
	if err = repo.DeleteDraft(ctx, super, id, 3); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Draft(ctx, super, id); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("deleted draft readable: %v", err)
	}
	if err = repo.DeleteDraft(ctx, super, id, 3); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("repeated deletion: %v", err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM storefront_media WHERE id='retained-photo'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("media changed: %d %v", count, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action='storefront.draft_deleted' AND resource_id=$1`, fmt.Sprint(id)).Scan(&count); err != nil || count != 1 {
		t.Fatalf("audit: %d %v", count, err)
	}
	after, err := ReadPublishedStorefront(ctx, pool)
	if err != nil || after.ID != live.ID {
		t.Fatalf("live release changed: %v", err)
	}
	id, err = repo.NewDraft(ctx, super, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.Transition(ctx, super, id, 1, "publish", "Reviewed unchanged release"); err != nil {
		t.Fatal(err)
	}
	if err = repo.DeleteDraft(ctx, super, id, 2); !errors.Is(err, storefront.ErrLocked) {
		t.Fatalf("published draft: %v", err)
	}
}
