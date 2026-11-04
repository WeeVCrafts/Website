package payment

import (
	"context"
	"errors"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

type reviewRepository struct {
	record             ports.PaymentRefundRecord
	claimed, uncertain bool
	approved           int
	denied             bool
}

func (r *reviewRepository) AuthorizeReviewRefundRetry(context.Context, int64, int64, string, string) error {
	return ports.ErrRefundState
}

func (r *reviewRepository) ListPaymentReviews(context.Context, int64, int) ([]ports.PaymentReviewCase, error) {
	return nil, nil
}
func (r *reviewRepository) ApprovePaymentReview(context.Context, int64, int64, string) error {
	r.approved++
	return nil
}
func (r *reviewRepository) BeginReviewRefund(context.Context, int64, int64, string) (ports.PaymentRefundRecord, error) {
	if r.denied {
		return ports.PaymentRefundRecord{}, ports.ErrForbidden
	}
	return r.record, nil
}
func (r *reviewRepository) ClaimReviewRefundSubmission(context.Context, int64) (bool, error) {
	if r.claimed {
		return false, nil
	}
	r.claimed = true
	return true, nil
}
func (r *reviewRepository) CompleteReviewRefund(context.Context, int64, ports.PaymentRefund) error {
	return nil
}
func (r *reviewRepository) MarkReviewRefundUncertain(context.Context, int64) error {
	r.uncertain = true
	r.record.SubmissionState = "uncertain"
	return nil
}
func (r *reviewRepository) ClaimReviewRefundReconciliation(context.Context, int) ([]ports.PaymentRefundRecord, error) {
	return nil, nil
}

func TestReviewRefundRequiresSeparateAuthorizationAndNeverRepeatsUncertainPost(t *testing.T) {
	repo := &reviewRepository{record: ports.PaymentRefundRecord{ID: 7, ProviderPaymentID: "pay_review", AmountCents: 1000, Currency: "INR", Status: "pending", SubmissionState: "queued", Receipt: "payment-review-7"}}
	gateway := &fakeRefundGateway{err: context.DeadlineExceeded}
	service := NewReviewService(gateway, repo)
	if err := service.Refund(context.Background(), 0, 7, "approved refund"); !errors.Is(err, ports.ErrForbidden) || gateway.calls != 0 {
		t.Fatal("missing actor accepted")
	}
	repo.denied = true
	if err := service.Refund(context.Background(), 10, 7, "approved refund"); !errors.Is(err, ports.ErrForbidden) || gateway.calls != 0 {
		t.Fatal("repository authorization bypass")
	}
	repo.denied = false
	if err := service.Refund(context.Background(), 10, 7, "approved refund"); !errors.Is(err, ErrRefundCreationFailed) || !repo.uncertain {
		t.Fatalf("timeout was not retained: %v", err)
	}
	if err := service.Refund(context.Background(), 10, 7, "explicit retry"); err != nil || gateway.calls != 1 {
		t.Fatalf("uncertain refund repeated: calls=%d err=%v", gateway.calls, err)
	}
	if gateway.request.Receipt != "payment-review-7" || gateway.request.AmountCents != 1000 {
		t.Fatal("refund lost stable identity or authoritative amount")
	}
}
