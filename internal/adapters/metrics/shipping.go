package metrics

import (
	"context"
	"sync/atomic"
	"time"
)

type ShippingSnapshot struct {
	Enabled              bool   `json:"enabled"`
	BacklogAvailable     bool   `json:"backlog_available"`
	PendingOperations    int64  `json:"pending_operations"`
	OldestPendingSeconds int64  `json:"oldest_pending_seconds"`
	OpenExceptions       int64  `json:"open_exceptions"`
	CourierAvailable     uint64 `json:"courier_available"`
	CourierUnavailable   uint64 `json:"courier_unavailable"`
	CourierErrors        uint64 `json:"courier_errors"`
	ReconciliationErrors uint64 `json:"reconciliation_errors"`
}

type Shipping struct {
	courierAvailable, courierUnavailable, courierErrors, reconciliationErrors atomic.Uint64
	backlog                                                                   atomic.Pointer[shippingBacklog]
}

type shippingBacklog struct {
	available           bool
	pending, exceptions int64
	oldest              time.Time
}

func (s *Shipping) CourierResult(available bool, err error) {
	switch {
	case err != nil:
		s.courierErrors.Add(1)
	case available:
		s.courierAvailable.Add(1)
	default:
		s.courierUnavailable.Add(1)
	}
}

type ShippingBacklogReader interface {
	ShippingBacklog(context.Context, string) (int64, time.Time, int64, error)
}

func (s *Shipping) RefreshBacklog(ctx context.Context, repo ShippingBacklogReader, environment string, reconcileErr error) error {
	if reconcileErr != nil {
		s.reconciliationErrors.Add(1)
	}
	pending, oldest, exceptions, err := repo.ShippingBacklog(ctx, environment)
	s.backlog.Store(&shippingBacklog{err == nil, pending, exceptions, oldest})
	return err
}

func (s *Shipping) Snapshot(enabled bool) ShippingSnapshot {
	v := ShippingSnapshot{Enabled: enabled, CourierAvailable: s.courierAvailable.Load(), CourierUnavailable: s.courierUnavailable.Load(), CourierErrors: s.courierErrors.Load(), ReconciliationErrors: s.reconciliationErrors.Load()}
	if b := s.backlog.Load(); b != nil && b.available {
		v.BacklogAvailable = true
		v.PendingOperations, v.OpenExceptions = b.pending, b.exceptions
		if !b.oldest.IsZero() {
			v.OldestPendingSeconds = max(0, int64(time.Since(b.oldest)/time.Second))
		}
	}
	return v
}
