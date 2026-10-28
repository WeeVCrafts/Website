package payment

import (
	"context"
	"errors"
	"strings"

	"github.com/wecratfs/commerce/internal/ports"
)

var ErrWebhookRejected = errors.New("payment webhook rejected")

type Service struct {
	verifier   ports.PaymentWebhookVerifier
	repository ports.PaymentWebhookRepository
}

func NewService(verifier ports.PaymentWebhookVerifier, repository ports.PaymentWebhookRepository) *Service {
	return &Service{verifier: verifier, repository: repository}
}

func (s *Service) HandleWebhook(ctx context.Context, rawBody []byte, signature string, deliveryEventID ...string) (bool, error) {
	if s.verifier == nil || s.repository == nil || len(rawBody) == 0 || len(rawBody) > 1<<20 {
		return false, ErrWebhookRejected
	}
	webhook, err := s.verifier.VerifyWebhook(ctx, rawBody, signature)
	if err != nil {
		return false, ErrWebhookRejected
	}
	if !supportedWebhookEvent(webhook) {
		return false, ErrWebhookRejected
	}
	// Only use delivery metadata after authenticating the original raw body.
	if len(deliveryEventID) > 0 && strings.TrimSpace(deliveryEventID[0]) != "" {
		webhook.EventID = strings.TrimSpace(deliveryEventID[0])
	}
	processed, err := s.repository.ProcessWebhook(ctx, webhook, rawBody)
	if err != nil {
		return false, err
	}
	return processed, nil
}

func supportedWebhookEvent(webhook ports.PaymentWebhook) bool {
	switch webhook.EventType {
	case "payment.captured", "order.paid", "payment.failed", "payment.authorized", "payment.pending":
		return true
	default:
		return false
	}
}
