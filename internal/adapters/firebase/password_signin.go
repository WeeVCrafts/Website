package firebase

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

var passwordSignInClient = &http.Client{
	Timeout: 8 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func (p *Provider) WithWebAPIKey(key string) {
	p.webAPIKey = key
}

func (p *Provider) PasswordSignIn(ctx context.Context, email, password string) (string, error) {
	if p.webAPIKey == "" {
		return "", ports.ErrCredentialsNotFound
	}
	body, err := json.Marshal(struct {
		Email             string `json:"email"`
		Password          string `json:"password"`
		ReturnSecureToken bool   `json:"returnSecureToken"`
	}{Email: email, Password: password, ReturnSecureToken: true})
	if err != nil {
		return "", ports.ErrCredentialsNotFound
	}
	const action = "identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key="
	endpoint := "https://" + action + url.QueryEscape(p.webAPIKey)
	if p.authEmulatorHost != "" {
		endpoint = "http://" + p.authEmulatorHost + "/" + action + url.QueryEscape(p.webAPIKey)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", ports.ErrCredentialsNotFound
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := passwordSignInClient.Do(request)
	if err != nil {
		return "", ports.ErrCredentialsNotFound
	}
	defer response.Body.Close()
	var result struct {
		IDToken string `json:"idToken"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&result) != nil || result.IDToken == "" {
		return "", ports.ErrCredentialsNotFound
	}
	return result.IDToken, nil
}
