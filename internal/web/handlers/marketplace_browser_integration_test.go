package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	redisadapter "github.com/wecratfs/commerce/internal/adapters/redis"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/assets"
	"github.com/wecratfs/commerce/web/pages"
)

// An opt-in authenticated browser surface, with no checkout or provider worker.
func TestMarketplaceBrowserPreview(t *testing.T) {
	file := os.Getenv("MARKETPLACE_PREVIEW_URL_FILE")
	if file == "" {
		t.Skip("opt-in browser preview")
	}
	dsn := os.Getenv("DATABASE_URL")
	if !strings.Contains(dsn, "@127.0.0.1:15465/marketplace_policy_test") {
		t.Fatal("dedicated disposable browser database required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	client := redisclient.NewClient(&redisclient.Options{Addr: "127.0.0.1:15464"})
	defer client.Close()
	sessions := applicationauth.NewSessionService(redisadapter.NewSessionStore(client)).WithSessionChecker(postgres.NewUserRepository(pool))
	repo := postgres.NewMarketplaceRepository(pool).WithInvoiceRenderer(MarketplaceInvoicePDF)
	var fixtures struct{ Order, PolicyOrder string }
	data, err := os.ReadFile(os.Getenv("MARKETPLACE_UI_FIXTURE_FILE"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, `SELECT id FROM orders WHERE workflow_version=2 AND status='paid' AND order_number=ANY($1) ORDER BY id`, []string{fixtures.Order, fixtures.PolicyOrder})
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err = repo.IssueInvoices(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	fallback := http.NewServeMux()
	fallback.HandleFunc("/returns-replacements-authenticity", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := pages.ReturnPolicy(viewmodels.CustomerPage{Route: r.URL.Path, Brand: "WeevCrafts", ReturnPolicyEffective: "16 November 2026", ReturnPolicySections: domain.ReturnPolicySections()}).Render(r.Context(), w); err != nil {
			t.Error(err)
		}
	})
	fallback.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets.FS))))
	handler := (&MarketplaceHandler{Repository: repo, Sessions: sessions, ShippingConfigured: true, PickupChecker: &setupBrowserPickups{}, Storage: &claimBrowserStorage{objects: map[string][]byte{}}}).Middleware(fallback)
	server := httptest.NewServer(handler)
	defer server.Close()
	if err = os.WriteFile(file, []byte(server.URL), 0600); err != nil {
		t.Fatal(err)
	}
	timer := time.NewTimer(10 * time.Minute)
	defer timer.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-timer.C:
			return
		case <-ticker.C:
			if _, err = os.Stat(file + ".stop"); err == nil {
				return
			}
		}
	}
}

type setupBrowserPickups struct {
	mu    sync.Mutex
	calls int
}

func (p *setupBrowserPickups) PickupPostcodes(context.Context) (map[string]string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.calls == 1 {
		return nil, ports.ErrShippingUnavailable
	}
	return map[string]string{"Fixture Depot": "411001", "Second Verified Depot": "500079"}, nil
}

// Isolated bytes exercise the real evidence handler without cloud credentials.
type claimBrowserStorage struct {
	ports.ObjectStorage
	mu      sync.Mutex
	objects map[string][]byte
}

func (s *claimBrowserStorage) PutObject(_ context.Context, key, kind string, size int64, body io.Reader) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = b
	return nil
}
func (s *claimBrowserStorage) OpenObject(_ context.Context, key string) (io.ReadCloser, ports.ObjectInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.objects[key]
	if !ok {
		return nil, ports.ObjectInfo{}, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(b)), ports.ObjectInfo{ByteSize: int64(len(b))}, nil
}
