package payment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

var (
	ErrRefundCreationFailed   = errors.New("refund could not be created")
	ErrRefundResponseMismatch = errors.New("refund response does not match the order")
	ErrRefundInvalidRequest   = errors.New("refund request is invalid")
)

// RefundService coordinates a provider refund with the authoritative order
// state. PostgreSQL reserves the full-refund intent before the provider call;
// the order is only marked refunded after the provider response is validated.
type RefundService struct {
	gateway    ports.PaymentRefundGateway
	repository ports.PaymentRefundRepository
}

func NewRefundService(gateway ports.PaymentRefundGateway, repository ports.PaymentRefundRepository) *RefundService {
	return &RefundService{gateway: gateway, repository: repository}
}

func (s *RefundService) Refund(ctx context.Context, userID int64, orderNumber, requestKey, reason string) (domaincommerce.Refund, error) {
	if userID <= 0 || strings.TrimSpace(orderNumber) == "" || len(strings.TrimSpace(requestKey)) < 16 || len(requestKey) > 200 || len(strings.TrimSpace(reason)) > 500 {
		return domaincommerce.Refund{}, ErrRefundInvalidRequest
	}
	if s.gateway == nil || s.repository == nil {
		return domaincommerce.Refund{}, ports.ErrRefundState
	}
	record, err := s.repository.BeginRefund(ctx, userID, strings.TrimSpace(orderNumber), strings.TrimSpace(reason), strings.TrimSpace(requestKey))
	if err != nil {
		return domaincommerce.Refund{}, err
	}
	if record.Status == "processed" {
		return mapRefund(record), nil
	}
	// A pending record with a provider reference has already crossed the
	// external side-effect boundary. The reconciliation worker resolves it.
	if record.Status == "pending" && record.ProviderRefundID != "" {
		return mapRefund(record), nil
	}
	if record.Status != "pending" {
		return domaincommerce.Refund{}, ports.ErrRefundState
	}
	if recovery, ok := s.repository.(ports.RefundRecoveryRepository); ok {
		claimed, err := recovery.ClaimRefundSubmission(ctx, record.ID)
		if err != nil {
			return domaincommerce.Refund{}, err
		}
		if !claimed {
			return mapRefund(record), nil
		}
	}
	providerRefund, err := s.gateway.CreateRefund(ctx, ports.PaymentRefundRequest{
		ProviderPaymentID: record.ProviderPaymentID,
		AmountCents:       record.AmountCents,
		Currency:          record.Currency,
		Receipt:           fmt.Sprintf("refund-%d", record.ID),
	})
	if err != nil {
		_ = s.repository.FailRefund(ctx, record.ID, "provider response uncertain; reconciliation required")
		return domaincommerce.Refund{}, ErrRefundCreationFailed
	}
	if providerRefund.ProviderRefundID == "" || providerRefund.AmountCents != record.AmountCents ||
		(providerRefund.ProviderPaymentID != "" && providerRefund.ProviderPaymentID != record.ProviderPaymentID) ||
		(providerRefund.Currency != "" && providerRefund.Currency != record.Currency) ||
		(providerRefund.Status != "pending" && providerRefund.Status != "processed") {
		_ = s.repository.FailRefund(ctx, record.ID, ErrRefundResponseMismatch.Error())
		return domaincommerce.Refund{}, ErrRefundResponseMismatch
	}
	completed, err := s.repository.CompleteRefund(ctx, record.ID, providerRefund)
	if err != nil {
		return domaincommerce.Refund{}, err
	}
	return mapRefund(completed), nil
}

// Reconcile only reads provider state. Missing or uncertain responses never
// authorize another refund POST.
func (s *RefundService) Reconcile(ctx context.Context, limit int) (int, error) {
	repository, ok := s.repository.(ports.RefundRecoveryRepository)
	gateway, supported := s.gateway.(ports.RefundRecoveryGateway)
	if !ok || !supported {
		return 0, nil
	}
	if limit < 1 || limit > 100 {
		return 0, ErrRefundInvalidRequest
	}
	records, err := repository.ClaimRefundReconciliation(ctx, limit)
	if err != nil {
		return 0, err
	}
	completed := 0
	var failures error
	for _, record := range records {
		refund, err := gateway.LookupRefund(ctx, record)
		if err == nil && (refund.ProviderRefundID == "" || refund.ProviderPaymentID != record.ProviderPaymentID || refund.AmountCents != record.AmountCents || refund.Currency != record.Currency) {
			err = ErrRefundResponseMismatch
		}
		if err == nil {
			_, err = s.repository.CompleteRefund(ctx, record.ID, refund)
		}
		if err != nil {
			failures = errors.Join(failures, err)
			continue
		}
		completed++
	}
	return completed, failures
}

func mapRefund(record ports.PaymentRefundRecord) domaincommerce.Refund {
	return domaincommerce.Refund{
		ID:          record.ID,
		OrderID:     record.OrderID,
		OrderNumber: record.OrderNumber,
		Status:      record.Status,
		AmountCents: record.AmountCents,
		Currency:    record.Currency,
		Reason:      record.Reason,
	}
}
