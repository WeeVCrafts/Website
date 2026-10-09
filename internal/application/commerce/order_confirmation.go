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
}

func NewOrderConfirmationProcessor(events ports.OutboxStore, source OrderConfirmationSource, sender ports.EmailSender, publicURL string) *OrderConfirmationProcessor {
	return &OrderConfirmationProcessor{events, source, sender, strings.TrimRight(publicURL, "/")}
}

func (p *OrderConfirmationProcessor) Process(ctx context.Context, limit int) (int, error) {
	events, err := p.events.ClaimByType(ctx, "OrderCreated", limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	var firstErr error
	for _, event := range events {
		recipient, enabled, deliveryErr := p.source.PrepareOrderConfirmation(ctx, event.ID, event.AggregateID)
		if deliveryErr == nil && enabled {
			if p.sender == nil {
				deliveryErr = fmt.Errorf("order confirmation email is not configured")
			} else {
				link := p.publicURL + "/orders/" + url.PathEscape(event.AggregateID)
				deliveryErr = p.sender.Send(ctx, ports.EmailMessage{To: recipient.Email, DisplayName: recipient.DisplayName, Subject: "Your Weevcrafts order has been received", ActionURL: link, Body: fmt.Sprintf("We received order %s.\n\nYour order is saved. This message does not confirm payment. View its current payment and delivery status: %s", event.AggregateID, link)})
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
