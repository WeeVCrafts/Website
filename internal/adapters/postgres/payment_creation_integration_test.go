package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/wecratfs/commerce/internal/adapters/metrics"
	"github.com/wecratfs/commerce/internal/adapters/razorpay"
	applicationpayment "github.com/wecratfs/commerce/internal/application/payment"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestPaymentCreationClaimSurvivesConcurrentRequestsAndCancellation(t *testing.T) {
	pool := openIntegrationPool(t)
	fixture := newCommerceFixture(t, pool, 1, 1)
	ctx := t.Context()
	commerce := NewCommerceRepository(pool)
	repository := NewPaymentRepository(pool)
	userID := fixture.userIDs[0]
	order, err := commerce.CreateOrder(ctx, userID, fixture.cartIDs[0], fmt.Sprintf("creation-%d", userID), domaincommerce.AddressInput{RecipientName: "Fixture customer", Phone: "9876543210", Line1: "1 Craft Lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
	if err != nil {
		t.Fatal(err)
	}
	if claim, err := repository.ClaimExistingPaymentCreation(ctx, userID, order.OrderNumber); err != nil || claim.AttemptID != 0 {
		t.Fatalf("read-only refresh started an operation: %+v %v", claim, err)
	}
	if _, err := repository.ClaimPaymentCreation(ctx, fixture.ownerID, order.OrderNumber); !errors.Is(err, ports.ErrPaymentNotFound) {
		t.Fatalf("another account claimed payment: %v", err)
	}
	start := make(chan struct{})
	claims := make(chan ports.PaymentCreationClaim, 20)
	failures := make(chan error, 20)
	var workers sync.WaitGroup
	for range 20 {
		workers.Go(func() {
			<-start
			claim, err := repository.ClaimPaymentCreation(ctx, userID, order.OrderNumber)
			if err == nil {
				claims <- claim
			} else {
				failures <- err
			}
		})
	}
	close(start)
	workers.Wait()
	close(claims)
	close(failures)
	if len(claims) != 1 || len(failures) != 19 {
		t.Fatalf("claim winners=%d pending requests=%d", len(claims), len(failures))
	}
	claimed := <-claims
	if !claimed.Submit || claimed.AttemptID <= 0 {
		t.Fatalf("initial claim=%+v", claimed)
	}
	for err := range failures {
		if !errors.Is(err, ports.ErrPaymentCreationPending) {
			t.Errorf("concurrent request error=%v", err)
		}
	}
	detail, err := commerce.GetOrder(ctx, userID, order.OrderNumber)
	if err != nil || detail.CanPay || !detail.PaymentCreationPending || detail.PaymentReview {
		t.Fatalf("creation projection=%+v error=%v", detail, err)
	}
	if _, err := repository.AttachProviderOrder(ctx, userID, order.OrderNumber, ports.PaymentOrder{ProviderOrderID: "bypass", AmountCents: order.TotalCents, Currency: order.Currency}); !errors.Is(err, ports.ErrPaymentState) {
		t.Fatalf("legacy writer bypassed claim: %v", err)
	}
	if err := commerce.CancelOrder(ctx, userID, order.OrderNumber); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE payment_attempts SET creation_next_lookup_at=NOW()-INTERVAL '1 minute' WHERE id=$1`, claimed.AttemptID); err != nil {
		t.Fatal(err)
	}
	// A new repository represents recovery by another application instance.
	restarted := NewPaymentRepository(pool)
	recovered, err := restarted.ClaimExistingPaymentCreation(ctx, userID, order.OrderNumber)
	if err != nil || recovered.Submit || recovered.AttemptID != claimed.AttemptID {
		t.Fatalf("recovery claim=%+v error=%v", recovered, err)
	}
	providerID := fmt.Sprintf("order_creation_%d", order.ID)
	if _, err := restarted.AttachClaimedProviderOrder(ctx, recovered, ports.PaymentOrder{ProviderOrderID: providerID, AmountCents: order.TotalCents, Currency: order.Currency}); err != nil {
		t.Fatal(err)
	}
	event := ports.PaymentWebhook{EventID: providerID + "_captured", EventType: "payment.captured", ProviderOrderID: providerID, ProviderPaymentID: fmt.Sprintf("pay_creation_%d", order.ID), AmountCents: order.TotalCents, Currency: order.Currency}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM payment_review_cases WHERE order_id=$1`, order.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM payment_webhook_events WHERE event_id=$1`, event.EventID)
	})
	if _, err := restarted.ProcessWebhook(ctx, event, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	detail, err = commerce.GetOrder(ctx, userID, order.OrderNumber)
	if err != nil || !detail.PaymentReview || detail.CanPay || detail.PaymentStatus != "captured" || detail.Status != "cancelled" {
		t.Fatalf("late recovered capture=%+v error=%v", detail, err)
	}
}

func TestPaymentCreationMismatchIsVisibleOnlyToReviewers(t *testing.T) {
	pool := openIntegrationPool(t)
	fixture := newCommerceFixture(t, pool, 1, 1)
	ctx := t.Context()
	commerce := NewCommerceRepository(pool)
	repository := NewPaymentRepository(pool)
	userID := fixture.userIDs[0]
	order, err := commerce.CreateOrder(ctx, userID, fixture.cartIDs[0], fmt.Sprintf("creation-review-%d", userID), domaincommerce.AddressInput{RecipientName: "Fixture customer", Phone: "9876543210", Line1: "1 Craft Lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
	if err != nil {
		t.Fatal(err)
	}
	claim, err := repository.ClaimPaymentCreation(ctx, userID, order.OrderNumber)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.ReviewPaymentCreation(ctx, claim, "receipt matched multiple provider orders"); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ClaimPaymentCreation(ctx, userID, order.OrderNumber); !errors.Is(err, ports.ErrPaymentCreationReview) {
		t.Fatalf("review allowed a retry: %v", err)
	}
	detail, err := commerce.GetOrder(ctx, userID, order.OrderNumber)
	if err != nil || !detail.PaymentCreationReview || detail.PaymentReview || detail.CanPay {
		t.Fatalf("review projection=%+v error=%v", detail, err)
	}
	if _, err := repository.ListPaymentReviews(ctx, userID, 100); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("customer accessed review queue: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'admin')`, fixture.ownerID); err != nil {
		t.Fatal(err)
	}
	cases, err := repository.ListPaymentReviews(ctx, fixture.ownerID, 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range cases {
		if item.OrderID == order.ID {
			found = item.Status == "creation_review" && !item.CanApprove && item.ProviderPaymentID == ""
		}
	}
	if !found {
		t.Fatalf("creation exception absent or offers capture action: %+v", cases)
	}
	if _, _, review, err := repository.PaymentReconciliationBacklog(ctx); err != nil || review < 1 {
		t.Fatalf("review backlog=%d error=%v", review, err)
	}
}

func TestPaymentCreationRecoversProviderResponseLossWithSavedReceipt(t *testing.T) {
	pool := openIntegrationPool(t)
	fixture := newCommerceFixture(t, pool, 1, 1)
	ctx := t.Context()
	commerce := NewCommerceRepository(pool)
	userID := fixture.userIDs[0]
	order, err := commerce.CreateOrder(ctx, userID, fixture.cartIDs[0], fmt.Sprintf("creation-lost-%d", userID), domaincommerce.AddressInput{RecipientName: "Fixture customer", Phone: "9876543210", Line1: "1 Craft Lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
	if err != nil {
		t.Fatal(err)
	}
	providerID := fmt.Sprintf("order_lost_%d", order.ID)
	var state sync.Mutex
	posts := 0
	visible := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.Lock()
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/orders":
			posts++
			state.Unlock()
			// The provider commits the order but the response cannot be decoded.
			fmt.Fprint(w, `{"id":`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/orders":
			canRead := visible
			state.Unlock()
			if r.URL.Query().Get("receipt") != order.OrderNumber {
				t.Error("lookup changed the receipt")
			}
			if canRead {
				fmt.Fprintf(w, `{"entity":"collection","items":[{"id":%q,"receipt":%q,"amount":%d,"currency":%q}]}`, providerID, order.OrderNumber, order.TotalCents, order.Currency)
			} else {
				<-r.Context().Done()
			}
		case r.Method == http.MethodGet && r.URL.Path == "/v1/orders/"+providerID+"/payments":
			state.Unlock()
			fmt.Fprint(w, `{"items":[]}`)
		default:
			state.Unlock()
			http.Error(w, "Unexpected provider request", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	gateway := razorpay.NewGateway("rzp_test_fixture", "fixture-only", "", server.URL)
	observations := &metrics.PaymentMetrics{}
	if _, err := applicationpayment.NewOrderService(gateway, NewPaymentRepository(pool)).WithCreationObserver(observations.Observe).Create(ctx, userID, order.OrderNumber); !errors.Is(err, ports.ErrPaymentCreationPending) {
		t.Fatalf("lost response did not remain pending: %v", err)
	}
	if snapshot := observations.Snapshot(); snapshot.Lookup.Timeout != 1 || snapshot.Creation.Error != 1 {
		t.Fatalf("lookup timeout and creation error were not distinguished: %+v", snapshot)
	}
	state.Lock()
	visible = true
	state.Unlock()
	if _, err := pool.Exec(ctx, `UPDATE payment_attempts SET creation_next_lookup_at=NOW()-INTERVAL '1 minute',
		creation_started_at=NOW()-INTERVAL '2 minutes' WHERE order_id=$1`, order.ID); err != nil {
		t.Fatal(err)
	}
	due, age, _, err := NewPaymentRepository(pool).PaymentReconciliationBacklog(ctx)
	if err != nil || due < 1 || age < 120 {
		t.Fatalf("pending order missing from reconciliation backlog: due=%d age=%d error=%v", due, age, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE inventory_reservations SET expires_at=NOW()-INTERVAL '1 minute' WHERE order_id=$1`, order.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := commerce.ReleaseExpiredReservations(ctx, 100); err != nil {
		t.Fatal(err)
	}
	if err := applicationpayment.NewOrderService(gateway, NewPaymentRepository(pool)).Refresh(ctx, userID, order.OrderNumber); err != nil {
		t.Fatal(err)
	}
	saved, err := NewPaymentRepository(pool).GetPaymentOrder(ctx, userID, order.OrderNumber)
	if err != nil || saved.ProviderOrderID != providerID || saved.PaymentStatus != "pending" {
		t.Fatalf("recovered payment=%+v error=%v", saved, err)
	}
	if payable, err := NewPaymentRepository(pool).PaymentOrderPayable(ctx, userID, order.OrderNumber); err != nil || payable {
		t.Fatalf("recovery reopened an expired payment: payable=%v error=%v", payable, err)
	}
	state.Lock()
	defer state.Unlock()
	if posts != 1 {
		t.Fatalf("response loss repeated provider creation %d times", posts)
	}
}
