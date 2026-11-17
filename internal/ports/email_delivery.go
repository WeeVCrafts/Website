package ports

import (
	"context"
	"errors"
	"time"
)

var ErrEmailRejected = errors.New("email request rejected")

type EmailReceiptSender interface {
	SendWithReceipt(context.Context, EmailMessage) (string, error)
}

type EmailDeliveryAttempt struct {
	ID, ProviderID, State string
	StartedAt             time.Time
}

type EmailDeliveryEvent struct {
	ID, DeliveryID, ProviderID, State string
	At                                time.Time
}

type EmailDeliveryStore interface {
	BeginEmailDelivery(context.Context, EmailMessage) (EmailDeliveryAttempt, error)
	FinishEmailDelivery(ctx context.Context, deliveryID, providerID, state string) error
	RecordEmailDeliveryEvent(context.Context, EmailDeliveryEvent) error
}
