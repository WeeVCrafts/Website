package payment

import (
	"context"
	"errors"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

func (s *OrderService) completePaymentCreation(ctx context.Context, repository ports.PaymentCreationRepository, gateway ports.PaymentOrderLookupGateway, claim ports.PaymentCreationClaim) (ports.PaymentOrderRecord, error) {
	var order ports.PaymentOrder
	if claim.Submit {
		created, err := s.gateway.CreateOrder(ctx, ports.PaymentOrderRequest{
			Reference: claim.Order.OrderNumber, AmountCents: claim.Order.AmountCents, Currency: claim.Order.Currency,
		})
		s.observeProvider("creation", err)
		if errors.Is(err, ports.ErrPaymentMismatch) {
			return s.reviewCreation(ctx, repository, claim, "provider create response did not match the saved order")
		}
		if err == nil {
			order = created
		}
	}
	if order.ProviderOrderID == "" {
		matches, err := gateway.LookupOrdersByReceipt(ctx, claim.Order.OrderNumber)
		switch {
		case errors.Is(err, ports.ErrPaymentMismatch), err == nil && len(matches) > 1:
			s.observeCreation("lookup", "mismatch")
			return s.reviewCreation(ctx, repository, claim, "receipt lookup did not identify one matching provider order")
		case err != nil:
			s.observeProvider("lookup", err)
			return ports.PaymentOrderRecord{}, ports.ErrPaymentCreationPending
		case len(matches) == 0:
			s.observeCreation("lookup", "missing")
			return ports.PaymentOrderRecord{}, ports.ErrPaymentCreationPending
		}
		order = matches[0]
		if order.ProviderOrderID != "" && order.AmountCents == claim.Order.AmountCents && order.Currency == claim.Order.Currency {
			s.observeCreation("lookup", "success")
		} else {
			s.observeCreation("lookup", "mismatch")
		}
	}
	if order.ProviderOrderID == "" || order.AmountCents != claim.Order.AmountCents || order.Currency != claim.Order.Currency {
		return s.reviewCreation(ctx, repository, claim, "provider order amount or currency does not match the saved order")
	}
	attached, err := repository.AttachClaimedProviderOrder(ctx, claim, order)
	if errors.Is(err, ports.ErrPaymentMismatch) {
		return s.reviewCreation(ctx, repository, claim, "provider order reference conflicts with a saved payment attempt")
	}
	if err != nil {
		return ports.PaymentOrderRecord{}, ports.ErrPaymentCreationPending
	}
	return attached, nil
}

func (s *OrderService) reviewCreation(ctx context.Context, repository ports.PaymentCreationRepository, claim ports.PaymentCreationClaim, reason string) (ports.PaymentOrderRecord, error) {
	reviewCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if err := repository.ReviewPaymentCreation(reviewCtx, claim, reason); err != nil {
		return ports.PaymentOrderRecord{}, ports.ErrPaymentCreationPending
	}
	return ports.PaymentOrderRecord{}, ports.ErrPaymentCreationReview
}

func (s *OrderService) observeProvider(operation string, err error) {
	var timedOut interface{ Timeout() bool }
	switch {
	case err == nil:
		s.observeCreation(operation, "success")
	case errors.Is(err, ports.ErrPaymentMismatch):
		s.observeCreation(operation, "mismatch")
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &timedOut) && timedOut.Timeout():
		s.observeCreation(operation, "timeout")
	default:
		s.observeCreation(operation, "error")
	}
}

func (s *OrderService) observeCreation(operation, outcome string) {
	if s.observe != nil {
		s.observe(operation, outcome)
	}
}

func (s *OrderService) reconcilePaymentCreations(ctx context.Context, limit int) (int, error) {
	gateway, gatewayOK := s.gateway.(ports.PaymentOrderLookupGateway)
	repository, repositoryOK := s.repository.(ports.PaymentCreationRepository)
	if !gatewayOK || !repositoryOK {
		return 0, nil
	}
	claims, err := repository.ClaimPaymentCreationRecovery(ctx, limit)
	if err != nil {
		return 0, err
	}
	completed := 0
	var failures error
	for _, claim := range claims {
		if err := ctx.Err(); err != nil {
			return completed, err
		}
		if _, err := s.completePaymentCreation(ctx, repository, gateway, claim); err == nil {
			completed++
		} else {
			failures = errors.Join(failures, err)
		}
	}
	return completed, failures
}
