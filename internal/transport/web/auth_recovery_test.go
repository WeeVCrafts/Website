package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
)

func TestForgotPasswordRendersCSRFForHTMLSubmission(t *testing.T) {
	h := &Handler{recoveryService: applicationauth.NewRecoveryService(nil, nil, nil, "https://example.invalid")}
	response := httptest.NewRecorder()
	h.forgotPassword(response, httptest.NewRequest(http.MethodGet, "/forgot-password", nil))
	if response.Code != http.StatusOK || len(response.Result().Cookies()) == 0 || !strings.Contains(response.Body.String(), `name="csrf_token"`) {
		t.Fatal("forgot-password form needs the CSRF cookie and a matching form field")
	}
}
