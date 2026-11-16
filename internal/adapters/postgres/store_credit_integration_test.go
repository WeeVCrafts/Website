package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
	"time"
)

func TestStoreCreditFullyFundsWithoutCapturedProviderPayment(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := domain.WithStoreCredit(context.Background(), true)
	user := x.f.userIDs[1]
	if _, err := x.pool.Exec(ctx, `INSERT INTO store_credit_grants(user_id,amount_cents,remaining_cents,expires_at) VALUES($1,100000,100000,now()+interval '6 months')`, user); err != nil {
		t.Fatal(err)
	}
	o, err := x.commerce.CreateOrder(ctx, user, x.f.cartIDs[1], fmt.Sprintf("credit-checkout-%d", time.Now().UnixNano()), domain.AddressInput{RecipientName: "Fixture buyer", Phone: "9000000000", Line1: "Fixture lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != "paid" || o.CreditCents != o.TotalCents {
		t.Fatalf("not funded: %+v", o)
	}
	var provider, status string
	var amount int64
	var providerID *string
	var funded bool
	if err = x.pool.QueryRow(ctx, `SELECT provider,status,amount_cents,provider_payment_id,marketplace_order_funded(order_id) FROM payment_attempts WHERE order_id=$1`, o.ID).Scan(&provider, &status, &amount, &providerID, &funded); err != nil {
		t.Fatal(err)
	}
	if provider != "store_credit" || status != "funded" || amount != 0 || providerID != nil || !funded {
		t.Fatalf("fabricated provider capture: %s %s %d %v %v", provider, status, amount, providerID, funded)
	}
}

func TestCreditReservationReleaseAndQuoteConflict(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := domain.WithStoreCredit(context.Background(), true)
	user := x.f.userIDs[1]
	if _, err := x.pool.Exec(ctx, `INSERT INTO store_credit_grants(user_id,amount_cents,remaining_cents,expires_at) VALUES($1,500,500,now()+interval '6 months')`, user); err != nil {
		t.Fatal(err)
	}
	address := domain.AddressInput{RecipientName: "Fixture buyer", Phone: "9000000000", Line1: "Fixture lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"}
	if _, err := x.commerce.CreateOrder(domain.WithExpectedCredit(ctx, 600), user, x.f.cartIDs[1], "credit-stale-quote-0001", address); err != ports.ErrQuoteChanged {
		t.Fatalf("stale credit charged: %v", err)
	}
	o, err := x.commerce.CreateOrder(ctx, user, x.f.cartIDs[1], "credit-reservation-0001", address)
	if err != nil {
		t.Fatal(err)
	}
	if o.CreditCents != 500 || o.Status != "pending_payment" {
		t.Fatalf("unexpected allocation %+v", o)
	}
	var cash int64
	if err = x.pool.QueryRow(ctx, `SELECT amount_cents FROM payment_attempts WHERE order_id=$1`, o.ID).Scan(&cash); err != nil || cash != o.TotalCents-500 {
		t.Fatalf("wrong Razorpay due %d %v", cash, err)
	}
	for i := 0; i < 2; i++ {
		if _, err = x.pool.Exec(ctx, `UPDATE orders SET status='payment_failed' WHERE id=$1`, o.ID); err != nil {
			t.Fatal(err)
		}
	}
	left, err := creditAvailable(ctx, x.pool, user)
	if err != nil || left != 500 {
		t.Fatalf("reservation not restored exactly once %d %v", left, err)
	}
}

func TestCreditRestorationIsIdempotentAndExtendsExpiredGrant(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := context.Background()
	user := x.f.userIDs[0]
	var grant int64
	if err := x.pool.QueryRow(ctx, `INSERT INTO store_credit_grants(user_id,amount_cents,remaining_cents,expires_at) VALUES($1,1000,200,now()-interval '1 day') RETURNING id`, user).Scan(&grant); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(ctx, `INSERT INTO store_credit_allocations(order_id,grant_id,amount_cents,status) VALUES($1,$2,800,'committed')`, x.order.ID, grant); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := WithinTransaction(ctx, x.pool, func(ctx context.Context, tx pgx.Tx) error {
			return restoreOrderCredit(ctx, tx, x.order.ID, 500, fmt.Sprintf("refund-test:%d", x.order.ID))
		}); err != nil {
			t.Fatal(err)
		}
	}
	var remaining int64
	var grace bool
	if err := x.pool.QueryRow(ctx, `SELECT remaining_cents,expires_at>=now()+interval '29 days' FROM store_credit_grants WHERE source_grant_id=$1`, grant).Scan(&remaining, &grace); err != nil || remaining != 500 || !grace {
		t.Fatalf("restoration %d %v %v", remaining, grace, err)
	}
}

func TestExpiredUnspentCreditIsNotRevivedByRefund(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := context.Background()
	user := x.f.userIDs[0]
	var grant int64
	if err := x.pool.QueryRow(ctx, `INSERT INTO store_credit_grants(user_id,amount_cents,remaining_cents,expires_at) VALUES($1,1000,800,now()-interval '1 day') RETURNING id`, user).Scan(&grant); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(ctx, `INSERT INTO store_credit_allocations(order_id,grant_id,amount_cents,status) VALUES($1,$2,200,'committed')`, x.order.ID, grant); err != nil {
		t.Fatal(err)
	}
	if err := WithinTransaction(ctx, x.pool, func(ctx context.Context, tx pgx.Tx) error {
		return restoreOrderCredit(ctx, tx, x.order.ID, 100, fmt.Sprintf("refund-expiry:%d", x.order.ID))
	}); err != nil {
		t.Fatal(err)
	}
	amount, err := creditAvailable(ctx, x.pool, user)
	if err != nil || amount != 100 {
		t.Fatalf("revived expired balance: %d %v", amount, err)
	}
}
