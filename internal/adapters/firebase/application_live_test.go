package firebase

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	firebaseauth "firebase.google.com/go/v4/auth"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestLiveFirebaseApplicationSession(t *testing.T) {
	base := os.Getenv("WECRATFS_FIREBASE_TEST_BASE_URL")
	if os.Getenv("WECRATFS_FIREBASE_LIVE_TEST") != "1" || base == "" {
		t.Skip("requires isolated QA application and live Firebase opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	p, err := New(ctx, "weevcrafts-708af", os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if err != nil {
		t.Fatal("Firebase provider unavailable")
	}
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal("QA database unavailable")
	}
	defer pool.Close()
	random := make([]byte, 16)
	if _, err = rand.Read(random); err != nil {
		t.Fatal("fixture generation failed")
	}
	suffix := hex.EncodeToString(random)
	email := "qa-http-" + suffix + "@example.invalid"
	password := "QA-" + suffix + "!"
	user, err := p.auth.CreateUser(ctx, (&firebaseauth.UserToCreate{}).UID("qa-http-"+suffix).Email(email).Password(password).EmailVerified(true).DisplayName("Disposable HTTP fixture"))
	if err != nil {
		t.Fatal("Firebase fixture unavailable")
	}
	var userID int64
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if p.DeleteIdentity(cleanup, user.UID) != nil {
			t.Error("Firebase HTTP fixture cleanup failed")
		}
		if userID == 0 {
			_ = pool.QueryRow(cleanup, `SELECT id FROM users WHERE email=$1`, email).Scan(&userID)
		}
		if userID > 0 {
			_, _ = pool.Exec(cleanup, `DELETE FROM firebase_identities WHERE user_id=$1`, userID)
			_, _ = pool.Exec(cleanup, `DELETE FROM carts WHERE user_id=$1`, userID)
			_, _ = pool.Exec(cleanup, `DELETE FROM consent_records WHERE user_id=$1`, userID)
			_, _ = pool.Exec(cleanup, `DELETE FROM audit_logs WHERE actor_id=$1`, strconv.FormatInt(userID, 10))
			_, _ = pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, userID)
		}
	}()
	authBody, _ := json.Marshal(map[string]any{"email": email, "password": password, "returnSecureToken": true})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key="+os.Getenv("WECRATFS_FIREBASE_TEST_API_KEY"), bytes.NewReader(authBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("Firebase fixture sign-in unavailable")
	}
	var identity struct {
		IDToken string `json:"idToken"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(resp.Body).Decode(&identity) != nil {
		resp.Body.Close()
		t.Fatal("Firebase fixture sign-in rejected")
	}
	resp.Body.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 20 * time.Second}
	resp, err = client.Get(base + "/login")
	if err != nil {
		t.Fatal("QA login page unavailable")
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	csrf := ""
	for _, c := range resp.Cookies() {
		if c.Name == "wecratfs_csrf" {
			csrf = c.Value
		}
	}
	if csrf == "" {
		t.Fatal("login CSRF cookie missing")
	}
	body, _ := json.Marshal(map[string]any{"id_token": identity.IDToken, "accepted_terms": true, "phone": "+919876543210", "next": "/checkout"})
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/auth/firebase/session", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", base)
	request.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(request)
	if err != nil {
		t.Fatal("Firebase application session unavailable")
	}
	var result struct {
		Data struct {
			ID    int64    `json:"id"`
			Roles []string `json:"roles"`
		} `json:"data"`
		Redirect string `json:"redirect_to"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(resp.Body).Decode(&result) != nil {
		resp.Body.Close()
		t.Fatalf("verified application session rejected with status %d", resp.StatusCode)
	}
	resp.Body.Close()
	userID = result.Data.ID
	if userID == 0 || len(result.Data.Roles) != 1 || result.Data.Roles[0] != "customer" || result.Redirect != "/checkout" {
		t.Fatal("customer roles or safe return path changed")
	}
	hasPrivateCookie := false
	for _, c := range resp.Cookies() {
		if c.Name == "wecratfs_session" {
			hasPrivateCookie = c.HttpOnly && c.SameSite == http.SameSiteLaxMode
		}
	}
	if !hasPrivateCookie {
		t.Fatal("first-party authentication cookie is not private")
	}
	resp, err = client.Get(base + "/api/v1/auth/session")
	if err != nil {
		t.Fatal("session check unavailable")
	}
	status := resp.StatusCode
	var current struct {
		Data bool `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&current)
	resp.Body.Close()
	if status != 200 || !current.Data {
		t.Fatal("new authenticated session unusable")
	}
	timer := time.NewTimer(time.Second + 100*time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		t.Fatal("fixture timed out")
	}
	if p.auth.RevokeRefreshTokens(ctx, user.UID) != nil {
		t.Fatal("Firebase revocation failed")
	}
	resp, err = client.Get(base + "/api/v1/auth/session")
	if err != nil {
		t.Fatal("revocation check unavailable")
	}
	status = resp.StatusCode
	current.Data = true
	_ = json.NewDecoder(resp.Body).Decode(&current)
	resp.Body.Close()
	if status != 200 || current.Data {
		t.Fatal("Firebase revocation did not invalidate application cookie")
	}
}
