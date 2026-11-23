package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

type mockStorefrontQueryer struct {
	calls  int
	id     int64
	config []byte
}

type mockStorefrontRow struct {
	id     int64
	config []byte
}

func (r *mockStorefrontRow) Scan(dest ...any) error {
	if len(dest) >= 2 {
		if idPtr, ok := dest[0].(*int64); ok {
			*idPtr = r.id
		}
		if bytePtr, ok := dest[1].(*[]byte); ok {
			*bytePtr = r.config
		}
	}
	return nil
}

func (m *mockStorefrontQueryer) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.calls++
	return &mockStorefrontRow{id: m.id, config: m.config}
}

func TestReadPublishedStorefront_MemoryCacheAndInvalidation(t *testing.T) {
	InvalidatePublishedStorefrontCache()

	queryer := &mockStorefrontQueryer{
		id:     101,
		config: []byte(`{"version":1,"settings":{"features":{"wishlist":true}}}`),
	}

	ctx := context.Background()

	// First call should query the database and populate the cache.
	rev1, err := ReadPublishedStorefront(ctx, queryer)
	if err != nil {
		t.Fatalf("first call unexpected error: %v", err)
	}
	if rev1.ID != 101 {
		t.Fatalf("expected rev1.ID = 101, got %d", rev1.ID)
	}
	if queryer.calls != 1 {
		t.Fatalf("expected queryer.calls = 1, got %d", queryer.calls)
	}

	// Second call should hit the memory cache (queryer.calls remains 1).
	rev2, err := ReadPublishedStorefront(ctx, queryer)
	if err != nil {
		t.Fatalf("second call unexpected error: %v", err)
	}
	if rev2.ID != 101 {
		t.Fatalf("expected rev2.ID = 101, got %d", rev2.ID)
	}
	if queryer.calls != 1 {
		t.Fatalf("expected queryer.calls to remain 1 due to cache, got %d", queryer.calls)
	}

	// Invalidate cache and verify the next call queries the database again.
	InvalidatePublishedStorefrontCache()
	queryer.id = 102
	queryer.config = []byte(`{"version":1,"settings":{"features":{"wishlist":false}}}`)

	rev3, err := ReadPublishedStorefront(ctx, queryer)
	if err != nil {
		t.Fatalf("third call unexpected error: %v", err)
	}
	if rev3.ID != 102 {
		t.Fatalf("expected rev3.ID = 102, got %d", rev3.ID)
	}
	if queryer.calls != 2 {
		t.Fatalf("expected queryer.calls = 2 after invalidation, got %d", queryer.calls)
	}
}
