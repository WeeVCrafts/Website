package metrics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPMetricsKeepBoundedGroupsAndExcludePrivateData(t *testing.T) {
	m := NewHTTPMetrics()
	h := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(409)
		} else {
			w.WriteHeader(200)
		}
	}))
	for i := 0; i < 250; i++ {
		r := httptest.NewRequest("POST", "/api/v1/checkout/intents?token=DO_NOT_RECORD_TOKEN", nil)
		r.Header.Set("Authorization", "Bearer DO_NOT_RECORD_AUTH")
		h.ServeHTTP(httptest.NewRecorder(), r)
	}
	for _, p := range []string{"/account/customer-SENSITIVE-ID", "/unknown/SENSITIVE-ID", "/health/metrics?secret=SENSITIVE-ID"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", p, nil))
	}
	s := m.Snapshot()
	if s.Requests != 252 || s.WriteRequests != 250 || s.ReadRequests != 2 || s.Errors != 0 || s.InFlight != 0 {
		t.Fatalf("unexpected counters: %+v", s)
	}
	if len(s.Groups) != 10 {
		t.Fatal("unbounded groups")
	}
	b, _ := json.Marshal(s)
	for _, private := range []string{"DO_NOT_RECORD", "SENSITIVE-ID", "Authorization", "token="} {
		if strings.Contains(string(b), private) {
			t.Fatal("metrics retain private request data")
		}
	}
}

func TestHTTPMetricsReleaseInflightAndRecordPanics(t *testing.T) {
	m := NewHTTPMetrics()
	h := m.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("synthetic failure") }))
	func() {
		defer func() {
			if recover() == nil {
				t.Error("middleware swallowed a panic")
			}
		}()
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/products", nil))
	}()
	s := m.Snapshot()
	if s.InFlight != 0 || s.Requests != 1 || s.Errors != 1 {
		t.Fatalf("panic not accounted for: %+v", s)
	}
}

func TestHTTPMetricsObserveFailuresBucketsAndConcurrentRequests(t *testing.T) {
	m := NewHTTPMetrics()
	m.observe(0, "GET", 503, 125*time.Millisecond)
	s := m.Snapshot()
	if s.Errors != 1 || s.Requests != 1 || s.LatencyBuckets[0].Count != 0 || s.LatencyBuckets[1].Count != 1 || s.LatencyBuckets[len(s.LatencyBuckets)-1].Count != 1 {
		t.Fatalf("invalid histogram or errors: %+v", s)
	}
	entered, release, done := make(chan struct{}, 2), make(chan struct{}), make(chan struct{}, 2)
	h := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { entered <- struct{}{}; <-release; w.WriteHeader(204) }))
	for range 2 {
		go func() {
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/products", nil))
			done <- struct{}{}
		}()
	}
	<-entered
	<-entered
	if v := m.Snapshot(); v.InFlight != 2 || v.PeakInFlight != 2 {
		t.Fatalf("in-flight not observed: %+v", v)
	}
	close(release)
	<-done
	<-done
	if v := m.Snapshot(); v.InFlight != 0 || v.Requests != 3 {
		t.Fatalf("in-flight not released: %+v", v)
	}
}

func TestHTTPMetricsWriterPreservesFlushAndFirstStatus(t *testing.T) {
	m := NewHTTPMetrics()
	h := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
		w.WriteHeader(500)
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("ok"))
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/products", nil))
	if w.Code != 202 || w.Body.String() != "ok" || !w.Flushed || m.Snapshot().Errors != 0 {
		t.Fatal("middleware changed HTTP semantics")
	}
}

func TestOperationalSnapshotPreservesSearchFields(t *testing.T) {
	s := OperationalSnapshot{SearchSnapshot: SearchSnapshot{Observations: 9, CacheHits: 4}, HTTP: NewHTTPMetrics().Snapshot()}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"observations", "cache_hits", "postgres_queries", "http", "database_pool"} {
		if _, ok := data[key]; !ok {
			t.Errorf("missing compatibility field %s", key)
		}
	}
}
