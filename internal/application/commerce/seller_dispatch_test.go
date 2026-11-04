package commerce

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

type dispatchEvents struct {
	events   []ports.OutboxEvent
	acked    []int64
	failures int
}

func (f *dispatchEvents) Publish(context.Context, ports.OutboxEventInput) (ports.OutboxEvent, error) {
	return ports.OutboxEvent{}, nil
}
func (f *dispatchEvents) Claim(context.Context, int) ([]ports.OutboxEvent, error) { return nil, nil }
func (f *dispatchEvents) ClaimByType(_ context.Context, kind string, _ int) ([]ports.OutboxEvent, error) {
	if kind != SellerOrderShippedEventType {
		return nil, nil
	}
	return f.events, nil
}
func (f *dispatchEvents) Ack(_ context.Context, id int64) error {
	f.acked = append(f.acked, id)
	return nil
}
func (f *dispatchEvents) Fail(context.Context, int64, time.Duration, string) error {
	f.failures++
	return nil
}

type dispatchContacts struct{ contact ports.SellerDispatchContact }

func (f dispatchContacts) LookupSellerDispatchContact(context.Context, int64, string) (ports.SellerDispatchContact, error) {
	return f.contact, nil
}

type dispatchSender struct{ message ports.EmailMessage }

func (f *dispatchSender) Send(_ context.Context, message ports.EmailMessage) error {
	f.message = message
	return nil
}

func TestSellerDispatchProcessorSendsTrackingEmailAndAcknowledgesOnce(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{"seller_id": 7, "order_number": "WC-42", "carrier": "India Post", "tracking_number": "TRK-42"})
	events := &dispatchEvents{events: []ports.OutboxEvent{{ID: 9, EventType: SellerOrderShippedEventType, Payload: payload}}}
	sender := &dispatchSender{}
	processor := NewSellerDispatchProcessor(events, dispatchContacts{ports.SellerDispatchContact{Email: "customer@example.test", DisplayName: "Customer"}}, sender, "https://shop.example.test/")
	processed, err := processor.Process(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if processed != 1 || len(events.acked) != 1 || events.failures != 0 {
		t.Fatalf("processed=%d acked=%v failures=%d", processed, events.acked, events.failures)
	}
	if sender.message.To != "customer@example.test" || sender.message.Subject != "Your Weevcrafts order has shipped" || sender.message.ActionURL != "https://shop.example.test/orders/WC-42/tracking" {
		t.Fatalf("unexpected dispatch email: %#v", sender.message)
	}
	if sender.message.Body == "" {
		t.Fatal("expected tracking details in message body")
	}
}

func TestSellerDispatchProcessorKeepsTrackingWithoutEmailConfiguration(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{"seller_id": 7, "order_number": "WC-42", "carrier": "India Post", "tracking_number": "TRK-42"})
	events := &dispatchEvents{events: []ports.OutboxEvent{{ID: 10, EventType: SellerOrderShippedEventType, Payload: payload}}}
	processor := NewSellerDispatchProcessor(events, nil, nil, "")
	processed, err := processor.Process(context.Background(), 10)
	if err != nil || processed != 1 || len(events.acked) != 1 {
		t.Fatalf("processed=%d acked=%v err=%v", processed, events.acked, err)
	}
}
