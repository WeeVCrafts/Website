package payment

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

func (s *OrderService) Reconcile(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		return 0, ports.ErrPaymentState
	}
	// A slow receipt lookup must leave time for capture reconciliation.
	creationContext, cancelCreation := context.WithTimeout(ctx, 15*time.Second)
	created, creationErr := s.reconcilePaymentCreations(creationContext, min(limit, 3))
	cancelCreation()
	gateway, ok := s.gateway.(ports.PaymentStatusGateway)
	if !ok {
		return created, creationErr
	}
	repository, ok := s.repository.(ports.PaymentRecoveryRepository)
	if !ok {
		return created, creationErr
	}
	webhooks, ok := s.repository.(ports.PaymentWebhookRepository)
	if !ok {
		return created, creationErr
	}
	records, err := repository.ClaimPaymentReconciliation(ctx, limit)
	if err != nil {
		return created, errors.Join(creationErr, err)
	}
	total := 0
	var failures error
	for _, record := range records {
		if err := s.recoverProviderOrder(ctx, gateway, webhooks, record); err != nil {
			failures = errors.Join(failures, err)
		} else {
			total++
		}
	}
	return created + total, errors.Join(creationErr, failures)
}

func (s *OrderService) Refresh(ctx context.Context, userID int64, number string) error {
	record, err := s.repository.GetPaymentOrder(ctx, userID, number)
	if err != nil {
		return err
	}
	if record.ProviderOrderID == "" {
		gateway, gatewayOK := s.gateway.(ports.PaymentOrderLookupGateway)
		repository, repositoryOK := s.repository.(ports.PaymentCreationRepository)
		if !gatewayOK || !repositoryOK {
			return nil
		}
		claim, err := repository.ClaimExistingPaymentCreation(ctx, userID, number)
		if errors.Is(err, ports.ErrPaymentCreationPending) || errors.Is(err, ports.ErrPaymentCreationReview) || errors.Is(err, ports.ErrPaymentState) {
			return nil
		}
		if err != nil || claim.AttemptID == 0 {
			return err
		}
		record, err = s.completePaymentCreation(ctx, repository, gateway, claim)
		if errors.Is(err, ports.ErrPaymentCreationPending) || errors.Is(err, ports.ErrPaymentCreationReview) {
			return nil
		}
		if err != nil {
			return err
		}
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
	return s.recoverProviderOrder(ctx, gateway, webhooks, record)
}

func (s *OrderService) recoverProviderOrder(ctx context.Context, gateway ports.PaymentStatusGateway, repository ports.PaymentWebhookRepository, record ports.PaymentOrderRecord) error {
	events, err := gateway.LookupPayments(ctx, record)
	if err != nil {
		s.observeProvider("lookup", err)
		return err
	}
	for _, event := range events {
		if event.ProviderOrderID != record.ProviderOrderID || event.ProviderPaymentID == "" || event.AmountCents != record.AmountCents || event.Currency != record.Currency || !supportedWebhookEvent(event) {
			s.observeCreation("lookup", "mismatch")
			return ports.ErrPaymentMismatch
		}
	}
	s.observeCreation("lookup", "success")
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err = repository.ProcessWebhook(ctx, event, payload); err != nil {
			return err
		}
	}
	if tracker, ok := repository.(ports.PaymentLookupResultRepository); ok {
		return tracker.RecordPaymentLookup(ctx, record.OrderID, events)
	}
	return nil
}
