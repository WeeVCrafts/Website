package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/ports"
)

type pushDeliveryFixture struct{ delivered string }

func (*pushDeliveryFixture) Verify(context.Context, string) (ports.FirebaseIdentity, error) {
	return ports.FirebaseIdentity{}, errors.New("unused")
}
func (*pushDeliveryFixture) ValidSession(context.Context, string, time.Time, string) (bool, error) {
	return true, nil
}
func (*pushDeliveryFixture) DeleteIdentity(context.Context, string) error { return nil }
func (p *pushDeliveryFixture) SendPush(_ context.Context, token string) error {
	p.delivered = token
	return nil
}

func TestPushRegistrationOwnershipAndPrivateResponses(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated QA database")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil || !strings.HasPrefix(config.ConnConfig.Database, "weev_qa_") {
		t.Skip("requires isolated QA database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal("QA database unavailable")
	}
	defer pool.Close()
	var ids [2]int64
	for i := range ids {
		if err = pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status) VALUES($1,'Disposable push fixture','active') RETURNING id`, fmt.Sprintf("push-qa-%d-%d@example.invalid", time.Now().UnixNano(), i)).Scan(&ids[i]); err != nil {
			t.Fatal("fixture creation failed")
		}
	}
	defer func() {
		for _, id := range ids {
			_, _ = pool.Exec(context.Background(), `DELETE FROM customer_push_subscriptions WHERE user_id=$1`, id)
			_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, id)
		}
	}()
	delivery := &pushDeliveryFixture{}
	record := func(id int64, key string) ports.SessionRecord {
		return ports.SessionRecord{ID: key, UserID: id, RoleSlugs: []string{"customer"}, CSRFToken: "fixture-csrf", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	}
	first := record(ids[0], "first-fixture-session")
	second := record(ids[1], "second-fixture-session")
	request := func(session ports.SessionRecord, method, path, body string) *httptest.ResponseRecorder {
		handler := &customerPush{pool: pool, provider: delivery, sessions: applicationauth.NewSessionService(pushSessionFixture{session})}
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: session.ID})
		r.Header.Set("X-CSRF-Token", session.CSRFToken)
		w := httptest.NewRecorder()
		handler.Handle(w, r)
		return w
	}
	token := fmt.Sprintf("disposable-push-token-%d", time.Now().UnixNano())
	body := fmt.Sprintf(`{"token":%q}`, token)
	if w := request(first, http.MethodPost, "/api/v1/customer/push", body); w.Code != 200 {
		t.Fatalf("registration status %d", w.Code)
	}
	if w := request(second, http.MethodPost, "/api/v1/customer/push", body); w.Code != 409 {
		t.Fatal("cross-account token transfer accepted")
	}
	if w := request(second, http.MethodPost, "/api/v1/customer/push/test", "{}"); w.Code != 404 || delivery.delivered != "" {
		t.Fatal("cross-account delivery accepted")
	}
	w := request(first, http.MethodGet, "/api/v1/customer/push", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"enabled":true`) || strings.Contains(w.Body.String(), token) || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("private subscription state contract failed")
	}
	if w = request(first, http.MethodPost, "/api/v1/customer/push/test", "{}"); w.Code != 200 || delivery.delivered != token {
		t.Fatal("owner delivery failed")
	}
	if w = request(second, http.MethodDelete, "/api/v1/customer/push", ""); w.Code != 200 {
		t.Fatal("second account unsubscribe failed")
	}
	if w = request(first, http.MethodGet, "/api/v1/customer/push", ""); !strings.Contains(w.Body.String(), `"enabled":true`) {
		t.Fatal("another account deleted owner subscription")
	}
	if w = request(first, http.MethodDelete, "/api/v1/customer/push", ""); w.Code != 200 {
		t.Fatal("owner unsubscribe failed")
	}
	if w = request(first, http.MethodGet, "/api/v1/customer/push", ""); !strings.Contains(w.Body.String(), `"enabled":false`) {
		t.Fatal("unsubscribed token retained")
	}
}
