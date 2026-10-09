package handlers

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
)

func TestPortalRuntimeReportsConfiguredProbeResults(t *testing.T) {
	h := NewLivePortalHandler(nil, nil, nil)
	called := 0
	h.SetRuntimeProbes("local", map[string]func(context.Context) error{
		"database": func(ctx context.Context) error {
			called++
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("probe has no deadline")
			}
			return nil
		},
		"search": func(context.Context) error { called++; return errors.New("search is unavailable") },
	})
	configs := h.runtimeConfigs(httptest.NewRequest("GET", "/admin/system-health", nil))
	if called != 2 {
		t.Fatalf("probes called %d times", called)
	}
	byName := map[string]string{}
	for _, item := range configs {
		byName[item.Name] = item.Value
	}
	if byName["Payment mode"] != "local" || byName["Database"] != "Ready" || byName["Search"] != "Unavailable" || byName["Redis"] != "Not inspected" || byName["Storage"] != "Not inspected" {
		t.Fatalf("runtime results: %+v", configs)
	}
}
