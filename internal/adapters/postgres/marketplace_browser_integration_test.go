package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	redisclient "github.com/redis/go-redis/v9"
	redisadapter "github.com/wecratfs/commerce/internal/adapters/redis"
	"github.com/wecratfs/commerce/internal/ports"
	"os"
	"strings"
	"testing"
	"time"
)

// Opt-in fixture creation is restricted to the disposable integration services.
// Never run this helper against the business database or production Redis.
func TestMarketplaceBrowserFixture(t *testing.T) {
	file := os.Getenv("MARKETPLACE_UI_FIXTURE_FILE")
	if file == "" {
		t.Skip("opt-in browser fixture")
	}
	if !strings.Contains(os.Getenv("DATABASE_URL"), "@127.0.0.1:15465/marketplace_policy_test") {
		t.Fatal("isolated database required")
	}
	x := newMarketplaceFixture(t)
	x.capture(t)
	var policyOrder string
	if os.Getenv("MARKETPLACE_POLICY_UI") == "1" {
		// The policy fixture resets its global setting during cleanup. Re-publish
		// after that cleanup so the real public policy handler remains usable.
		t.Cleanup(func() {
			if err := x.repo.ConfigurePolicy(context.Background(), x.admin, "publish_policy", "Isolated browser policy publication after fixture cleanup", 0, 0); err != nil {
				t.Error(err)
			}
		})
		if err := x.repo.ConfigurePolicy(context.Background(), x.admin, "publish_policy", "Isolated browser policy fixture publication", 0, 0); err != nil {
			t.Fatal(err)
		}
		px, item, parcel := policyFixture(t)
		submitPolicyClaim(t, px, item, parcel, "change_of_mind")
		if _, err := px.pool.Exec(context.Background(), `UPDATE orders SET user_id=$2 WHERE id=$1`, px.order.ID, x.f.userIDs[0]); err != nil {
			t.Fatal(err)
		}
		if _, err := px.pool.Exec(context.Background(), `INSERT INTO store_credit_grants(user_id,amount_cents,remaining_cents,expires_at) VALUES($1,50000,50000,now()+interval '6 months')`, x.f.userIDs[0]); err != nil {
			t.Fatal(err)
		}
		policyOrder = px.order.OrderNumber
	}

	mixed, second := mixedCommerceFixture(t, x)
	legacy := newMarketplaceFixtureWithPool(t, x.pool)
	legacy.capture(t)
	if _, err := x.pool.Exec(context.Background(), `UPDATE orders SET workflow_version=1,status='delivered',lifecycle_status='awaiting_payment' WHERE id=$1`, legacy.order.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(context.Background(), `UPDATE seller_fulfillments SET status='delivered',carrier='Recorded historical carrier',tracking_number='HISTORICAL-SAVED-TRACK' WHERE order_id=$1`, legacy.order.ID); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := x.pool.Exec(ctx, `UPDATE order_delivery_decisions SET decision='manual_fallback' WHERE order_id=$1`, x.order.ID); err != nil {
		t.Fatal(err)
	}
	var admin int64
	if err := x.pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status) VALUES($1,'Test company operator','active') RETURNING id`, fmt.Sprintf("marketplace-ui-%d@example.invalid", time.Now().UnixNano())).Scan(&admin); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'admin')`, admin); err != nil {
		t.Fatal(err)
	}
	client := redisclient.NewClient(&redisclient.Options{Addr: "127.0.0.1:15464"})
	defer client.Close()
	store := redisadapter.NewSessionStore(client)
	type identity struct {
		Token, CSRF string
		UserID      int64
	}
	result := struct {
		Order, PolicyOrder, MixedOrder, LegacyOrder string
		SellerID, VariantID, SecondSellerID         int64
		Roles                                       map[string]identity
	}{Order: x.order.OrderNumber, LegacyOrder: legacy.order.OrderNumber, PolicyOrder: policyOrder, MixedOrder: mixed.order.OrderNumber, SecondSellerID: second.seller, SellerID: x.seller, VariantID: x.f.variantID, Roles: map[string]identity{}}
	for role, user := range map[string]int64{"customer": x.f.userIDs[0], "seller": x.f.ownerID, "second_seller": second.f.ownerID, "legacy_customer": legacy.f.userIDs[0], "legacy_seller": legacy.f.ownerID, "admin": admin, "super_admin": x.admin} {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			t.Fatal(err)
		}
		token := hex.EncodeToString(b)
		if _, err := rand.Read(b); err != nil {
			t.Fatal(err)
		}
		csrf := hex.EncodeToString(b)
		now := time.Now().UTC()
		session := ports.SessionRecord{ID: token, UserID: user, RoleSlugs: []string{func() string {
			if role == "legacy_customer" {
				return "customer"
			}
			if role == "second_seller" || role == "legacy_seller" {
				return "seller"
			}
			return role
		}()}, CSRFToken: csrf, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(3 * time.Hour)}
		if err := store.Create(ctx, session); err != nil {
			t.Fatal(err)
		}
		result.Roles[role] = identity{token, csrf, user}
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
}
