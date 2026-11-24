package handlers

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDailyReconciliationShowsOpenAndCreationReviews(t *testing.T) {
	h := financeTestHandler(t)
	ctx := context.Background()
	_, err := h.pool.Exec(ctx, `
		INSERT INTO order_status_history(order_id,from_status,to_status)
		VALUES(900104,'pending_payment','paid');
		UPDATE payment_attempts SET creation_review_reason='Provider lookup needs review' WHERE order_id=900104;
		INSERT INTO payment_review_cases(order_id,payment_attempt_id,provider_payment_id,amount_cents,currency,reason,status)
		SELECT 900104,id,'finance-open-review',100,'INR','Open exception','open'
		FROM payment_attempts WHERE order_id=900104;
		INSERT INTO payment_review_cases(order_id,payment_attempt_id,provider_payment_id,amount_cents,currency,reason,status)
		SELECT 900104,id,'finance-approved-review',100,'INR','Resolved exception','approved'
		FROM payment_attempts WHERE order_id=900104;
	`)
	if err != nil {
		t.Fatal(err)
	}
	date := time.Now().In(reconciliationIST).Format(time.DateOnly)
	report, err := h.dailyReconciliation(httptest.NewRequest("GET", "/admin/finance-reconciliation?date="+date, nil), "/admin")
	if err != nil {
		t.Fatal(err)
	}
	if report.Totals[0].Value != "INR 10" && report.Totals[0].Value != "INR 10.00" {
		t.Fatalf("captured amount = %s", report.Totals[0].Value)
	}
	if len(report.FollowUps) != 2 {
		t.Fatalf("got %d follow-ups, want one open review and one creation review", len(report.FollowUps))
	}
	for _, item := range report.FollowUps {
		if item.Status == "approved" || item.URL != "/admin/payment-reviews" {
			t.Fatalf("incorrect follow-up entry: %+v", item)
		}
	}
}
