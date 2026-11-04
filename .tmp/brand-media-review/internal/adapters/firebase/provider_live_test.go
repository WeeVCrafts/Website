package firebase

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	firebaseauth "firebase.google.com/go/v4/auth"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestLiveFirebaseProofAndRevocation(t *testing.T) {
	if os.Getenv("WECRATFS_FIREBASE_LIVE_TEST") != "1" {
		t.Skip("opt-in live Firebase fixture test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	p, err := New(ctx, "weevcrafts-708af", os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if err != nil {
		t.Fatal("Firebase provider unavailable")
	}
	random := make([]byte, 16)
	if _, err = rand.Read(random); err != nil {
		t.Fatal("fixture generation failed")
	}
	suffix := hex.EncodeToString(random)
	email := "qa-" + suffix + "@example.invalid"
	password := "QA-" + suffix + "!"
	user, err := p.auth.CreateUser(ctx, (&firebaseauth.UserToCreate{}).UID("qa-"+suffix).Email(email).Password(password).EmailVerified(true).DisplayName("Disposable QA fixture"))
	if err != nil {
		t.Fatal("Firebase fixture creation failed")
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		if p.DeleteIdentity(cleanupCtx, user.UID) != nil {
			t.Error("Firebase fixture cleanup failed")
		}
	}()
	body, _ := json.Marshal(map[string]any{"email": email, "password": password, "returnSecureToken": true})
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key="+os.Getenv("WECRATFS_FIREBASE_TEST_API_KEY"), bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal("Firebase fixture sign-in unavailable")
	}
	defer response.Body.Close()
	var result struct {
		IDToken string `json:"idToken"`
	}
	if response.StatusCode != 200 || json.NewDecoder(response.Body).Decode(&result) != nil {
		t.Fatal("Firebase fixture sign-in rejected")
	}
	proof, err := p.Verify(ctx, result.IDToken)
	if err != nil || proof.UID != user.UID || proof.Provider != "password" || !proof.EmailVerified {
		t.Fatal("real Firebase proof failed verification")
	}
	issued := time.Now()
	active, err := p.ValidSession(ctx, user.UID, issued, email)
	if err != nil || !active {
		t.Fatal("new Firebase session incorrectly rejected")
	}
	timer := time.NewTimer(time.Second + 100*time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		t.Fatal("fixture timed out")
	}
	if p.auth.RevokeRefreshTokens(ctx, user.UID) != nil {
		t.Fatal("fixture revocation failed")
	}
	active, err = p.ValidSession(ctx, user.UID, issued, email)
	if err != nil || active {
		t.Fatal("revoked Firebase first-party session stayed active")
	}
	if _, err = p.Verify(ctx, result.IDToken); err == nil {
		t.Fatal("revoked Firebase token accepted")
	}
	if _, err = p.Verify(ctx, "invalid.jwt.signature"); err == nil {
		t.Fatal("forged Firebase token accepted")
	}
}
