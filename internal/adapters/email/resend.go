package email

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

//go:embed templates/action.html
var actionTemplate string

type Resend struct {
	key, from, replyTo, endpoint string
	client                       *http.Client
}

func NewResend(key, from, replyTo string) (*Resend, error) {
	key = strings.TrimSpace(key)
	if key == "" || !validHeader(key) || !validHeader(from) {
		return nil, errors.New("Resend configuration incomplete")
	}
	if _, err := mail.ParseAddress(from); err != nil {
		return nil, errors.New("Resend sender invalid")
	}
	if replyTo != "" {
		if _, err := mail.ParseAddress(replyTo); err != nil || !validHeader(replyTo) {
			return nil, errors.New("Resend reply address invalid")
		}
	}
	return &Resend{key: key, from: from, replyTo: replyTo, endpoint: "https://api.resend.com/emails", client: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (s *Resend) Send(ctx context.Context, message ports.EmailMessage) error {
	recipient, err := mail.ParseAddress(message.To)
	action, actionErr := url.Parse(message.ActionURL)
	if err != nil || recipient.Address != message.To || !validHeader(message.To) || !validHeader(message.Subject) || actionErr != nil || action.Host == "" || (action.Scheme != "https" && !(action.Scheme == "http" && (action.Hostname() == "localhost" || action.Hostname() == "127.0.0.1"))) || action.User != nil {
		return errors.New("email message invalid")
	}
	label, intro := "View account update", "An update is available in your Weevcrafts account."
	if strings.Contains(strings.ToLower(message.Subject), "reset") {
		label = "Reset password"
		intro = "We received a request to reset your Weevcrafts password. Use the secure link below to choose a new password."
	}
	if strings.Contains(strings.ToLower(message.Subject), "verify") {
		label = "Verify email"
		intro = "Confirm your email address to protect your Weevcrafts account."
	}
	if message.Body != "" {
		intro = message.Body
	}
	data := struct{ Title, Intro, Name, ActionURL, Label string }{message.Subject, intro, message.DisplayName, message.ActionURL, label}
	view, err := template.New("action").Parse(actionTemplate)
	if err != nil {
		return errors.New("email template unavailable")
	}
	var html bytes.Buffer
	if view.Execute(&html, data) != nil {
		return errors.New("email template unavailable")
	}
	payload := map[string]any{"from": s.from, "to": []string{message.To}, "subject": message.Subject, "html": html.String(), "text": message.Subject + "\n\n" + intro + "\n\n" + message.ActionURL + "\n\nIf you did not request this account action, you can ignore this email."}
	if s.replyTo != "" {
		payload["reply_to"] = s.replyTo
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return errors.New("email preparation failed")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(raw))
	if err != nil {
		return errors.New("email preparation failed")
	}
	request.Header.Set("Authorization", "Bearer "+s.key)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return errors.New("email delivery unavailable")
	}
	defer response.Body.Close()
	// Provider bodies may contain addresses, action URLs or credential details.
	// Never return them to transports, logs or tests.
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8192))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New("email delivery unavailable")
	}
	return nil
}
