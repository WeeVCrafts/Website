package commerce

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

const SellerOrderShippedEventType = "SellerOrderShipped"

type SellerDispatchProcessor struct {
	events    ports.OutboxStore
	contacts  ports.SellerDispatchContactSource
	sender    ports.EmailSender
	publicURL string
}

func NewSellerDispatchProcessor(events ports.OutboxStore, contacts ports.SellerDispatchContactSource, sender ports.EmailSender, publicURL string) *SellerDispatchProcessor {
	return &SellerDispatchProcessor{events: events, contacts: contacts, sender: sender, publicURL: strings.TrimRight(publicURL, "/")}
}

func (p *SellerDispatchProcessor) Process(ctx context.Context, limit int) (int, error) {
	if p.events == nil || (p.sender != nil && p.contacts == nil) {
		return 0, fmt.Errorf("seller dispatch worker unavailable")
	}
	events, err := p.events.ClaimByType(ctx, SellerOrderShippedEventType, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	var processErr error
	for _, event := range events {
		var payload struct {
			SellerID       int64  `json:"seller_id"`
			OrderNumber    string `json:"order_number"`
			Carrier        string `json:"carrier"`
			TrackingNumber string `json:"tracking_number"`
		}
		if err = json.Unmarshal(event.Payload, &payload); err != nil || payload.SellerID <= 0 || payload.OrderNumber == "" {
			if err == nil {
				err = fmt.Errorf("invalid seller dispatch event")
			}
			_ = p.events.Fail(ctx, event.ID, time.Hour, err.Error())
			if processErr == nil {
				processErr = err
			}
			continue
		}
		if p.sender != nil {
			contact, lookupErr := p.contacts.LookupSellerDispatchContact(ctx, payload.SellerID, payload.OrderNumber)
			if lookupErr != nil {
				_ = p.events.Fail(ctx, event.ID, time.Minute, lookupErr.Error())
				if processErr == nil {
					processErr = lookupErr
				}
				continue
			}
			trackingURL := p.publicURL + "/orders/" + url.PathEscape(payload.OrderNumber) + "/tracking"
			body := fmt.Sprintf("Your Weevcrafts order %s has shipped.\n\nCarrier: %s\nTracking number: %s\n\nTrack your order: %s", payload.OrderNumber, payload.Carrier, payload.TrackingNumber, trackingURL)
			if sendErr := p.sender.Send(ctx, ports.EmailMessage{To: contact.Email, DisplayName: contact.DisplayName, Subject: "Your Weevcrafts order has shipped", ActionURL: trackingURL, Body: body}); sendErr != nil {
				_ = p.events.Fail(ctx, event.ID, time.Minute, sendErr.Error())
				if processErr == nil {
					processErr = sendErr
				}
				continue
			}
		}
		if ackErr := p.events.Ack(ctx, event.ID); ackErr != nil {
			if processErr == nil {
				processErr = ackErr
			}
			continue
		}
		processed++
	}
	return processed, processErr
}
