package middleware

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
)

// Recovery wraps an http.Handler with panic recovery to prevent unhandled runtime
// panics from terminating the HTTP server. It logs structured errors and returns a 500.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := debug.Stack()
					reqID := r.Header.Get("X-Request-ID")

					if logger != nil {
						logger.Error("HTTP panic recovered",
							"error", fmt.Sprint(rec),
							"request_id", reqID,
							"method", r.Method,
							"path", r.URL.Path,
							"remote_addr", r.RemoteAddr,
							"stack", string(stack),
						)
					}

					if strings.HasPrefix(r.Header.Get("Accept"), "application/json") || strings.HasPrefix(r.URL.Path, "/api/") {
						w.Header().Set("Content-Type", "application/json; charset=utf-8")
						w.WriteHeader(http.StatusInternalServerError)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"error": map[string]string{
								"code":    "INTERNAL_SERVER_ERROR",
								"message": "An unexpected error occurred. Please try again later.",
							},
						})
						return
					}

					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>Something went wrong - Weevcrafts</title><meta name="viewport" content="width=device-width, initial-scale=1"></head><body style="font-family:sans-serif;text-align:center;padding:50px 20px;"><h1>Something went wrong</h1><p>We encountered an unexpected error while processing your request. Please try again.</p><p><a href="/" style="color:#0f392b;font-weight:bold;">Return to Home</a></p></body></html>`))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
