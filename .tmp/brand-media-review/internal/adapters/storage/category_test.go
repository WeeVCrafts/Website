package storage

import "testing"

func TestCategoryMediaUsesScopedServerStorage(t *testing.T) {
	key := "categories/0123456789abcdef0123456789abcdef.jpg"
	if !validStoredObjectKey(key) || validProductObjectKey(key) {
		t.Fatal("category media must use server-proxied storage")
	}
	for _, key := range []string{"categories/../products/file", "categories/invalid.jpg", "categories/0123456789abcdef0123456789abcdef.svg", "/categories/0123456789abcdef0123456789abcdef.jpg"} {
		if validStoredObjectKey(key) {
			t.Fatalf("unsafe category key accepted: %s", key)
		}
	}
}
