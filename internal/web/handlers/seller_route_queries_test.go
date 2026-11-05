package handlers

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type sellerQueryCounter struct{ calls int }

func (c *sellerQueryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.calls++
	return ctx
}
func (c *sellerQueryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestSellerRoutesLoadOnlyTheirWorkspaceData(t *testing.T) {
	base := projectionTestPool(t)
	_, err := base.Exec(t.Context(), `INSERT INTO users(id,email,display_name,status) VALUES(930001,'route@example.invalid','Route Seller','active'); INSERT INTO sellers(id,owner_user_id,display_name,status) VALUES(930001,930001,'Route Seller','active')`)
	if err != nil {
		t.Fatal(err)
	}
	counter := &sellerQueryCounter{}
	config := base.Config()
	config.ConnConfig.Tracer = counter
	pool, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	h := NewLivePortalHandler(pool, nil, nil)
	for _, tc := range []struct {
		path       string
		maxQueries int
	}{
		{"/seller-admin/settings", 3}, {"/seller-admin/terms", 1},
		{"/seller-admin/products", 4}, {"/seller-admin/returns", 4},
	} {
		counter.calls = 0
		p := h.sellerPage(httptest.NewRequest("GET", tc.path, nil), 930001, true, nil)
		if p.DataWarning != "" {
			t.Fatalf("%s: %s", tc.path, p.DataWarning)
		}
		t.Logf("%s: %d SQL queries", tc.path, counter.calls)
		if counter.calls > tc.maxQueries {
			t.Errorf("%s performs %d queries; expected at most %d", tc.path, counter.calls, tc.maxQueries)
		}
		if p.Orders != nil || p.Inventory != nil || p.TopProducts != nil || p.Ledger != nil || p.Settlements != nil {
			t.Errorf("%s loaded unrelated workspace datasets", tc.path)
		}
	}
}
