package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

func TestTransactionalSMSCommitsWithBusinessNoticesAndDoesNotReplay(t *testing.T) {
	x := newMarketplaceFixtureWithPool(t, isolatedRolePool(t, false))
	x.capture(t)
	ctx := context.Background()
	repo := NewSMSRepository(x.pool)
	if _, err := x.pool.Exec(ctx, `UPDATE users SET phone='9876543210' WHERE id IN ($1,$2)`, x.f.userIDs[0], x.f.ownerID); err != nil {
		t.Fatal(err)
	}
	// Rolling back a business notice also rolls back its SMS and durable job.
	tx, err := x.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var rollbackID int64
	if err = tx.QueryRow(ctx, `INSERT INTO notifications(user_id,kind,title,body,link_url) VALUES($1,'order','Order confirmed','Private body must not leave the app',$2) RETURNING id`, x.f.userIDs[0], "/orders/"+x.order.OrderNumber).Scan(&rollbackID); err != nil {
		t.Fatal(err)
	}
	tx.Rollback(ctx)
	var count int
	if err = x.pool.QueryRow(ctx, `SELECT count(*) FROM transactional_sms WHERE notification_id=$1`, rollbackID).Scan(&count); err != nil || count != 0 {
		t.Fatal("rolled back notice sent SMS", count, err)
	}
	for i := 0; i < 2; i++ {
		if err = x.repo.NotifySellers(ctx, x.order.ID); err != nil {
			t.Fatal(err)
		}
	}
	var eventID int64
	if err = x.pool.QueryRow(ctx, `SELECT id FROM outbox_events WHERE event_type='OrderPaid' AND aggregate_id=$1`, x.order.OrderNumber).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, _, err = x.commerce.PreparePaidOrderConfirmation(ctx, eventID, x.order.OrderNumber); err != nil {
			t.Fatal(err)
		}
	}
	if err = x.pool.QueryRow(ctx, `SELECT count(*) FROM transactional_sms WHERE user_id IN ($1,$2) AND reference=$3`, x.f.userIDs[0], x.f.ownerID, x.order.OrderNumber).Scan(&count); err != nil || count != 2 {
		t.Fatal("business replay duplicated SMS", count, err)
	}
	first, claimed, err := repo.ClaimSMS(ctx)
	if err != nil || !claimed {
		t.Fatal("first claim", claimed, err)
	}
	if first.TemplateKey != "seller_order" || first.UserID != x.f.ownerID || first.Update != "New paid order" {
		t.Fatal("seller recipient leaked", first.TemplateKey, first.UserID)
	}
	if err = repo.FinishSMS(ctx, first.ID, ports.SMSResult{Outcome: "accepted", ProviderID: fmt.Sprintf("fixture-request-%d", first.ID)}); err != nil {
		t.Fatal(err)
	}
	second, claimed, err := repo.ClaimSMS(ctx)
	if err != nil || !claimed || second.UserID != x.f.userIDs[0] {
		t.Fatal("customer claim", claimed, err)
	}
	// Simulate a process crash after the persisted send intent.
	if _, err = x.pool.Exec(ctx, `UPDATE transactional_sms SET submitted_at=now()-interval '6 minutes' WHERE id=$1`, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, claimed, err = repo.ClaimSMS(ctx); err != nil || claimed {
		t.Fatal("ambiguous SMS was resubmitted", claimed, err)
	}
	var status string
	if err = x.pool.QueryRow(ctx, `SELECT status FROM transactional_sms WHERE id=$1`, second.ID).Scan(&status); err != nil || status != "uncertain" {
		t.Fatal(status, err)
	}
	// An opt-out is rechecked at dispatch, not just when the notice is saved.
	if _, err = x.pool.Exec(ctx, `INSERT INTO notification_preferences(user_id,order_updates) VALUES($1,false) ON CONFLICT(user_id) DO UPDATE SET order_updates=false`, x.f.userIDs[0]); err != nil {
		t.Fatal(err)
	}
	var notice int64
	if err = x.pool.QueryRow(ctx, `INSERT INTO notifications(user_id,kind,title,body,link_url) VALUES($1,'order','Package delivered','Proof remains private',$2) RETURNING id`, x.f.userIDs[0], "/orders/"+x.order.OrderNumber).Scan(&notice); err != nil {
		t.Fatal(err)
	}
	if _, claimed, err = repo.ClaimSMS(ctx); err != nil || claimed {
		t.Fatal("opted-out account was contacted", claimed, err)
	}
	if err = x.pool.QueryRow(ctx, `SELECT status FROM transactional_sms WHERE notification_id=$1`, notice).Scan(&status); err != nil || status != "skipped" {
		t.Fatal(status, err)
	}
	// Exception return notices use a generic state, never the customer's reason.
	var item, ret int64
	if err = x.pool.QueryRow(ctx, `SELECT id FROM order_items WHERE order_id=$1 LIMIT 1`, x.order.ID).Scan(&item); err != nil {
		t.Fatal(err)
	}
	if err = x.pool.QueryRow(ctx, `INSERT INTO marketplace_returns(order_id,seller_id,order_item_id,quantity,reason,request_key) VALUES($1,$2,$3,1,'Sensitive inspection details',$4) RETURNING id`, x.order.ID, x.seller, item, fmt.Sprintf("sms-return-%d", x.order.ID)).Scan(&ret); err != nil {
		t.Fatal(err)
	}
	if err = x.pool.QueryRow(ctx, `SELECT count(*) FROM transactional_sms WHERE reference=$1 AND update_label='Return requested'`, x.order.OrderNumber).Scan(&count); err != nil || count != 2 {
		t.Fatal("return notices missing", count, err)
	}
	// Current role changes also make the saved recipient ineligible.
	if _, err = x.pool.Exec(ctx, `UPDATE user_roles SET role_slug='customer' WHERE user_id=$1 AND role_slug='seller'`, x.f.ownerID); err != nil {
		t.Fatal(err)
	}
	if _, claimed, err = repo.ClaimSMS(ctx); err != nil || claimed {
		t.Fatal("previous seller role retained SMS access", claimed, err)
	}
}
