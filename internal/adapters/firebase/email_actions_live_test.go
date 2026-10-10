package firebase

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	firebaseauth "firebase.google.com/go/v4/auth"
	emailadapter "github.com/wecratfs/commerce/internal/adapters/email"
	"github.com/wecratfs/commerce/internal/ports"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

// Uses only a disposable identity; no real customer's password or inbox.
func TestLiveFirebaseEmailActions(t *testing.T) {
	if os.Getenv("WECRATFS_FIREBASE_LIVE_TEST") != "1" {
		t.Skip("opt-in live Firebase fixture test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	p, err := New(ctx, "weevcrafts-708af", os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if err != nil {
		t.Fatal("provider unavailable")
	}
	random := make([]byte, 16)
	if _, err = rand.Read(random); err != nil {
		t.Fatal("fixture generation failed")
	}
	suffix := hex.EncodeToString(random)
	email := "qa-mail-" + suffix + "@example.invalid"
	password := "QA-" + suffix + "!"
	user, err := p.auth.CreateUser(ctx, (&firebaseauth.UserToCreate{}).UID("qa-mail-"+suffix).Email(email).Password(password).EmailVerified(false))
	if err != nil {
		t.Fatal("fixture creation failed")
	}
	defer func() {
		cleanup, c := context.WithTimeout(context.Background(), 15*time.Second)
		defer c()
		if p.DeleteIdentity(cleanup, user.UID) != nil {
			t.Error("fixture cleanup failed")
		}
	}()
	proof := ports.FirebaseIdentity{UID: user.UID, Email: email}
	post := func(action string, payload map[string]any) bool {
		body, _ := json.Marshal(payload)
		request, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://identitytoolkit.googleapis.com/v1/accounts:"+action+"?key="+os.Getenv("WECRATFS_FIREBASE_TEST_API_KEY"), bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response, e := http.DefaultClient.Do(request)
		if e != nil {
			return false
		}
		defer response.Body.Close()
		return response.StatusCode == 200
	}
	verification, err := p.EmailVerificationLink(ctx, proof, "https://weevcrafts.com/")
	if err != nil {
		t.Fatal("verification link generation rejected")
	}
	// Separate opt-in delivery run, only to the explicitly selected QA inbox.
	var sender ports.EmailSender
	if inbox := os.Getenv("WECRATFS_RESEND_TEST_INBOX"); inbox != "" {
		key, e := os.ReadFile(os.Getenv("WECRATFS_RESEND_API_KEY_FILE"))
		if e != nil {
			t.Fatal("test mail credential unavailable")
		}
		sender, e = emailadapter.NewResend(string(key), "Weevcrafts <no-reply@mail.weevcrafts.com>", "weevcrafts@gmail.com")
		if e != nil || sender.Send(ctx, ports.EmailMessage{To: inbox, DisplayName: "Email integration test", Subject: "Verify your Weevcrafts email", ActionURL: verification}) != nil {
			t.Fatal("branded verification delivery rejected")
		}
	}
	link, err := url.Parse(verification)
	if err != nil || link.Query().Get("oobCode") == "" || link.Query().Get("continueUrl") != "https://weevcrafts.com/" {
		t.Fatal("invalid verification action")
	}
	if !post("update", map[string]any{"oobCode": link.Query().Get("oobCode")}) {
		t.Fatal("verification action rejected")
	}
	verified, err := p.auth.GetUser(ctx, user.UID)
	if err != nil || !verified.EmailVerified {
		t.Fatal("verification did not persist")
	}
	reset, err := p.PasswordResetLink(ctx, proof, "https://weevcrafts.com/")
	if err != nil {
		t.Fatal("reset link generation rejected")
	}
	if sender != nil && sender.Send(ctx, ports.EmailMessage{To: os.Getenv("WECRATFS_RESEND_TEST_INBOX"), DisplayName: "Email integration test", Subject: "Reset your Weevcrafts password", ActionURL: reset}) != nil {
		t.Fatal("branded reset delivery rejected")
	}
	link, err = url.Parse(reset)
	if err != nil || link.Query().Get("oobCode") == "" {
		t.Fatal("invalid reset action")
	}
	newPassword := password + "new"
	if !post("resetPassword", map[string]any{"oobCode": link.Query().Get("oobCode"), "newPassword": newPassword}) {
		t.Fatal("reset action rejected")
	}
	if post("signInWithPassword", map[string]any{"email": email, "password": password, "returnSecureToken": true}) {
		t.Fatal("old password still accepted")
	}
	if !post("signInWithPassword", map[string]any{"email": email, "password": newPassword, "returnSecureToken": true}) {
		t.Fatal("reset password sign-in failed")
	}
	if post("resetPassword", map[string]any{"oobCode": link.Query().Get("oobCode"), "newPassword": password}) {
		t.Fatal("reset action reused")
	}
	proof.Email = "different@example.invalid"
	if _, err = p.EmailVerificationLink(ctx, proof, "https://weevcrafts.com/"); err == nil {
		t.Fatal("cross-identity email action allowed")
	}
}
