package storage

import "testing"

func TestStorefrontMediaKeysPermitOnlyOwnedJPEGVariants(t *testing.T) {
	for _, key := range []string{
		"storefront/item-0123456789abcdef01234567.jpg",
		"storefront/item-0123456789abcdef01234567-480.jpg",
		"storefront/item-0123456789abcdef01234567-960.jpg",
	} {
		if !validStoredObjectKey(key) {
			t.Errorf("valid storefront media rejected: %s", key)
		}
	}
	for _, key := range []string{
		"storefront/../private.jpg",
		"storefront/item-0123456789abcdef01234567.svg",
		"storefront/item-0123456789abcdef01234567-200.jpg",
		"storefront/item-0123456789abcdef01234567.jpg/extra",
		"storefront/item-short.jpg",
	} {
		if validStoredObjectKey(key) {
			t.Errorf("unsafe storefront media accepted: %s", key)
		}
	}
}
