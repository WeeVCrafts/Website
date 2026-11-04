package components

import (
	"strings"
	"testing"
)

func TestUploadedProductPhotographsHaveResponsiveDelivery(t *testing.T) {
	src := "/media/products/17"
	got := ResponsiveImageSrcSet(src, "")
	for _, part := range []string{"v=2&w=320 320w", "v=2&w=640 640w", "v=2&w=960 960w"} {
		if !strings.Contains(got, part) {
			t.Fatalf("missing %s in %s", part, got)
		}
	}
	for _, src := range []string{"/media/products/17/preview", "https://private.example/photo", "/media/products/not-an-id"} {
		if ResponsiveImageSrcSet(src, "") != "" || MediaDeliveryURL(src) != src {
			t.Fatalf("private/custom media changed: %s", src)
		}
	}
}

func TestStorefrontDeliveryURLsPreserveQueriesAndInvalidateOldFailures(t *testing.T) {
	src := "/media/storefront/item-0123456789abcdef01234567?existing=yes"
	got := ResponsiveImageSrcSet(src, "")
	for _, part := range []string{"existing=yes&v=2&w=480 480w", "existing=yes&v=2&w=960 960w", "existing=yes&v=2 1920w"} {
		if !strings.Contains(got, part) {
			t.Fatalf("missing %q in %q", part, got)
		}
	}
	if MediaDeliveryURL("/media/private/photo?signature=secret") != "/media/private/photo?signature=secret" {
		t.Fatal("private media changed")
	}
	if MediaDeliveryURL(MediaDeliveryURL(src)) != MediaDeliveryURL(src) {
		t.Fatal("versioning must be stable")
	}
}

func TestResponsiveImageSrcSetUpgradesMaintainedLegacyCandidates(t *testing.T) {
	src := "/assets/images/customer/blue-tote.webp"
	legacy := "/assets/images/customer/blue-tote-480.webp 480w, " + src + " 1254w"
	for _, existing := range []string{"", src, legacy, strings.ReplaceAll(legacy, ".webp", ".webp?v=old")} {
		got := ResponsiveImageSrcSet(src+"?v=source", existing)
		if !strings.Contains(got, "responsive/blue-tote-320.webp?v=") || !strings.Contains(got, " 960w") {
			t.Errorf("maintained candidates not upgraded: %q", got)
		}
	}
}

func TestResponsiveImageSrcSetPreservesCustomMedia(t *testing.T) {
	for _, test := range []struct{ src, existing string }{
		{"/media/private/photo", "/media/private/small?signature=opaque 320w, /media/private/large 960w"},
		{"/assets/images/customer/blue-tote.webp", "https://media.example.test/photo?signature=opaque 480w"},
		{"/assets/images/customer/blue-tote.webp", "/assets/images/customer/blue-tote-480.webp 480w, /media/custom/photo 960w"},
		{"/assets/images/customer/blue-tote.webp", "/assets/images/customer/blue-tote-480.webp"},
	} {
		if got := ResponsiveImageSrcSet(test.src, test.existing); got != test.existing {
			t.Errorf("custom candidates changed: got %q, want %q", got, test.existing)
		}
	}
}

func TestRatingStarsRoundsWithinFiveStarRange(t *testing.T) {
	tests := []struct {
		rating string
		want   int
	}{
		{rating: "4.8", want: 5},
		{rating: "4.3", want: 4},
		{rating: "0", want: 0},
		{rating: "not-a-rating", want: 0},
		{rating: "6", want: 5},
	}
	for _, test := range tests {
		if got := ratingStars(test.rating); got != test.want {
			t.Errorf("ratingStars(%q) = %d, want %d", test.rating, got, test.want)
		}
	}
}
