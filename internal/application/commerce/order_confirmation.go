package commerce

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

// OrderConfirmationSource records an in-app notice idempotently and returns
// the current recipient and their transactional email preference.
type OrderConfirmationSource interface {
	PrepareOrderConfirmation(context.Context, int64, string) (ports.EmailRecipient, bool, error)
}

type OrderConfirmationProcessor struct {
	events    ports.OutboxStore
	source    OrderConfirmationSource
	sender    ports.EmailSender
	publicURL string
	eventType string
}

func NewOrderConfirmationProcessor(events ports.OutboxStore, source OrderConfirmationSource, sender ports.EmailSender, publicURL string) *OrderConfirmationProcessor {
	return &OrderConfirmationProcessor{events: events, source: source, sender: sender, publicURL: strings.TrimRight(publicURL, "/"), eventType: "OrderCreated"}
}

type PaidOrderConfirmationSource interface {
	PreparePaidOrderConfirmation(context.Context, int64, string) (ports.EmailRecipient, bool, error)
}

func NewPaidOrderConfirmationProcessor(events ports.OutboxStore, source OrderConfirmationSource, sender ports.EmailSender, publicURL string) *OrderConfirmationProcessor {
	p := NewOrderConfirmationProcessor(events, source, sender, publicURL)
	p.eventType = "OrderPaid"
	return p
}

func (p *OrderConfirmationProcessor) Process(ctx context.Context, limit int) (int, error) {
	events, err := p.events.ClaimByType(ctx, p.eventType, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	var firstErr error
	for _, event := range events {
		var recipient ports.EmailRecipient
		var enabled bool
		var deliveryErr error
		if p.eventType == "OrderPaid" {
			if source, ok := p.source.(PaidOrderConfirmationSource); ok {
				recipient, enabled, deliveryErr = source.PreparePaidOrderConfirmation(ctx, event.ID, event.AggregateID)
			} else {
				deliveryErr = fmt.Errorf("paid order confirmation unavailable")
			}
		} else {
			recipient, enabled, deliveryErr = p.source.PrepareOrderConfirmation(ctx, event.ID, event.AggregateID)
		}
		if deliveryErr == nil && enabled {
			if p.sender == nil {
				deliveryErr = fmt.Errorf("order confirmation email is not configured")
			} else {
				link := p.publicURL + "/orders/" + url.PathEscape(event.AggregateID)
				tracking := link + "/tracking"
				subject := "Your Weevcrafts order has been received"
				body := fmt.Sprintf("We received order %s.\n\nYour order is saved. This message does not confirm payment. View your order and payment status: %s\n\nTrack delivery: %s\nTracking details will appear when your maker prepares the shipment.", event.AggregateID, link, tracking)
				if p.eventType == "OrderPaid" {
					subject = "Your Weevcrafts order is confirmed"
					body = fmt.Sprintf("Payment confirmed for order %s. Your maker can now prepare your pieces.\n\nView your order: %s\n\nTrack delivery: %s\nTracking details will appear when your maker prepares the shipment. Sign in with the account used for this purchase.", event.AggregateID, link, tracking)
					link = tracking
				}
				deliveryErr = p.sender.Send(ctx, ports.EmailMessage{To: recipient.Email, DisplayName: recipient.DisplayName, Subject: subject, ActionURL: link, Body: body})
			}
		}
		if deliveryErr != nil {
			if err := p.events.Fail(ctx, event.ID, time.Minute*5, deliveryErr.Error()); err != nil {
				deliveryErr = err
			}
			if firstErr == nil {
				firstErr = deliveryErr
			}
			continue
		}
		if err := p.events.Ack(ctx, event.ID); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		processed++
	}
	return processed, firstErr
}
