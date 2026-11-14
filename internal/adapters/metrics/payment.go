package metrics

import "sync/atomic"

type PaymentMetrics struct {
	creation, lookup paymentOutcomes
	backlog          atomic.Pointer[PaymentBacklogSnapshot]
}

type paymentOutcomes struct {
	success, timeout, mismatch, missing, failure atomic.Uint64
}

type PaymentOutcomeSnapshot struct {
	Success  uint64 `json:"success"`
	Timeout  uint64 `json:"timeout"`
	Mismatch uint64 `json:"mismatch"`
	Missing  uint64 `json:"missing"`
	Error    uint64 `json:"error"`
}

type PaymentBacklogSnapshot struct {
	Available            bool  `json:"available"`
	Due                  int64 `json:"due"`
	OldestPendingSeconds int64 `json:"oldest_pending_seconds"`
	CreationReview       int64 `json:"creation_review"`
}

type PaymentSnapshot struct {
	Creation PaymentOutcomeSnapshot `json:"provider_order_creation"`
	Lookup   PaymentOutcomeSnapshot `json:"provider_lookup"`
	Backlog  PaymentBacklogSnapshot `json:"reconciliation_backlog"`
}

func (m *PaymentMetrics) Observe(operation, outcome string) {
	var counters *paymentOutcomes
	switch operation {
	case "creation":
		counters = &m.creation
	case "lookup":
		counters = &m.lookup
	default:
		return
	}
	switch outcome {
	case "success":
		counters.success.Add(1)
	case "timeout":
		counters.timeout.Add(1)
	case "mismatch":
		counters.mismatch.Add(1)
	case "missing":
		counters.missing.Add(1)
	case "error":
		counters.failure.Add(1)
	}
}

func (m *PaymentMetrics) SetBacklog(snapshot PaymentBacklogSnapshot) {
	m.backlog.Store(&snapshot)
}

func (m *PaymentMetrics) Snapshot() PaymentSnapshot {
	snapshot := PaymentSnapshot{Creation: m.creation.snapshot(), Lookup: m.lookup.snapshot()}
	if backlog := m.backlog.Load(); backlog != nil {
		snapshot.Backlog = *backlog
	}
	return snapshot
}

func (c *paymentOutcomes) snapshot() PaymentOutcomeSnapshot {
	return PaymentOutcomeSnapshot{Success: c.success.Load(), Timeout: c.timeout.Load(), Mismatch: c.mismatch.Load(), Missing: c.missing.Load(), Error: c.failure.Load()}
}
