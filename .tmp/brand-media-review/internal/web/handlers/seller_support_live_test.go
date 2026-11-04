package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	redisadapter "github.com/wecratfs/commerce/internal/adapters/redis"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/ports"
)

func liveSupportFixture(t *testing.T) (*pgxpool.Pool, *applicationauth.SessionService, []int64, []ports.SessionRecord) {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" || os.Getenv("REDIS_ADDR") == "" {
		t.Skip("real PostgreSQL and Redis required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.ApplyMigrations(ctx, pool); err != nil {
		t.Fatal(err)
	}
	client := redisclient.NewClient(&redisclient.Options{Addr: os.Getenv("REDIS_ADDR")})
	t.Cleanup(func() { _ = client.Close() })
	service := applicationauth.NewSessionService(redisadapter.NewSessionStore(client))
	var ids []int64
	var sessions []ports.SessionRecord
	t.Cleanup(func() {
		for _, session := range sessions {
			_ = service.Revoke(context.Background(), session.ID)
		}
		for _, id := range ids {
			_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE actor_id=$1`, fmt.Sprint(id))
			_, _ = pool.Exec(context.Background(), `DELETE FROM support_tickets WHERE user_id=$1`, id)
			_, _ = pool.Exec(context.Background(), `DELETE FROM return_requests WHERE user_id=$1`, id)
			_, _ = pool.Exec(context.Background(), `DELETE FROM orders WHERE user_id=$1`, id)
			_, _ = pool.Exec(context.Background(), `DELETE FROM sellers WHERE owner_user_id=$1`, id)
			_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, id)
		}
	})
	for index, role := range []string{"seller", "seller", "customer", "customer", "customer"} {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status) VALUES($1,'Live support QA','active') RETURNING id`, fmt.Sprintf("seller-support-%d-%d@example.invalid", time.Now().UnixNano(), index)).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,$2)`, id, role); err != nil {
			t.Fatal(err)
		}
		if role == "seller" {
			if _, err := pool.Exec(ctx, `INSERT INTO sellers(owner_user_id,display_name,status) VALUES($1,'Support QA seller','active')`, id); err != nil {
				t.Fatal(err)
			}
		}
		// Session creation uses real Redis; authorization reads persisted database roles.
		var session ports.SessionRecord
		if err := client.Ping(ctx).Err(); err != nil {
			t.Fatal(err)
		}
		session = ports.SessionRecord{ID: fmt.Sprintf("seller-support-test-%d", id), UserID: id, CSRFToken: fmt.Sprintf("csrf-%d", id), ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now()}
		if err := redisadapter.NewSessionStore(client).Create(ctx, session); err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, session)
	}
	return pool, service, ids, sessions
}

func supportTestRequest(method, path string, form url.Values, session ports.SessionRecord) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: session.ID})
	return r
}

func TestSellerSupportLivePersistsRepliesAndRejectsForeignTickets(t *testing.T) {
	pool, sessions, ids, records := liveSupportFixture(t)
	h := &LivePortalHandler{pool: pool, sessions: sessions}
	invoke := func(method, path string, form url.Values, session ports.SessionRecord) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		if !h.serveSellerSupport(response, supportTestRequest(method, path, form, session)) {
			t.Fatal("support route not handled")
		}
		return response
	}
	response := invoke(http.MethodPost, "/seller-admin/support/create", url.Values{"csrf_token": {records[0].CSRFToken}, "subject": {"Persistent seller help"}, "message": {"Please review my catalogue application."}}, records[0])
	if response.Code != http.StatusSeeOther {
		t.Fatalf("create status %d: %s", response.Code, response.Body.String())
	}
	var ticketID int64
	if err := pool.QueryRow(context.Background(), `SELECT id FROM support_tickets WHERE user_id=$1 AND subject='Persistent seller help'`, ids[0]).Scan(&ticketID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE support_tickets SET status='waiting_customer',agent_note='Private note' WHERE id=$1`, ticketID); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"csrf_token": {records[0].CSRFToken}, "ticket_id": {fmt.Sprint(ticketID)}, "message": {"Catalogue details supplied by seller."}}
	response = invoke(http.MethodPost, "/seller-admin/support/reply", form, records[0])
	if response.Code != http.StatusSeeOther {
		t.Fatalf("reply: %d %s", response.Code, response.Body.String())
	}
	var role, body, status, note string
	if err := pool.QueryRow(context.Background(), `SELECT m.author_role,m.body,t.status,t.agent_note FROM support_ticket_messages m JOIN support_tickets t ON t.id=m.ticket_id WHERE t.id=$1`, ticketID).Scan(&role, &body, &status, &note); err != nil {
		t.Fatal(err)
	}
	if role != "seller" || body != form.Get("message") || status != "in_progress" || note != "Private note" {
		t.Fatalf("incorrect persisted conversation: %s %s %s %s", role, body, status, note)
	}
	response = invoke(http.MethodGet, "/seller-admin/support?q=Catalogue+details", nil, records[0])
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), body) || strings.Contains(response.Body.String(), "Private note") {
		t.Fatalf("seller conversation missing or internal note exposed: %d", response.Code)
	}
	form.Set("csrf_token", records[1].CSRFToken)
	response = invoke(http.MethodPost, "/seller-admin/support/reply", form, records[1])
	if response.Code != http.StatusForbidden {
		t.Fatalf("foreign reply returned %d", response.Code)
	}
	response = invoke(http.MethodGet, "/seller-admin/support?q=Persistent", nil, records[1])
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "Persistent seller help") {
		t.Fatalf("foreign ticket exposed: %d", response.Code)
	}
	for _, session := range []ports.SessionRecord{records[2], records[4]} {
		response = invoke(http.MethodGet, "/seller-admin/support", nil, session)
		if response.Code != http.StatusForbidden {
			t.Fatalf("non-owner allowed %d", response.Code)
		}
	}
	form.Set("csrf_token", "invalid")
	response = invoke(http.MethodPost, "/seller-admin/support/reply", form, records[0])
	if response.Code != http.StatusForbidden {
		t.Fatalf("CSRF allowed %d", response.Code)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE support_tickets SET status='closed' WHERE id=$1`, ticketID); err != nil {
		t.Fatal(err)
	}
	form.Set("csrf_token", records[0].CSRFToken)
	response = invoke(http.MethodPost, "/seller-admin/support/reply", form, records[0])
	if response.Code != http.StatusConflict {
		t.Fatalf("closed ticket reply returned %d", response.Code)
	}
}
