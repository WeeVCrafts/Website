package payment

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/wecratfs/commerce/internal/ports"
)

func (s *OrderService) Reconcile(ctx context.Context, limit int) (int, error) {
	gateway, ok := s.gateway.(ports.PaymentStatusGateway)
	if !ok {
		return 0, nil
	}
	repository, ok := s.repository.(ports.PaymentRecoveryRepository)
	if !ok {
		return 0, nil
	}
	webhooks, ok := s.repository.(ports.PaymentWebhookRepository)
	if !ok {
		return 0, nil
	}
	if limit < 1 || limit > 100 {
		return 0, ports.ErrPaymentState
	}
	records, err := repository.ClaimPaymentReconciliation(ctx, limit)
	if err != nil {
		return 0, err
	}
	total := 0
	var failures error
	for _, record := range records {
		if err := recoverProviderOrder(ctx, gateway, webhooks, record); err != nil {
			failures = errors.Join(failures, err)
		} else {
			total++
		}
	}
	return total, failures
}

func (s *OrderService) Refresh(ctx context.Context, userID int64, number string) error {
	record, err := s.repository.GetPaymentOrder(ctx, userID, number)
	if err != nil {
		return err
	}
	if record.ProviderOrderID == "" {
		return nil
	}
	gateway, ok := s.gateway.(ports.PaymentStatusGateway)
	if !ok {
		return nil
	}
	repository, ok := s.repository.(ports.PaymentRecoveryRepository)
	if !ok {
		return nil
	}
	webhooks, ok := s.repository.(ports.PaymentWebhookRepository)
	if !ok {
		return nil
	}
	claimed, err := repository.ClaimOrderPaymentReconciliation(ctx, userID, number)
	if err != nil || !claimed {
		return err
	}
	return recoverProviderOrder(ctx, gateway, webhooks, record)
}

func recoverProviderOrder(ctx context.Context, gateway ports.PaymentStatusGateway, repository ports.PaymentWebhookRepository, record ports.PaymentOrderRecord) error {
	events, err := gateway.LookupPayments(ctx, record)
	if err != nil {
		return err
	}
	for _, event := range events {
		if event.ProviderOrderID != record.ProviderOrderID || event.ProviderPaymentID == "" || event.AmountCents != record.AmountCents || event.Currency != record.Currency || !supportedWebhookEvent(event) {
			return ports.ErrPaymentMismatch
		}
	}
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err = repository.ProcessWebhook(ctx, event, payload); err != nil {
			return err
		}
	}
	return nil
}
