package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"slices"
	"testing"

	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestStorefrontDraftApprovalIsolationConcurrencyAndRestore(t *testing.T) {
	pool := isolatedRolePool(t, false)
	ctx := context.Background()
	r := NewStorefrontRepository(pool)
	admin := roleFixture(t, pool, "admin")
	super := roleFixture(t, pool, "super_admin")
	seller := roleFixture(t, pool, "seller")
	if e := r.Bootstrap(ctx); e != nil {
		t.Fatal(e)
	}
	initial, e := ReadPublishedStorefront(ctx, pool)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.NewDraft(ctx, seller, 0); !errors.Is(e, ports.ErrForbidden) {
		t.Fatalf("seller draft=%v", e)
	}
	id, e := r.NewDraft(ctx, admin, 0)
	if e != nil {
		t.Fatal(e)
	}
	d, e := r.Draft(ctx, admin, id)
	if e != nil {
		t.Fatal(e)
	}
	d.Config.Slides[0].Title = "Private draft hero"
	d.Config.Categories[0].Order = 999
	if e = r.Save(ctx, admin, id, d.Version, d.Config); e != nil {
		t.Fatal(e)
	}
	if e = r.Save(ctx, admin, id, d.Version, d.Config); !errors.Is(e, storefront.ErrConflict) {
		t.Fatalf("stale save=%v", e)
	}
	live, e := ReadPublishedStorefront(ctx, pool)
	if e != nil || live.ID != initial.ID || live.Config.Slides[0].Title == "Private draft hero" {
		t.Fatalf("draft leaked: %+v %v", live, e)
	}
	if e = r.Transition(ctx, admin, id, 2, "publish", "Cannot publish"); !errors.Is(e, ports.ErrForbidden) {
		t.Fatalf("admin publication=%v", e)
	}
	if e = r.Transition(ctx, admin, id, 2, "submit", ""); e != nil {
		t.Fatal(e)
	}
	if e = r.Save(ctx, admin, id, 3, d.Config); !errors.Is(e, storefront.ErrLocked) {
		t.Fatalf("submitted save=%v", e)
	}
	if e = r.Transition(ctx, super, id, 3, "request-changes", "Please review heading"); e != nil {
		t.Fatal(e)
	}
	if e = r.Transition(ctx, admin, id, 4, "submit", ""); e != nil {
		t.Fatal(e)
	}
	if e = r.Transition(ctx, super, id, 5, "publish", "Reviewed homepage"); e != nil {
		t.Fatal(e)
	}
	live, e = ReadPublishedStorefront(ctx, pool)
	if e != nil || live.ID == initial.ID || live.Config.Slides[0].Title != "Private draft hero" {
		t.Fatalf("publication missing: %+v %v", live, e)
	}
	if _, e = pool.Exec(ctx, `UPDATE storefront_releases SET note='rewritten' WHERE id=$1`, live.ID); e == nil {
		t.Fatal("release was mutable")
	}
	restored, e := r.NewDraft(ctx, super, initial.ID)
	if e != nil {
		t.Fatal(e)
	}
	rd, e := r.Draft(ctx, super, restored)
	if e != nil || rd.Config.Slides[0].Title != initial.Config.Slides[0].Title || rd.State != "draft" {
		t.Fatalf("restore=%+v %v", rd, e)
	}
	if _, e = pool.Exec(ctx, `UPDATE user_roles SET role_slug='customer' WHERE user_id=$1`, admin); e != nil {
		t.Fatal(e)
	}
	if _, e = r.Draft(ctx, admin, id); !errors.Is(e, ports.ErrForbidden) {
		t.Fatalf("revoked preview=%v", e)
	}
	if e = NewManagementRepository(pool).MutateManagement(ctx, super, ports.ManagementMutation{Section: "settings", Action: "save"}); !errors.Is(e, ports.ErrForbidden) {
		t.Fatalf("legacy settings bypass=%v", e)
	}
}

func TestStorefrontPoliciesQuoteOrderSnapshotsAndCouponCounters(t *testing.T) {
	pool := isolatedRolePool(t, false)
	ctx := context.Background()
	f := newCommerceFixture(t, pool, 2, 20)
	actor := roleFixture(t, pool, "super_admin")
	repo := NewStorefrontRepository(pool)
	commerce := NewCommerceRepository(pool)
	if e := repo.Bootstrap(ctx); e != nil {
		t.Fatal(e)
	}
	initial, _ := ReadPublishedStorefront(ctx, pool)
	id, e := repo.NewDraft(ctx, actor, 0)
	if e != nil {
		t.Fatal(e)
	}
	d, _ := repo.Draft(ctx, actor, id)
	d.Config.Settings.Policy.ShippingCents = 19900
	d.Config.Settings.Policy.ReturnDays = 7
	d.Config.Settings.Policy.PINRestricted = true
	d.Config.Settings.Policy.PINCodes = []string{"411001"}
	d.Config.Promotions = []storefront.Promotion{{Key: "snapshot-discount", Name: "Snapshot discount", Kind: "percentage", Value: 10, Status: "active", TargetMode: "products", ProductIDs: []int64{f.productID}, UsageLimit: 50, Coupons: []storefront.Coupon{{Code: "SNAPSHOT10", UsageLimit: 50, Active: true}}}}
	if e = repo.Save(ctx, actor, id, 1, d.Config); e != nil {
		t.Fatal(e)
	}
	if e = repo.Transition(ctx, actor, id, 2, "publish", "Reviewed shopping settings"); e != nil {
		t.Fatal(e)
	}
	tx, e := pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	storedPromotions, readErr := readStorefrontPromotions(ctx, tx)
	_ = tx.Rollback(ctx)
	if readErr != nil || len(storedPromotions) != 1 || storedPromotions[0].TargetMode != "products" || !slices.Equal(storedPromotions[0].ProductIDs, []int64{f.productID}) {
		t.Fatalf("selected-product promotion did not survive catalog conversion: %+v %v", storedPromotions, readErr)
	}
	quote, e := commerce.QuoteOrder(ctx, f.userIDs[0], f.cartIDs[0], "SNAPSHOT10")
	if e != nil || quote.PolicyRevision == initial.ID || quote.ShippingCents != 19900 {
		t.Fatalf("quote=%+v %v", quote, e)
	}
	address := domaincommerce.AddressInput{RecipientName: "QA", Phone: "9000000000", Line1: "Test lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"}
	if _, e = commerce.CreateQuotedOrder(storefront.WithExpectedPolicy(ctx, initial.ID), f.userIDs[0], f.cartIDs[0], "stale-policy-revision", address, "SNAPSHOT10", quote.TotalCents); !errors.Is(e, ports.ErrQuoteChanged) {
		t.Fatalf("stale quote=%v", e)
	}
	bad := address
	bad.PostalCode = "500001"
	if _, e = commerce.CreateQuotedOrder(ctx, f.userIDs[0], f.cartIDs[0], "unserviceable-pin", bad, "", quote.TotalCents); !errors.Is(e, ports.ErrInvalidState) {
		t.Fatalf("PIN accepted=%v", e)
	}
	order, e := commerce.CreateQuotedOrder(storefront.WithExpectedPolicy(ctx, quote.PolicyRevision), f.userIDs[0], f.cartIDs[0], "policy-snapshot-checkout", address, "SNAPSHOT10", quote.TotalCents)
	if e != nil || order.TotalCents != quote.TotalCents {
		t.Fatalf("order=%+v %v", order, e)
	}
	var policy storefront.Policy
	var data []byte
	var revision int64
	if e = pool.QueryRow(ctx, `SELECT policy_revision,policy_snapshot FROM orders WHERE id=$1`, order.ID).Scan(&revision, &data); e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(data, &policy); e != nil || policy.ReturnDays != 7 || revision != quote.PolicyRevision {
		t.Fatalf("snapshot=%+v %v", policy, e)
	}
	restore, e := repo.NewDraft(ctx, actor, initial.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = repo.Transition(ctx, actor, restore, 1, "publish", "Restore original presentation and defaults"); e != nil {
		t.Fatal(e)
	}
	var uses int
	if e = pool.QueryRow(ctx, `SELECT usage_count FROM coupons WHERE code='SNAPSHOT10'`).Scan(&uses); e != nil || uses != 1 {
		t.Fatalf("coupon counters rewound: %d %v", uses, e)
	}
	retry, e := commerce.CreateQuotedOrder(storefront.WithExpectedPolicy(ctx, quote.PolicyRevision), f.userIDs[0], f.cartIDs[0], "policy-snapshot-checkout", address, "SNAPSHOT10", quote.TotalCents)
	if e != nil || retry.ID != order.ID {
		t.Fatalf("retry changed historical order: %+v %v", retry, e)
	}
	if _, e = pool.Exec(ctx, `UPDATE orders SET status='delivered',delivered_at=now()-interval '8 days' WHERE id=$1`, order.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = commerce.CreateReturnRequest(ctx, f.userIDs[0], order.OrderNumber, "Window must follow purchase promise"); !errors.Is(e, ports.ErrReturnNotFound) {
		t.Fatalf("restoration rewound return window: %v", e)
	}
}
func TestStorefrontFailedPublicationRetainsPreviousRevision(t *testing.T) {
	pool := isolatedRolePool(t, false)
	ctx := context.Background()
	r := NewStorefrontRepository(pool)
	actor := roleFixture(t, pool, "super_admin")
	if e := r.Bootstrap(ctx); e != nil {
		t.Fatal(e)
	}
	id, e := r.NewDraft(ctx, actor, 0)
	if e != nil {
		t.Fatal(e)
	}
	d, e := r.Draft(ctx, actor, id)
	if e != nil {
		t.Fatal(e)
	}
	// A dangling promotion reference fails inside the publication transaction.
	d.Config.Promotions = []storefront.Promotion{{Key: "invalid-product", Name: "Invalid reference", Kind: "fixed", Value: 100, Status: "active", TargetMode: "products", ProductIDs: []int64{999999999}}}
	if e = r.Save(ctx, actor, id, 1, d.Config); e != nil {
		t.Fatal(e)
	}
	before, _ := ReadPublishedStorefront(ctx, pool)
	if e = r.Transition(ctx, actor, id, 2, "publish", "Review invalid reference"); e == nil {
		t.Fatal("published invalid reference")
	}
	after, _ := ReadPublishedStorefront(ctx, pool)
	if after.ID != before.ID {
		t.Fatal("failed publication changed head")
	}
	d, _ = r.Draft(ctx, actor, id)
	if d.State != "draft" || d.Version != 2 {
		t.Fatalf("failure mutated draft: %+v", d)
	}
}

func TestStorefrontConcurrentPublishHasOneWinnerAndReferencedCategoriesStay(t *testing.T) {
	pool := isolatedRolePool(t, false)
	ctx := context.Background()
	r := NewStorefrontRepository(pool)
	f := newCommerceFixture(t, pool, 1, 5)
	actors := []int64{roleFixture(t, pool, "super_admin"), roleFixture(t, pool, "super_admin")}
	if e := r.Bootstrap(ctx); e != nil {
		t.Fatal(e)
	}
	ids := []int64{}
	for i, actor := range actors {
		id, e := r.NewDraft(ctx, actor, 0)
		if e != nil {
			t.Fatal(e)
		}
		d, _ := r.Draft(ctx, actor, id)
		d.Config.Slides[0].Title = fmt.Sprintf("Concurrent publication %d", i)
		if e = r.Save(ctx, actor, id, 1, d.Config); e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	start := make(chan struct{})
	out := make(chan error, 2)
	for i, actor := range actors {
		go func(actor, id int64) {
			<-start
			out <- r.Transition(ctx, actor, id, 2, "publish", "Reviewed concurrent publication")
		}(actor, ids[i])
	}
	close(start)
	first, second := <-out, <-out
	if !((first == nil && errors.Is(second, storefront.ErrConflict)) || (second == nil && errors.Is(first, storefront.ErrConflict))) {
		t.Fatalf("publish results: %v %v", first, second)
	}
	var count int
	if e := pool.QueryRow(ctx, `SELECT count(*) FROM storefront_releases`).Scan(&count); e != nil || count != 2 {
		t.Fatalf("publication count=%d %v", count, e)
	}
	id, e := r.NewDraft(ctx, actors[0], 0)
	if e != nil {
		t.Fatal(e)
	}
	d, _ := r.Draft(ctx, actors[0], id)
	d.Config.Categories = slices.DeleteFunc(d.Config.Categories, func(c storefront.Category) bool { return c.Slug == f.category })
	if e = r.Save(ctx, actors[0], id, 1, d.Config); e != nil {
		t.Fatal(e)
	}
	if e = r.Transition(ctx, actors[0], id, 2, "publish", "Reject category with product membership"); !errors.Is(e, storefront.ErrValidation) {
		t.Fatalf("deleted category in use: %v", e)
	}
}
