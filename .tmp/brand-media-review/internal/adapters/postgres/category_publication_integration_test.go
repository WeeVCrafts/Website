package postgres

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestCategoryPublicationIsAtomicScopedAndVersioned(t *testing.T) {
	pool := isolatedRolePool(t, false)
	ctx := context.Background()
	repo := NewStorefrontRepository(pool)
	super, admin, seller := roleFixture(t, pool, "super_admin"), roleFixture(t, pool, "admin"), roleFixture(t, pool, "seller")
	if err := repo.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	initial, err := ReadPublishedStorefront(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := repo.NewDraft(ctx, admin, 0)
	if err != nil {
		t.Fatal(err)
	}
	image := &CategoryImage{ID: "item-0123456789abcdef01234567", ObjectKey: "storefront/item-0123456789abcdef01234567.jpg", ByteSize: 400, Variants: map[string]string{"1920": "storefront/item-0123456789abcdef01234567.jpg", "960": "storefront/item-0123456789abcdef01234567-960.jpg", "480": "storefront/item-0123456789abcdef01234567-480.jpg"}}
	category := storefront.Category{Slug: "publication-qa", Name: "Publication QA", Order: 2, Visible: true}
	for _, actor := range []int64{admin, seller} {
		if err := repo.PublishCategory(ctx, actor, initial.ID, "create", category, image, "Unauthorized publication"); !errors.Is(err, ports.ErrForbidden) {
			t.Fatalf("actor %d: %v", actor, err)
		}
	}
	if err := repo.PublishCategory(ctx, super, initial.ID, "create", category, nil, "Missing photograph"); !errors.Is(err, ports.ErrManagementValidation) {
		t.Fatalf("missing image: %v", err)
	}
	if err := repo.PublishCategory(ctx, super, initial.ID, "create", category, image, "Create category QA"); err != nil {
		t.Fatal(err)
	}
	live, err := ReadPublishedStorefront(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if live.ID == initial.ID || len(live.Config.Categories) != len(initial.Config.Categories)+1 {
		t.Fatal("release not advanced")
	}
	comparison := live.Config
	comparison.Categories = initial.Config.Categories
	if !reflect.DeepEqual(comparison, initial.Config) {
		t.Fatal("unrelated published configuration changed")
	}
	var url string
	var visible bool
	var order int
	if err := pool.QueryRow(ctx, `SELECT image_url,show_in_navigation,display_order FROM categories WHERE slug=$1`, category.Slug).Scan(&url, &visible, &order); err != nil || url != "/media/storefront/"+image.ID || !visible || order != 2 {
		t.Fatalf("projection: %q %v %d %v", url, visible, order, err)
	}
	if err := repo.PublishCategory(ctx, super, initial.ID, "update", category, nil, "Stale change"); !errors.Is(err, storefront.ErrConflict) {
		t.Fatalf("stale: %v", err)
	}
	if err := repo.PublishCategory(ctx, super, live.ID, "create", category, image, "Duplicate slug"); !errors.Is(err, ports.ErrManagementConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	category.Name = "Renamed category"
	category.Visible = false
	category.Order = 4
	if err := repo.PublishCategory(ctx, super, live.ID, "update", category, nil, "Rename and hide"); err != nil {
		t.Fatal(err)
	}
	hidden, _ := ReadPublishedStorefront(ctx, pool)
	for _, c := range hidden.Config.Categories {
		if c.Slug == category.Slug && (c.Image != url || c.Visible || c.Name != category.Name) {
			t.Fatal("edit lost photo or visibility")
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE resource_type='category' AND resource_id=$1`, category.Slug).Scan(&count); err != nil || count != 2 {
		t.Fatalf("audit: %d %v", count, err)
	}
	if err := repo.Transition(ctx, admin, draft, 1, "submit", ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.Transition(ctx, super, draft, 2, "publish", "Older draft must not overwrite categories"); !errors.Is(err, storefront.ErrConflict) {
		t.Fatalf("old draft: %v", err)
	}
	if _, err := pool.Exec(ctx, `CREATE FUNCTION reject_category_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.resource_type='category' THEN RAISE EXCEPTION 'QA audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_category_audit BEFORE INSERT ON audit_logs FOR EACH ROW EXECUTE FUNCTION reject_category_audit()`); err != nil {
		t.Fatal(err)
	}
	category.Name = "Must roll back"
	replacement := &CategoryImage{ID: "item-fedcba987654321001234567", ByteSize: image.ByteSize, Variants: map[string]string{}}
	replacement.ObjectKey = strings.ReplaceAll(image.ObjectKey, image.ID, replacement.ID)
	for width, key := range image.Variants {
		replacement.Variants[width] = strings.ReplaceAll(key, image.ID, replacement.ID)
	}
	if err := repo.PublishCategory(ctx, super, hidden.ID, "update", category, replacement, "Force rollback"); err == nil {
		t.Fatal("expected audit failure")
	}
	after, _ := ReadPublishedStorefront(ctx, pool)
	if after.ID != hidden.ID || !reflect.DeepEqual(after.Config, hidden.Config) {
		t.Fatal("failed transaction changed release")
	}
	var name string
	_ = pool.QueryRow(ctx, `SELECT name FROM categories WHERE slug=$1`, category.Slug).Scan(&name)
	if name != "Renamed category" {
		t.Fatal("failed transaction changed projection")
	}
	var mediaCount int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM storefront_media WHERE id=$1`, image.ID).Scan(&mediaCount)
	if mediaCount != 1 {
		t.Fatal("historical media was removed")
	}
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM storefront_media WHERE id=$1`, replacement.ID).Scan(&mediaCount)
	if mediaCount != 0 {
		t.Fatal("failed transaction retained new media metadata")
	}
}
