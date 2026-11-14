package payment

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

type creationRepository struct {
	mu                     sync.Mutex
	record                 ports.PaymentOrderRecord
	started, due, reviewed bool
	attachErr              error
}

func newCreationRepository() *creationRepository {
	return &creationRepository{record: ports.PaymentOrderRecord{OrderID: 1, OrderNumber: "WC-CREATE", OrderStatus: "pending_payment", Provider: "razorpay", PaymentStatus: "created", AmountCents: 1200, Currency: "INR"}}
}

func (r *creationRepository) GetPaymentOrder(context.Context, int64, string) (ports.PaymentOrderRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.record, nil
}

func (r *creationRepository) ClaimPaymentCreation(_ context.Context, userID int64, _ string) (ports.PaymentCreationClaim, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reviewed {
		return ports.PaymentCreationClaim{}, ports.ErrPaymentCreationReview
	}
	if r.started && !r.due && r.record.ProviderOrderID == "" {
		return ports.PaymentCreationClaim{}, ports.ErrPaymentCreationPending
	}
	claim := ports.PaymentCreationClaim{AttemptID: 1, UserID: userID, Order: r.record, Submit: !r.started}
	r.started, r.due = true, false
	return claim, nil
}

func (r *creationRepository) ClaimExistingPaymentCreation(ctx context.Context, userID int64, number string) (ports.PaymentCreationClaim, error) {
	return r.ClaimPaymentCreation(ctx, userID, number)
}

func (r *creationRepository) ClaimPaymentCreationRecovery(context.Context, int) ([]ports.PaymentCreationClaim, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.started || !r.due || r.reviewed || r.record.ProviderOrderID != "" {
		return nil, nil
	}
	r.due = false
	return []ports.PaymentCreationClaim{{AttemptID: 1, UserID: 7, Order: r.record}}, nil
}

func (r *creationRepository) AttachProviderOrder(ctx context.Context, userID int64, number string, order ports.PaymentOrder) (ports.PaymentOrderRecord, error) {
	return r.AttachClaimedProviderOrder(ctx, ports.PaymentCreationClaim{}, order)
}

func (r *creationRepository) AttachClaimedProviderOrder(_ context.Context, _ ports.PaymentCreationClaim, order ports.PaymentOrder) (ports.PaymentOrderRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.attachErr != nil {
		return ports.PaymentOrderRecord{}, r.attachErr
	}
	r.record.ProviderOrderID, r.record.PaymentStatus = order.ProviderOrderID, "pending"
	return r.record, nil
}

func (r *creationRepository) ReviewPaymentCreation(context.Context, ports.PaymentCreationClaim, string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reviewed = true
	return nil
}

type creationGateway struct {
	mu                   sync.Mutex
	posts, lookups       int
	createErr, lookupErr error
	matches              []ports.PaymentOrder
	entered, release     chan struct{}
}

func matchingCreationOrder() ports.PaymentOrder {
	return ports.PaymentOrder{ProviderOrderID: "order_saved", AmountCents: 1200, Currency: "INR"}
}

func (g *creationGateway) CreateOrder(context.Context, ports.PaymentOrderRequest) (ports.PaymentOrder, error) {
	g.mu.Lock()
	g.posts++
	g.mu.Unlock()
	if g.entered != nil {
		close(g.entered)
		<-g.release
	}
	return matchingCreationOrder(), g.createErr
}

func (g *creationGateway) VerifyPayment(context.Context, ports.PaymentVerification) error { return nil }

func (g *creationGateway) LookupOrdersByReceipt(context.Context, string) ([]ports.PaymentOrder, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lookups++
	return g.matches, g.lookupErr
}

func TestPaymentCreationConcurrentRequestsShareDurableClaim(t *testing.T) {
	r := newCreationRepository()
	g := &creationGateway{entered: make(chan struct{}), release: make(chan struct{})}
	s := NewOrderService(g, r)
	finished := make(chan error, 1)
	go func() { _, err := s.Create(t.Context(), 7, "WC-CREATE"); finished <- err }()
	<-g.entered
	for range 20 {
		if _, err := s.Create(t.Context(), 7, "WC-CREATE"); !errors.Is(err, ports.ErrPaymentCreationPending) {
			t.Errorf("concurrent create = %v", err)
		}
	}
	close(g.release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	intent, err := s.Create(t.Context(), 7, "WC-CREATE")
	if err != nil || intent.ProviderOrderID != "order_saved" || g.posts != 1 {
		t.Fatalf("intent=%+v error=%v provider posts=%d", intent, err, g.posts)
	}
}

func TestPaymentCreationRecoversLostResponseAndAttachment(t *testing.T) {
	for _, failure := range []string{"response", "attachment"} {
		t.Run(failure, func(t *testing.T) {
			r := newCreationRepository()
			g := &creationGateway{matches: []ports.PaymentOrder{matchingCreationOrder()}}
			if failure == "response" {
				g.createErr = context.DeadlineExceeded
				g.lookupErr = context.DeadlineExceeded
			} else {
				r.attachErr = errors.New("connection closed")
			}
			if _, err := NewOrderService(g, r).Create(t.Context(), 7, "WC-CREATE"); !errors.Is(err, ports.ErrPaymentCreationPending) {
				t.Fatalf("create=%v", err)
			}
			r.attachErr = nil
			r.due = true
			g.lookupErr = nil
			// A new service instance represents a process restart.
			count, err := NewOrderService(g, r).Reconcile(t.Context(), 20)
			if err != nil || count != 1 || r.record.ProviderOrderID != "order_saved" || g.posts != 1 {
				t.Fatalf("recovered=%d err=%v record=%+v posts=%d", count, err, r.record, g.posts)
			}
		})
	}
}

func TestPaymentCreationLookupNeverCreatesOrGuesses(t *testing.T) {
	cases := []struct {
		name      string
		matches   []ports.PaymentOrder
		lookupErr error
		review    bool
	}{
		{name: "none"},
		{name: "provider unavailable", lookupErr: context.DeadlineExceeded},
		{name: "multiple", matches: []ports.PaymentOrder{matchingCreationOrder(), matchingCreationOrder()}, review: true},
		{name: "amount mismatch", matches: []ports.PaymentOrder{{ProviderOrderID: "order_saved", AmountCents: 1300, Currency: "INR"}}, review: true},
		{name: "currency mismatch", matches: []ports.PaymentOrder{{ProviderOrderID: "order_saved", AmountCents: 1200, Currency: "USD"}}, review: true},
		{name: "receipt mismatch", lookupErr: ports.ErrPaymentMismatch, review: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newCreationRepository()
			r.started = true
			r.due = true
			g := &creationGateway{matches: tc.matches, lookupErr: tc.lookupErr}
			_, err := NewOrderService(g, r).Create(t.Context(), 7, "WC-CREATE")
			want := ports.ErrPaymentCreationPending
			if tc.review {
				want = ports.ErrPaymentCreationReview
			}
			if !errors.Is(err, want) || r.reviewed != tc.review || g.posts != 0 || r.record.ProviderOrderID != "" {
				t.Fatalf("error=%v reviewed=%v posts=%d record=%+v", err, r.reviewed, g.posts, r.record)
			}
		})
	}
}
