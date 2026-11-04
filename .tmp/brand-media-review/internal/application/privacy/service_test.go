package privacy

import (
	"context"
	"errors"
	"testing"

	domainprivacy "github.com/wecratfs/commerce/internal/domain/privacy"
)

type privacyRepositoryStub struct {
	requestCalls int
	request      domainprivacy.Request
}

func (s *privacyRepositoryStub) GetCenter(context.Context, int64) (domainprivacy.Center, error) {
	return domainprivacy.Center{}, nil
}

func (s *privacyRepositoryStub) RecordConsent(context.Context, int64, string, string, bool, string) error {
	return nil
}

func (s *privacyRepositoryStub) SetEmailMarketing(context.Context, int64, bool) error { return nil }

func (s *privacyRepositoryStub) CreatePrivacyRequest(_ context.Context, _ int64, requestType, details string) (domainprivacy.Request, error) {
	s.requestCalls++
	s.request = domainprivacy.Request{Type: requestType, Details: details}
	return s.request, nil
}

func (s *privacyRepositoryStub) ExecuteDeletion(context.Context, int64, int64) error { return nil }

func TestRequestRequiresNonBlankDetailsAndTrimsInput(t *testing.T) {
	repository := &privacyRepositoryStub{}
	service := NewService(repository)

	for _, details := range []string{"", "   \n\t"} {
		if _, err := service.Request(context.Background(), 9, "access", details); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("blank details %q returned %v", details, err)
		}
	}
	if repository.requestCalls != 0 {
		t.Fatalf("blank request reached repository %d times", repository.requestCalls)
	}

	if _, err := service.Request(context.Background(), 9, " access ", "  show my data  "); err != nil {
		t.Fatalf("valid request was rejected: %v", err)
	}
	if repository.requestCalls != 1 || repository.request.Type != "access" || repository.request.Details != "show my data" {
		t.Fatalf("request was not normalized before persistence: %+v calls=%d", repository.request, repository.requestCalls)
	}
}
