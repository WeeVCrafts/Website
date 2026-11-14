package metrics

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestPaymentMetricsKeepFixedOutcomesAndAtomicBacklog(t *testing.T) {
	m := &PaymentMetrics{}
	var workers sync.WaitGroup
	for range 10 {
		workers.Go(func() {
			for range 100 {
				m.Observe("lookup", "timeout")
			}
		})
	}
	workers.Wait()
	m.Observe("creation", "success")
	m.Observe("order_private_123", "error")
	m.Observe("lookup", "private_customer_456")
	m.SetBacklog(PaymentBacklogSnapshot{Available: true, Due: 3, OldestPendingSeconds: 120})
	s := m.Snapshot()
	if s.Lookup.Timeout != 1000 || s.Creation.Success != 1 || s.Backlog.Due != 3 || !s.Backlog.Available {
		t.Fatalf("snapshot=%+v", s)
	}
	data, err := json.Marshal(s)
	if err != nil || strings.Contains(string(data), "private") {
		t.Fatalf("json=%s error=%v", data, err)
	}
}
