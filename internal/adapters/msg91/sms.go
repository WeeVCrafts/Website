package msg91

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/wecratfs/commerce/internal/domain/communication"
	"github.com/wecratfs/commerce/internal/ports"
)

type Sender struct {
	key, senderID, endpoint string
	templates               map[string]ports.SMSTemplate
	client                  *http.Client
}

func New(key, senderID string, templates map[string]ports.SMSTemplate) (*Sender, error) {
	if len(key) < 16 || len(key) > 256 || strings.ContainsAny(key, "\r\n\x00\t ") || !regexp.MustCompile(`^[A-Z]{6}$`).MatchString(senderID) {
		return nil, errors.New("MSG91 sender configuration invalid")
	}
	return &Sender{key: key, senderID: senderID, templates: templates, endpoint: "https://control.msg91.com/api/v5/flow", client: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (s *Sender) SendTransactionalSMS(ctx context.Context, job ports.SMSJob) (ports.SMSResult, error) {
	uncertain := ports.SMSResult{Outcome: "uncertain"}
	t, ok := s.templates[job.TemplateKey]
	phone, valid := communication.IndianMobile(job.Phone)
	if !ok || !valid || !regexp.MustCompile(`^[a-fA-F0-9]{24}$`).MatchString(t.ID) || len(t.Variables) != 2 || !safeVariable(job.Reference) || !safeVariable(job.Update) {
		return ports.SMSResult{Outcome: "skipped"}, errors.New("SMS template or recipient invalid")
	}
	recipient := map[string]string{"mobiles": phone}
	seen := map[string]bool{}
	for key, canonical := range t.Variables {
		if strings.EqualFold(key, "mobiles") || !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,31}$`).MatchString(key) || canonical != "reference" && canonical != "status" || seen[canonical] {
			return ports.SMSResult{Outcome: "skipped"}, errors.New("SMS variable mapping invalid")
		}
		seen[canonical] = true
		if canonical == "reference" {
			recipient[key] = job.Reference
		} else {
			recipient[key] = job.Update
		}
	}
	// The approved MSG91 template owns the sender/DLT mapping. No arbitrary
	// message body, route override, international bypass or URL shortening.
	body, _ := json.Marshal(map[string]any{"template_id": t.ID, "sender": s.senderID, "short_url": "0", "recipients": []map[string]string{recipient}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return ports.SMSResult{Outcome: "skipped"}, errors.New("SMS preparation failed")
	}
	req.Header.Set("authkey", s.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	response, err := s.client.Do(req)
	if err != nil {
		return uncertain, errors.New("SMS submission uncertain; inspect provider logs before retry")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	if err != nil || len(raw) > 64<<10 {
		return uncertain, errors.New("SMS response uncertain")
	}
	var result struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return uncertain, errors.New("SMS response uncertain")
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 && result.Type == "success" && regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`).MatchString(result.Message) {
		return ports.SMSResult{Outcome: "accepted", ProviderID: result.Message}, nil
	}
	// Only an explicit rejection establishes that another send is safe. It
	// still requires an operator to correct the cause and approve a retry.
	if response.StatusCode < 500 && result.Type == "error" {
		return ports.SMSResult{Outcome: "rejected"}, errors.New("MSG91 rejected the SMS; inspect template/account configuration")
	}
	return uncertain, errors.New("SMS submission uncertain; inspect provider logs before retry")
}

func safeVariable(value string) bool {
	return len(value) > 0 && len(value) <= 80 && !strings.ContainsAny(value, "\r\n\x00\t")
}
