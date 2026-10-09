package commerce

import (
	"context"
	"errors"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

type confirmationEvents struct{ dispatchEvents }

func (e *confirmationEvents) ClaimByType(_ context.Context, kind string, _ int) ([]ports.OutboxEvent, error) {
	if kind != "OrderCreated" {
		return nil, errors.New("unexpected event type")
	}
	return e.events, nil
}

type confirmationSource struct{ enabled bool }

func (s confirmationSource) PrepareOrderConfirmation(context.Context, int64, string) (ports.EmailRecipient, bool, error) {
	return ports.EmailRecipient{Email: "customer@example.test", DisplayName: "Customer"}, s.enabled, nil
}

type confirmationSender struct {
	calls int
	err   error
}

func (s *confirmationSender) Send(context.Context, ports.EmailMessage) error { s.calls++; return s.err }

func TestOrderConfirmationDelivery(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		enabled, configured, fail bool
		ack, sends                int
	}{
		{"delivered", true, true, false, 1, 1},
		{"opted out", false, true, false, 1, 0},
		{"SMTP failure remains retryable", true, true, true, 0, 1},
		{"unconfigured remains retryable", true, false, false, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := &confirmationEvents{dispatchEvents: dispatchEvents{events: []ports.OutboxEvent{{ID: 1, AggregateID: "WC-1"}}}}
			sender := &confirmationSender{}
			if tc.fail {
				sender.err = errors.New("SMTP unavailable")
			}
			var delivery ports.EmailSender
			if tc.configured {
				delivery = sender
			}
			processor := NewOrderConfirmationProcessor(events, confirmationSource{tc.enabled}, delivery, "http://localhost:8080")
			count, err := processor.Process(context.Background(), 10)
			if count != tc.ack || len(events.acked) != tc.ack || sender.calls != tc.sends {
				t.Fatalf("Process() = count %d, acknowledgements %d, sends %d; want %d, %d, %d", count, len(events.acked), sender.calls, tc.ack, tc.ack, tc.sends)
			}
			if (err != nil) != (tc.ack == 0) {
				t.Fatalf("Process() error = %v; want failure %t", err, tc.ack == 0)
			}
		})
	}
}
