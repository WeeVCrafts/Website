package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLiveIsIndependentOfDependencies(t *testing.T) {
	handler := NewHandler(map[string]Probe{
		"database": func(context.Context) error { return errors.New("database is down") },
	})
	recorder := httptest.NewRecorder()
	handler.Live(recorder, httptest.NewRequest(http.MethodGet, "/health/live", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected live status 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"status":"live"`) {
		t.Fatalf("unexpected live body: %s", recorder.Body.String())
	}
}

func TestPublicReadyHidesDependencyDetails(t *testing.T) {
	for _, failed := range []bool{false, true} {
		h := NewHandler(map[string]Probe{"private-database": func(context.Context) error {
			if failed {
				return errors.New("private host unavailable")
			}
			return nil
		}})
		w := httptest.NewRecorder()
		h.PublicReady(w, httptest.NewRequest("GET", "/health/ready", nil))
		want := 200
		if failed {
			want = 503
		}
		if w.Code != want || strings.Contains(w.Body.String(), "private") || strings.Contains(w.Body.String(), "checks") {
			t.Fatalf("unexpected public health: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestReadyReportsFailedProbe(t *testing.T) {
	handler := NewHandler(map[string]Probe{
		"database": func(context.Context) error { return errors.New("database is down") },
		"cache":    func(context.Context) error { return nil },
	})
	recorder := httptest.NewRecorder()
	handler.Ready(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected not-ready status 503, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"status":"not_ready"`) || !strings.Contains(body, `"database":"not_ready"`) {
		t.Fatalf("unexpected readiness body: %s", body)
	}
}
