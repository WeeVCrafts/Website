package payment

import (
	"context"
	"errors"
	"strings"

	"github.com/wecratfs/commerce/internal/ports"
)

// Finance exceptions have their own explicit authorization boundary. Normal
// customer refunds keep their existing received-return requirement.
type ReviewService struct {
	gateway    ports.PaymentRefundGateway
	repository ports.PaymentReviewRepository
}

func NewReviewService(gateway ports.PaymentRefundGateway, repository ports.PaymentReviewRepository) *ReviewService {
	return &ReviewService{gateway: gateway, repository: repository}
}
func reviewReason(actor, id int64, reason string) error {
	if actor < 1 || id < 1 {
		return ports.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if len(reason) < 3 || len(reason) > 500 {
		return ErrRefundInvalidRequest
	}
	return nil
}
func (s *ReviewService) List(ctx context.Context, actor int64) ([]ports.PaymentReviewCase, error) {
	if actor < 1 || s.repository == nil {
		return nil, ports.ErrForbidden
	}
	return s.repository.ListPaymentReviews(ctx, actor, 100)
}
func (s *ReviewService) Approve(ctx context.Context, actor, id int64, reason string) error {
	if err := reviewReason(actor, id, reason); err != nil {
		return err
	}
	if s.repository == nil {
		return ports.ErrForbidden
	}
	return s.repository.ApprovePaymentReview(ctx, actor, id, strings.TrimSpace(reason))
}
func (s *ReviewService) Refund(ctx context.Context, actor, id int64, reason string) error {
	if err := reviewReason(actor, id, reason); err != nil {
		return err
	}
	if s.repository == nil || s.gateway == nil {
		return ports.ErrRefundState
	}
	record, err := s.repository.BeginReviewRefund(ctx, actor, id, strings.TrimSpace(reason))
	if err != nil {
		return err
	}
	if record.Status == "processed" || record.SubmissionState != "queued" {
		return nil
	}
	claimed, err := s.repository.ClaimReviewRefundSubmission(ctx, id)
	if err != nil || !claimed {
		return err
	}
	refund, err := s.gateway.CreateRefund(ctx, ports.PaymentRefundRequest{ProviderPaymentID: record.ProviderPaymentID, AmountCents: record.AmountCents, Currency: record.Currency, Receipt: record.Receipt})
	if err != nil {
		_ = s.repository.MarkReviewRefundUncertain(ctx, id)
		return ErrRefundCreationFailed
	}
	if !reviewRefundMatches(record, refund) {
		_ = s.repository.MarkReviewRefundUncertain(ctx, id)
		return ErrRefundResponseMismatch
	}
	return s.repository.CompleteReviewRefund(ctx, id, refund)
}

func (s *ReviewService) RetryRefund(ctx context.Context, actor, id int64, reason, expectedRefundID string) error {
	if err := reviewReason(actor, id, reason); err != nil {
		return err
	}
	if s.repository == nil || s.gateway == nil || expectedRefundID == "" || len(expectedRefundID) > 128 {
		return ports.ErrRefundState
	}
	if err := s.repository.AuthorizeReviewRefundRetry(ctx, actor, id, strings.TrimSpace(reason), expectedRefundID); err != nil {
		return err
	}
	return s.Refund(ctx, actor, id, reason)
}
func reviewRefundMatches(record ports.PaymentRefundRecord, refund ports.PaymentRefund) bool {
	return refund.ProviderRefundID != "" && refund.ProviderPaymentID == record.ProviderPaymentID && refund.AmountCents == record.AmountCents && refund.Currency == record.Currency && (refund.Status == "pending" || refund.Status == "processed" || refund.Status == "failed")
}
func (s *ReviewService) Reconcile(ctx context.Context, limit int) (int, error) {
	gateway, ok := s.gateway.(ports.RefundRecoveryGateway)
	if !ok || s.repository == nil {
		return 0, nil
	}
	if limit < 1 || limit > 100 {
		return 0, ErrRefundInvalidRequest
	}
	records, err := s.repository.ClaimReviewRefundReconciliation(ctx, limit)
	if err != nil {
		return 0, err
	}
	completed := 0
	var failures error
	for _, record := range records {
		refund, err := gateway.LookupRefund(ctx, record)
		if err == nil && !reviewRefundMatches(record, refund) {
			err = ErrRefundResponseMismatch
		}
		if err == nil {
			err = s.repository.CompleteReviewRefund(ctx, record.ID, refund)
		}
		if err != nil {
			failures = errors.Join(failures, err)
		} else {
			completed++
		}
	}
	return completed, failures
}
