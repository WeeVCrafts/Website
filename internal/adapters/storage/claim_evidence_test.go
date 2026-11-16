package storage

import (
	"strings"
	"testing"
)

func TestClaimEvidenceStorageIsPrivateAndUsesSeparateSizeLimits(t *testing.T) {
	key := "private/claim-evidence/" + strings.Repeat("a", 48)
	if !validStoredObjectKey(key) || validProductObjectKey(key) {
		t.Fatal("claim key is not private")
	}
	for _, bad := range []string{key + "/extra", key + ".mp4", "private/claim-evidence/../file", "private/claim-evidence/" + strings.Repeat("a", 32)} {
		if validStoredObjectKey(bad) {
			t.Fatal("invalid claim key accepted")
		}
	}
	for _, kind := range []string{"video/mp4", "video/quicktime", "video/webm"} {
		if !allowedStoredObjectType(key, kind, 200<<20) || allowedStoredObjectType(key, kind, (200<<20)+1) {
			t.Fatal("video limit wrong")
		}
	}
	if !allowedStoredObjectType(key, "image/png", 10<<20) || allowedStoredObjectType(key, "image/png", (10<<20)+1) || allowedStoredObjectType(key, "text/html", 20) || allowedStoredObjectType(key, "application/pdf", 20) {
		t.Fatal("private claim content policy wrong")
	}
	if allowedStoredObjectType("products/fixture", "video/quicktime", 30<<20) {
		t.Fatal("claim video rules escaped namespace")
	}
}
