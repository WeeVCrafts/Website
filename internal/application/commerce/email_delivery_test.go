package commerce

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

type deliveryStore struct {
	attempt             ports.EmailDeliveryAttempt
	savedID, savedState string
	failSave            bool
}

func (s *deliveryStore) BeginEmailDelivery(context.Context, ports.EmailMessage) (ports.EmailDeliveryAttempt, error) {
	return s.attempt, nil
}
func (s *deliveryStore) FinishEmailDelivery(_ context.Context, _ string, id, state string) error {
	if s.failSave {
		return errors.New("database unavailable")
	}
	s.savedID, s.savedState = id, state
	return nil
}
func (*deliveryStore) RecordEmailDeliveryEvent(context.Context, ports.EmailDeliveryEvent) error {
	return nil
}

type receiptSender struct {
	calls      int
	err        error
	deliveryID string
}

func (s *receiptSender) SendWithReceipt(_ context.Context, m ports.EmailMessage) (string, error) {
	s.calls++
	s.deliveryID = m.DeliveryID
	return "provider-fixture", s.err
}

func TestOrderEmailReceiptIsSavedAndAmbiguousRetriesStop(t *testing.T) {
	for _, tc := range []struct {
		name, state, provider string
		age                   time.Duration
		sendErr               error
		wantCalls             int
		wantState             string
	}{
		{"new", "submitting", "", 0, nil, 1, "accepted"},
		{"already delivered", "delivered", "provider-fixture", 0, nil, 0, ""},
		{"ambiguous timeout", "submitting", "", 0, errors.New("timeout"), 1, ""},
		{"expired retry window", "submitting", "", 24 * time.Hour, nil, 0, "uncertain"},
		{"rejected", "submitting", "", 0, ports.ErrEmailRejected, 1, "rejected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &deliveryStore{attempt: ports.EmailDeliveryAttempt{ID: "delivery-fixture", State: tc.state, ProviderID: tc.provider, StartedAt: time.Now().Add(-tc.age)}}
			sender := &receiptSender{err: tc.sendErr}
			err := (TrackedOrderEmail{sender, store}).Send(t.Context(), ports.EmailMessage{IdempotencyKey: "paid-42", OrderNumber: "WC-42", DeliveryKind: "OrderPaid", DeliveryQueuedAt: time.Now()})
			if sender.calls != tc.wantCalls || store.savedState != tc.wantState {
				t.Fatalf("calls=%d state=%q", sender.calls, store.savedState)
			}
			if tc.name == "ambiguous timeout" && err == nil {
				t.Fatal("uncertain send was acknowledged")
			}
			if tc.wantCalls > 0 && sender.deliveryID != "delivery-fixture" {
				t.Fatal("callback correlation missing")
			}
		})
	}
}
