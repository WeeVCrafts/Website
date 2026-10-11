package commerce

import (
	"context"
	"github.com/wecratfs/commerce/internal/ports"
	"strings"
	"testing"
)

type paidConfirmationEvents struct{ dispatchEvents }

func (e *paidConfirmationEvents) ClaimByType(_ context.Context, kind string, _ int) ([]ports.OutboxEvent, error) {
	if kind == "OrderPaid" {
		return e.events, nil
	}
	return nil, nil
}

type paidConfirmationSource struct{ confirmationSource }

func (s paidConfirmationSource) PreparePaidOrderConfirmation(ctx context.Context, id int64, n string) (ports.EmailRecipient, bool, error) {
	return s.PrepareOrderConfirmation(ctx, id, n)
}

func TestPaidConfirmationIncludesAuthenticatedTrackingLink(t *testing.T) {
	events := &paidConfirmationEvents{dispatchEvents: dispatchEvents{events: []ports.OutboxEvent{{ID: 42, AggregateID: "WC-42"}}}}
	sender := &dispatchSender{}
	processor := NewPaidOrderConfirmationProcessor(events, paidConfirmationSource{confirmationSource{enabled: true}}, sender, "https://shop.example.test/")
	n, err := processor.Process(context.Background(), 10)
	if err != nil || n != 1 || len(events.acked) != 1 {
		t.Fatalf("n=%d err=%v ack=%v", n, err, events.acked)
	}
	if sender.message.Subject != "Your Weevcrafts order is confirmed" || sender.message.ActionURL != "https://shop.example.test/orders/WC-42/tracking" || !strings.Contains(sender.message.Body, "Payment confirmed") {
		t.Fatalf("unexpected message: %+v", sender.message)
	}
}
