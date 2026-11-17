package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestEmailDeliveryReplayEarlyCallbackAndThirtyDayExpiry(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := t.Context()
	repository := NewEmailDeliveryRepository(pool)
	message := ports.EmailMessage{IdempotencyKey: "mail-fixture-" + uuid.NewString(), OrderNumber: "WC-EMAIL-FIXTURE", DeliveryKind: "OrderPaid"}
	attempt, err := repository.BeginEmailDelivery(ctx, message)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM order_email_deliveries WHERE id=$1::uuid`, attempt.ID)
	})
	again, err := repository.BeginEmailDelivery(ctx, message)
	if err != nil || again.ID != attempt.ID {
		t.Fatalf("retry duplicated delivery: %v", err)
	}
	providerID := uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	events := []ports.EmailDeliveryEvent{
		{ID: "msg-delivered-" + attempt.ID, DeliveryID: attempt.ID, ProviderID: providerID, State: "delivered", At: now},
		{ID: "msg-sent-" + attempt.ID, DeliveryID: attempt.ID, ProviderID: providerID, State: "accepted", At: now.Add(time.Minute)},
		{ID: "msg-delay-" + attempt.ID, DeliveryID: attempt.ID, ProviderID: providerID, State: "delayed", At: now.Add(2 * time.Minute)},
	}
	for _, e := range events {
		if err := repository.RecordEmailDeliveryEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
		if err := repository.RecordEmailDeliveryEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.FinishEmailDelivery(ctx, attempt.ID, providerID, "accepted"); err != nil {
		t.Fatal(err)
	}
	rows, err := repository.List(ctx, message.OrderNumber, "")
	if err != nil || len(rows) != 1 || rows[0].State != "delivered" || rows[0].ProviderID != providerID {
		t.Fatalf("early delivery lost: %+v err=%v", rows, err)
	}
	for i, state := range []string{"bounced", "complained", "delivered"} {
		e := ports.EmailDeliveryEvent{ID: fmt.Sprintf("msg-state-%d-%s", i, attempt.ID), DeliveryID: attempt.ID, ProviderID: providerID, State: state, At: now.Add(time.Duration(i+3) * time.Minute)}
		if err := repository.RecordEmailDeliveryEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = repository.List(ctx, message.OrderNumber, "")
	if err != nil || len(rows) != 1 || rows[0].State != "complained" {
		t.Fatalf("complaint overwritten: %+v %v", rows, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM order_email_events WHERE delivery_id=$1::uuid`, attempt.ID).Scan(&count); err != nil || count != 6 {
		t.Fatalf("event dedupe count=%d err=%v", count, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE order_email_deliveries SET started_at=now()-interval '31 days' WHERE id=$1::uuid`, attempt.ID); err != nil {
		t.Fatal(err)
	}
	if err := repository.ExpireEmailDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM order_email_events WHERE delivery_id=$1::uuid`, attempt.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("expired events retained")
	}
}
