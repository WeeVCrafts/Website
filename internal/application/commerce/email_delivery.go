package commerce

import (
	"context"
	"errors"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

type TrackedOrderEmail struct {
	Sender ports.EmailReceiptSender
	Store  ports.EmailDeliveryStore
}

func (s TrackedOrderEmail) Send(ctx context.Context, message ports.EmailMessage) error {
	if message.IdempotencyKey == "" || message.OrderNumber == "" || message.DeliveryKind == "" {
		return errors.New("order email reference missing")
	}
	if !message.DeliveryQueuedAt.IsZero() && time.Since(message.DeliveryQueuedAt) >= 30*24*time.Hour {
		return nil
	}
	attempt, err := s.Store.BeginEmailDelivery(ctx, message)
	if err != nil {
		return errors.New("order email tracking unavailable")
	}
	if attempt.ProviderID != "" || attempt.State != "submitting" {
		return nil
	}
	// Stop before the provider's idempotency window expires, including when
	// the previous request reached the provider but its response was lost.
	if time.Since(attempt.StartedAt) >= 23*time.Hour {
		return s.finish(ctx, attempt.ID, "", "uncertain")
	}
	message.DeliveryID = attempt.ID
	id, err := s.Sender.SendWithReceipt(ctx, message)
	if errors.Is(err, ports.ErrEmailRejected) {
		return s.finish(ctx, attempt.ID, "", "rejected")
	}
	if err != nil {
		return err
	}
	return s.finish(ctx, attempt.ID, id, "accepted")
}

func (s TrackedOrderEmail) finish(ctx context.Context, id, providerID, state string) error {
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.Store.FinishEmailDelivery(saveCtx, id, providerID, state); err != nil {
		return errors.New("order email receipt could not be recorded")
	}
	return nil
}
