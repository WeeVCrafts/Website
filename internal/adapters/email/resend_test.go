package email

import (
	"context"
	"encoding/json"
	"github.com/wecratfs/commerce/internal/ports"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResendBrandedMessageAndPrivateFailure(t *testing.T) {
	var payload struct {
		From, Subject, HTML, Text string
		To                        []string
	}
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-fixture" {
			t.Error("server authentication missing")
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error("invalid email payload")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"id":"fixture","message":"private-fixture https://example.invalid/reset?token=secret"}`))
	}))
	defer server.Close()
	sender, err := NewResend("private-fixture", "WeevCrafts <no-reply@example.invalid>", "")
	if err != nil {
		t.Fatal(err)
	}
	sender.endpoint = server.URL
	message := ports.EmailMessage{To: "customer@example.invalid", Subject: "Reset your WeevCrafts password", DisplayName: `<script>fixture</script>`, ActionURL: "https://example.invalid/reset?token=secret"}
	if err = sender.Send(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	if payload.From != "WeevCrafts <no-reply@example.invalid>" || len(payload.To) != 1 || payload.To[0] != message.To || !strings.Contains(payload.HTML, "WeevCrafts") || !strings.Contains(payload.HTML, "Reset password") || strings.Contains(payload.HTML, "<script>") || !strings.Contains(payload.Text, message.ActionURL) {
		t.Fatal("branded HTML/plain-text contract failed")
	}
	status = http.StatusForbidden
	err = sender.Send(context.Background(), message)
	if err == nil || strings.Contains(err.Error(), "private-fixture") || strings.Contains(err.Error(), "secret") {
		t.Fatal("provider failure leaked private details")
	}
}

func TestResendRejectsUnsafeMessageBeforeNetwork(t *testing.T) {
	sender, _ := NewResend("fixture-key", "WeevCrafts <no-reply@example.invalid>", "")
	for _, message := range []ports.EmailMessage{{To: "customer@example.invalid\r\nBcc:other@example.invalid", Subject: "Reset", ActionURL: "https://example.invalid"}, {To: "customer@example.invalid", Subject: "Reset", ActionURL: "javascript:alert(1)"}} {
		if sender.Send(context.Background(), message) == nil {
			t.Fatal("unsafe email accepted")
		}
	}
	if _, err := NewResend("fixture-key", "Bad\r\nFrom:sender@example.invalid", ""); err == nil {
		t.Fatal("unsafe sender accepted")
	}
}
