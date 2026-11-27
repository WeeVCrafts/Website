package handlers

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	redisclient "github.com/redis/go-redis/v9"
	redisadapter "github.com/wecratfs/commerce/internal/adapters/redis"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
)

func TestAddressEditRejectsForeignOrMissingAddressWithoutCreatingOne(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" || os.Getenv("REDIS_ADDR") == "" {
		t.Skip("real PostgreSQL and Redis required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	client := redisclient.NewClient(&redisclient.Options{Addr: os.Getenv("REDIS_ADDR")})
	t.Cleanup(func() { _ = client.Close() })
	var ownerID, otherID, addressID int64
	for index, target := range []*int64{&ownerID, &otherID} {
		if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status) VALUES($1,'Address QA','active') RETURNING id`, fmt.Sprintf("address-qa-%d-%d@example.invalid", time.Now().UnixNano(), index)).Scan(target); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'customer')`, *target); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM addresses WHERE user_id IN ($1,$2)`, ownerID, otherID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id IN ($1,$2)`, ownerID, otherID)
	})
	if err := pool.QueryRow(ctx, `INSERT INTO addresses(user_id,recipient_name,phone,line1,city,state,postal_code,country_code) VALUES($1,'Original owner','9000000000','1 QA Lane','Bengaluru','Karnataka','560001','IN') RETURNING id`, ownerID).Scan(&addressID); err != nil {
		t.Fatal(err)
	}
	sessions := applicationauth.NewSessionService(redisadapter.NewSessionStore(client))
	session, err := sessions.Start(ctx, domainidentity.User{ID: otherID}, "QA", "127.0.0.1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sessions.Revoke(context.Background(), session.ID) })
	handler := &LiveStorefrontHandler{pool: pool, sessions: sessions}
	for _, id := range []int64{addressID, -1} {
		if id < 0 {
			id = addressID + 1000000000
		}
		form := url.Values{"csrf_token": {session.CSRFToken}, "address_id": {fmt.Sprint(id)}, "recipient_name": {"New recipient"}, "phone": {"9000000000"}, "line1": {"2 QA Lane"}, "city": {"Bengaluru"}, "state": {"Karnataka"}, "postal_code": {"560001"}, "country_code": {"IN"}}
		request := httptest.NewRequest(http.MethodPost, "/account/addresses", strings.NewReader(form.Encode())).WithContext(ctx)
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: session.ID})
		response := httptest.NewRecorder()
		handler.saveAddress(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("address %d returned %d: %s", id, response.Code, response.Body.String())
		}
	}
	var count int
	var recipient string
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM addresses WHERE user_id=$1`, otherID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT recipient_name FROM addresses WHERE id=$1`, addressID).Scan(&recipient); err != nil {
		t.Fatal(err)
	}
	if count != 0 || recipient != "Original owner" {
		t.Fatalf("foreign/missing edit changed persisted data: count=%d recipient=%q", count, recipient)
	}
	t.Run("multipart portal save and delete require session CSRF", func(t *testing.T) {
		post := func(path string, fields map[string]string, handle http.HandlerFunc) *httptest.ResponseRecorder {
			t.Helper()
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			for name, value := range fields {
				if err := writer.WriteField(name, value); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, path, &body).WithContext(ctx)
			request.Header.Set("Content-Type", writer.FormDataContentType())
			request.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: session.ID})
			response := httptest.NewRecorder()
			handle(response, request)
			return response
		}
		fields := map[string]string{
			"return_to": "/seller-admin/profile", "recipient_name": "QA recipient", "phone": "9000000000",
			"line1": "3 QA Lane", "city": "Bengaluru", "state": "Karnataka", "postal_code": "560001", "country_code": "IN",
		}
		for _, token := range []string{"", "incorrect"} {
			fields["csrf_token"] = token
			response := post("/seller-admin/addresses", fields, handler.saveAddress)
			if response.Code != http.StatusForbidden {
				t.Fatalf("invalid token returned %d, want 403", response.Code)
			}
		}
		fields["csrf_token"] = session.CSRFToken
		response := post("/seller-admin/addresses", fields, handler.saveAddress)
		if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/seller-admin/profile?notice=Address+saved" {
			t.Fatalf("valid save returned %d: %s", response.Code, response.Body.String())
		}
		var savedID int64
		if err := pool.QueryRow(ctx, `SELECT id FROM addresses WHERE user_id=$1 AND recipient_name='QA recipient'`, otherID).Scan(&savedID); err != nil {
			t.Fatal(err)
		}
		deleteFields := map[string]string{
			"csrf_token": session.CSRFToken, "address_id": fmt.Sprint(savedID), "return_to": "/seller-admin/profile",
		}
		response = post("/seller-admin/addresses/delete", deleteFields, handler.deleteAddress)
		if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/seller-admin/profile?notice=Address+deleted" {
			t.Fatalf("valid delete returned %d: %s", response.Code, response.Body.String())
		}
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM addresses WHERE user_id=$1`, otherID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("after deleting saved address: count=%d err=%v", count, err)
		}
	})
}
