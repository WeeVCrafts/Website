package metrics

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Groups are a fixed vocabulary. Never retain a URL, method supplied by a
// client, account identifier, query, header or SQL statement as a label.
var httpGroups = [...]string{"other", "catalog", "search", "cart", "checkout", "account", "seller", "admin", "assets", "media"}
var latencyLimits = [...]time.Duration{100 * time.Millisecond, 250 * time.Millisecond, 500 * time.Millisecond, time.Second, 2500 * time.Millisecond, 5 * time.Second}
var latencyLabels = [...]string{"100", "250", "500", "1000", "2500", "5000", "+Inf"}

type httpCounters struct {
	requests, errors, durationNanos atomic.Int64
	buckets                         [7]atomic.Int64
}
type HTTPMetrics struct {
	counters                                            httpCounters
	groups                                              [10]httpCounters
	readRequests, writeRequests, inFlight, peakInFlight atomic.Int64
}
type LatencyBucket struct {
	UpperMillis string `json:"upper_millis"`
	Count       int64  `json:"count"`
}
type HTTPGroupSnapshot struct {
	Group          string          `json:"group"`
	Requests       int64           `json:"requests"`
	Errors         int64           `json:"errors"`
	DurationMillis int64           `json:"duration_millis"`
	LatencyBuckets []LatencyBucket `json:"latency_buckets"`
}
type HTTPSnapshot struct {
	Requests       int64               `json:"requests"`
	ReadRequests   int64               `json:"read_requests"`
	WriteRequests  int64               `json:"write_requests"`
	Errors         int64               `json:"errors"`
	InFlight       int64               `json:"in_flight"`
	PeakInFlight   int64               `json:"peak_in_flight"`
	DurationMillis int64               `json:"duration_millis"`
	LatencyBuckets []LatencyBucket     `json:"latency_buckets"`
	Groups         []HTTPGroupSnapshot `json:"groups"`
}

func NewHTTPMetrics() *HTTPMetrics { return &HTTPMetrics{} }
func buckets(c *httpCounters) []LatencyBucket {
	v := make([]LatencyBucket, len(latencyLabels))
	for i, label := range latencyLabels {
		v[i] = LatencyBucket{label, c.buckets[i].Load()}
	}
	return v
}

// Snapshot is cumulative since this process started, not a persistent visitor or
// billing counter. Concurrent observations can straddle individual atomic reads.
func (m *HTTPMetrics) Snapshot() HTTPSnapshot {
	s := HTTPSnapshot{Requests: m.counters.requests.Load(), ReadRequests: m.readRequests.Load(), WriteRequests: m.writeRequests.Load(), Errors: m.counters.errors.Load(), InFlight: m.inFlight.Load(), PeakInFlight: m.peakInFlight.Load(), DurationMillis: m.counters.durationNanos.Load() / int64(time.Millisecond), LatencyBuckets: buckets(&m.counters), Groups: make([]HTTPGroupSnapshot, len(httpGroups))}
	for i, g := range httpGroups {
		c := &m.groups[i]
		s.Groups[i] = HTTPGroupSnapshot{g, c.requests.Load(), c.errors.Load(), c.durationNanos.Load() / int64(time.Millisecond), buckets(c)}
	}
	return s
}
func (m *HTTPMetrics) observe(group int, method string, status int, elapsed time.Duration) {
	switch method {
	case "GET", "HEAD":
		m.readRequests.Add(1)
	case "POST", "PUT", "PATCH", "DELETE":
		m.writeRequests.Add(1)
	}
	for _, c := range []*httpCounters{&m.counters, &m.groups[group]} {
		c.requests.Add(1)
		c.durationNanos.Add(int64(elapsed))
		if status >= 500 {
			c.errors.Add(1)
		}
		for i := range latencyLabels {
			if i == len(latencyLimits) || elapsed <= latencyLimits[i] {
				c.buckets[i].Add(1)
			}
		}
	}
}
func routeGroup(path string) int {
	match := func(prefix string) bool { return path == prefix || strings.HasPrefix(path, prefix+"/") }
	switch {
	case match("/products") || match("/api/v1/products") || match("/categories") || match("/makers"):
		return 1
	case match("/api/v1/search") || match("/search"):
		return 2
	case match("/cart") || match("/api/v1/cart"):
		return 3
	case match("/checkout") || match("/api/v1/checkout"):
		return 4
	case match("/account") || match("/api/v1/auth") || match("/login") || match("/register") || match("/reset-password") || match("/forgot-password"):
		return 5
	case match("/seller") || match("/seller-admin"):
		return 6
	case match("/admin") || match("/super-admin") || match("/support-portal"):
		return 7
	case match("/assets"):
		return 8
	case match("/media"):
		return 9
	default:
		return 0
	}
}
func (m *HTTPMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/health/") {
			next.ServeHTTP(w, r)
			return
		}
		active := m.inFlight.Add(1)
		for old := m.peakInFlight.Load(); active > old; old = m.peakInFlight.Load() {
			if m.peakInFlight.CompareAndSwap(old, active) {
				break
			}
		}
		started := time.Now()
		capture := &statusWriter{ResponseWriter: w}
		completed := false
		defer func() {
			status := capture.status
			if status == 0 {
				status = 200
			}
			if !completed && status < 500 {
				status = 500
			}
			m.observe(routeGroup(r.URL.Path), r.Method, status, time.Since(started))
			m.inFlight.Add(-1)
		}()
		next.ServeHTTP(capture, r)
		completed = true
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *statusWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}
func (w *statusWriter) Push(target string, options *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, options)
	}
	return http.ErrNotSupported
}
func (w *statusWriter) ReadFrom(r io.Reader) (int64, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}
	return io.Copy(struct{ io.Writer }{w.ResponseWriter}, r)
}
