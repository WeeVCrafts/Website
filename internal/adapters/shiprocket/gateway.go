package shiprocket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

const (
	SandboxURL = "https://api-sandbox.shiprocket.in"
	LiveURL    = "https://apiv2.shiprocket.in"
)

type Gateway struct {
	base, email, password string
	client                *http.Client
	mu                    sync.Mutex
	token                 string
	courierResult         func(bool, error)
	authDone              chan struct{}
}

func New(base, email, password string) *Gateway {
	return &Gateway{base: base, email: email, password: password, client: &http.Client{
		Timeout:       45 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (g *Gateway) ObserveCourierResults(observer func(bool, error)) { g.courierResult = observer }

func (g *Gateway) authenticate(ctx context.Context, expired string) (string, error) {
	for {
		g.mu.Lock()
		if g.token != "" && g.token != expired {
			token := g.token
			g.mu.Unlock()
			return token, nil
		}
		if g.authDone == nil {
			g.authDone = make(chan struct{})
			g.mu.Unlock()
			break
		}
		wait := g.authDone
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			return "", ports.ErrShippingUnavailable
		case <-wait:
		}
	}
	defer func() {
		g.mu.Lock()
		close(g.authDone)
		g.authDone = nil
		g.mu.Unlock()
	}()
	var response struct {
		Token string `json:"token"`
	}
	credentials := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{g.email, g.password}
	status, err := g.request(ctx, http.MethodPost, "/auth/login", "", credentials, &response)
	if err != nil && status != 0 {
		return "", fmt.Errorf("%w: authentication HTTP %d", ports.ErrShippingUnavailable, status)
	}
	if err != nil || response.Token == "" {
		return "", ports.ErrShippingUnavailable
	}
	g.mu.Lock()
	g.token = response.Token
	g.mu.Unlock()
	return g.token, nil
}

func (g *Gateway) call(ctx context.Context, method, path string, input, output any) error {
	token, err := g.authenticate(ctx, "")
	if err != nil {
		return err
	}
	status, err := g.request(ctx, method, path, token, input, output)
	if status != http.StatusUnauthorized {
		return err
	}
	token, err = g.authenticate(ctx, token)
	if err != nil {
		return err
	}
	_, err = g.request(ctx, method, path, token, input, output)
	return err
}

func (g *Gateway) request(ctx context.Context, method, path, token string, input, output any) (int, error) {
	var data []byte
	if input != nil {
		var err error
		data, err = json.Marshal(input)
		if err != nil {
			return 0, ports.ErrShippingInput
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, g.base+"/v1/external"+path, bytes.NewReader(data))
	if err != nil {
		return 0, ports.ErrShippingInput
	}
	req.Header.Set("Accept", "application/json")
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return 0, ports.ErrShippingPending
	}
	defer resp.Body.Close()
	if resp.StatusCode == 400 || resp.StatusCode == 422 {
		if (path == "/orders/create/adhoc" || path == "/shipments/create/return-shipment") && resp.StatusCode == 422 {
			// Shiprocket also uses 422 for an existing channel order reference.
			return resp.StatusCode, ports.ErrShippingPending
		}
		return resp.StatusCode, ports.ErrShippingRejected
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return resp.StatusCode, ports.ErrShippingUnavailable
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, ports.ErrShippingPending
	}
	const limit = 2 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || len(body) > limit || json.Unmarshal(body, output) != nil {
		return resp.StatusCode, ports.ErrShippingPending
	}
	return resp.StatusCode, nil
}
