package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestSellerEarningsDiscountSnapshotsAndRefundReplay(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name, kind, targeting               string
		discount, first, second, commission int64
	}{
		{"none", "fixed", "all", 0, 10001, 10000, 3000},
		{"rounding", "fixed", "all", 1001, 9501, 9499, 2849},
		{"targeted", "fixed", "selected", 1001, 9000, 10000, 2900},
		{"free shipping", "free_shipping", "all", 999, 10001, 10000, 3000},
		{"full discount", "fixed", "all", 20001, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first := newCommerceFixture(t, pool, 1, 1)
			second := newCommerceFixture(t, pool, 0, 1)
			suffix := fmt.Sprint(time.Now().UnixNano())
			var promotionID, orderID int64
			if err := pool.QueryRow(ctx, `INSERT INTO promotions(slug,name,discount_type,discount_value,targeting_mode)
				VALUES($1,$1,$2,$3,$4) RETURNING id`, "snapshot-"+suffix, tc.kind, tc.discount, tc.targeting).Scan(&promotionID); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_, _ = pool.Exec(ctx, `DELETE FROM seller_ledger WHERE order_id=$1`, orderID)
				_, _ = pool.Exec(ctx, `DELETE FROM commission_rules WHERE name LIKE $1`, "snapshot-"+suffix+"-%")
				_, _ = pool.Exec(ctx, `DELETE FROM promotions WHERE id=$1`, promotionID)
			})
			if _, err := pool.Exec(ctx, `INSERT INTO promotion_products(promotion_id,product_id) VALUES($1,$2)`, promotionID, first.productID); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO commission_rules(name,category_slug,percentage_basis_points,active)
				VALUES($1,$2,1000,true),($3,$4,2000,true)`, "snapshot-"+suffix+"-1", first.category, "snapshot-"+suffix+"-2", second.category); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `INSERT INTO orders(order_number,user_id,status,subtotal_cents,shipping_cents,discount_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key)
				VALUES($1,$2,'paid',20001,999,$3::bigint,21000-$3::bigint,'{}','snapshot',$1) RETURNING id`, "SNAP-"+suffix, first.userIDs[0], tc.discount).Scan(&orderID); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO order_items(order_id,variant_id,product_name,sku,unit_price_cents,quantity,line_total_cents)
				VALUES($1,$2,'First','FIRST',10001,1,10001),($1,$3,'Second','SECOND',10000,1,10000)`, orderID, first.variantID, second.variantID); err != nil {
				t.Fatal(err)
			}
			if err := WithinTransaction(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
				return snapshotOrderDiscounts(ctx, tx, orderID, promotionID)
			}); err != nil {
				t.Fatal(err)
			}
			// Promotion/catalog changes between placement and capture cannot change allocation.
			if _, err := pool.Exec(ctx, `DELETE FROM promotion_products WHERE promotion_id=$1`, promotionID); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `UPDATE product_variants SET price_cents=1 WHERE id IN ($1,$2)`, first.variantID, second.variantID); err != nil {
				t.Fatal(err)
			}
			record := func(refund bool) {
				t.Helper()
				if err := WithinTransaction(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
					if refund {
						return recordSellerRefundLedger(ctx, tx, orderID)
					}
					return recordSellerSalesLedger(ctx, tx, orderID)
				}); err != nil {
					t.Fatal(err)
				}
			}
			record(false)
			var gross, commission, count int64
			if err := pool.QueryRow(ctx, `SELECT SUM(gross_cents)::bigint,SUM(commission_cents)::bigint,COUNT(*) FROM seller_ledger WHERE order_id=$1`, orderID).Scan(&gross, &commission, &count); err != nil || gross != tc.first+tc.second || commission != tc.commission || count != 2 {
				t.Fatalf("earnings gross=%d commission=%d count=%d: %v", gross, commission, count, err)
			}
			for _, seller := range []struct{ owner, want int64 }{{first.ownerID, tc.first}, {second.ownerID, tc.second}} {
				var got int64
				if err := pool.QueryRow(ctx, `SELECT sl.gross_cents FROM seller_ledger sl JOIN sellers s ON s.id=sl.seller_id WHERE sl.order_id=$1 AND s.owner_user_id=$2`, orderID, seller.owner).Scan(&got); err != nil || got != seller.want {
					t.Fatalf("seller allocation=%d want=%d: %v", got, seller.want, err)
				}
			}
			var before, after string
			var shipping, waived int64
			if err := pool.QueryRow(ctx, `SELECT seller_finance_snapshot::text,(seller_finance_snapshot->>'shipping_cents')::bigint,shipping_discount_cents FROM orders WHERE id=$1 AND seller_finance_snapshot->>'allocation_status'='saved'`, orderID).Scan(&before, &shipping, &waived); err != nil || shipping != 999 || (tc.kind == "free_shipping" && waived != 999) || (tc.kind != "free_shipping" && waived != 0) {
				t.Fatalf("shipping snapshot=%d waived=%d: %v", shipping, waived, err)
			}
			if _, err := pool.Exec(ctx, `UPDATE commission_rules SET percentage_basis_points=9999 WHERE name LIKE $1`, "snapshot-"+suffix+"-%"); err != nil {
				t.Fatal(err)
			}
			record(false)
			if err := pool.QueryRow(ctx, `SELECT seller_finance_snapshot::text FROM orders WHERE id=$1`, orderID).Scan(&after); err != nil || before != after {
				t.Fatalf("snapshot changed on capture replay: %v", err)
			}
			// A refund after a recorded payout appends negatives without rewriting paid entries.
			if _, err := pool.Exec(ctx, `UPDATE seller_ledger SET status='settled' WHERE order_id=$1`, orderID); err != nil {
				t.Fatal(err)
			}
			record(true)
			record(true)
			if err := pool.QueryRow(ctx, `SELECT SUM(gross_cents)::bigint,SUM(commission_cents)::bigint,COUNT(*) FROM seller_ledger WHERE order_id=$1`, orderID).Scan(&gross, &commission, &count); err != nil || gross != 0 || commission != 0 || count != 4 {
				t.Fatalf("refund failed to reverse saved earnings exactly once: gross=%d commission=%d count=%d: %v", gross, commission, count, err)
			}
		})
	}
}
