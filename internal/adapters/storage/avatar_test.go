package storage

import "testing"

func TestAvatarsArePrivateAndDoNotUseProductPresigning(t *testing.T) {
	key := "avatars/123/0123456789abcdef0123456789abcdef"
	if !validStoredObjectKey(key) || validProductObjectKey(key) {
		t.Fatal("avatar key must be stored privately and never product-presigned")
	}
	for _, value := range []string{"avatars/0/0123456789abcdef0123456789abcdef", "avatars/123/../../products/file", "avatars/123/invalid", "/avatars/123/0123456789abcdef0123456789abcdef"} {
		if validStoredObjectKey(value) {
			t.Fatal("unsafe avatar key accepted")
		}
	}
}
