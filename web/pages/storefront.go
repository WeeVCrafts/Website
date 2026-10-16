package pages

import (
	"fmt"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"html"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

func storefrontThemeStyle(p viewmodels.CustomerPage) string {
	if p.Storefront == nil {
		return ""
	}
	t := p.Storefront.Theme
	// Font names come from the validated allowlist. Unquoted CSS family names
	// avoid double entity escaping in templ's style-attribute sanitizer.
	return fmt.Sprintf("--brand-primary:%s;--forest:%s;--heading:%s;--brand-terracotta:%s;--terracotta:%s;--brand-surface:%s;--surface:%s;--paper:%s;--sans:%s,Arial,sans-serif;--serif:%s,Georgia,serif", t.Primary, t.Primary, t.Primary, t.Accent, t.Accent, t.Background, t.Background, t.Background, t.BodyFont, t.HeadingFont)
}
func storefrontPreviewID(p viewmodels.CustomerPage) string {
	if p.StorefrontPreview {
		return fmt.Sprint(p.StorefrontRevision)
	}
	return ""
}
func storefrontSectionStyle(s storefront.Section) string {
	r := s.Responsive
	desktop := r.DesktopColumns
	if desktop == 0 {
		desktop = 6
	}
	tablet := r.TabletColumns
	if tablet == 0 {
		tablet = 3
	}
	mobile := r.MobileColumns
	if mobile == 0 {
		mobile = 2
	}
	return fmt.Sprintf("--sf-columns:%d;--sf-tablet-columns:%d;--sf-mobile-columns:%d;--sf-spacing:%dpx;--sf-mobile-spacing:%dpx", desktop, tablet, mobile, r.Spacing, r.MobileSpacing)
}
func storefrontProductURL(p viewmodels.CustomerPage, s storefront.Section) string {
	if s.Button.URL != "" {
		return s.Button.URL
	}
	q := url.Values{}
	category := s.Selection.Category
	if category == "" && s.Selection.Mode == "category" {
		category = p.HomeSelectedCategory
	}
	if category != "" {
		q.Set("category", category)
	}
	if s.Selection.Mode == "newest" {
		q.Set("sort", "newest")
	}
	if s.Selection.Available {
		q.Set("availability", "in-stock")
	}
	if s.Selection.MaxPrice == 149999 {
		q.Set("price", "under-1500")
	}
	q.Set("storefront_section", s.ID)
	if p.Route != "/" {
		if p.StorefrontPage != nil && slices.ContainsFunc(p.StorefrontPage.Sections, func(v storefront.Section) bool { return v.ID == s.ID }) {
			q.Set("storefront_page", p.StorefrontPage.Slug)
		} else if p.StorefrontTemplate.ID != "" {
			q.Set("storefront_template", p.StorefrontTemplate.ID)
		}
	}
	return "/products?" + q.Encode()
}

func storefrontCategoryURL(p viewmodels.CustomerPage, sectionID, category string) string {
	q := url.Values{}
	for key, values := range p.Filters {
		q[key] = append([]string(nil), values...)
	}
	q.Set("home_category", category)
	return p.Route + "?" + q.Encode() + "#section-" + sectionID
}
func contentParagraphs(text string) []string {
	return strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
}

func storefrontFeatureValue(p viewmodels.CustomerPage, name string) string {
	if p.Storefront == nil {
		return "true"
	}
	f := p.Storefront.Settings.Features
	enabled := true
	switch name {
	case "wishlist":
		enabled = f.Wishlist
	case "quick-view":
		enabled = f.QuickView
	case "reviews":
		enabled = f.Reviews
	case "coupons":
		enabled = f.Coupons
	}
	return fmt.Sprint(enabled)
}

func storefrontImagePosition(p viewmodels.CustomerPage, src string) string {
	if value := p.MediaPositions[src]; value != "" {
		return "object-position:" + value
	}
	return ""
}
func storefrontImageAlt(p viewmodels.CustomerPage, src, alt string) string {
	if alt != "" {
		return alt
	}
	return p.MediaAlt[src]
}

func galleryStorefrontLogo(p []viewmodels.CustomerPage) string {
	if len(p) > 0 && p[0].Storefront != nil && p[0].Storefront.Theme.Logo != "" {
		return p[0].Storefront.Theme.Logo
	}
	return "/assets/images/brand/weevcrafts-logo.png"
}

func storefrontActionLabel(labels []string) string {
	if len(labels) > 0 && labels[0] != "" {
		return labels[0]
	}
	return "View all"
}

type storefrontTextSpan struct{ Kind, Text, URL string }

var storefrontInline = regexp.MustCompile(`\*\*([^*]+)\*\*|\*([^*]+)\*|\[([^\]]+)\]\(([^)]+)\)`)

func storefrontTextSpans(text string) []storefrontTextSpan {
	var spans []storefrontTextSpan
	offset := 0
	for _, match := range storefrontInline.FindAllStringSubmatchIndex(text, -1) {
		if match[0] > offset {
			spans = append(spans, storefrontTextSpan{Text: text[offset:match[0]]})
		}
		span := storefrontTextSpan{Text: text[match[0]:match[1]]}
		if match[2] >= 0 {
			span.Kind = "strong"
			span.Text = text[match[2]:match[3]]
		} else if match[4] >= 0 {
			span.Kind = "em"
			span.Text = text[match[4]:match[5]]
		} else {
			href := text[match[8]:match[9]]
			if storefront.SafeURL(href) {
				span.Kind = "link"
				span.Text = text[match[6]:match[7]]
				span.URL = href
			}
		}
		spans = append(spans, span)
		offset = match[1]
	}
	if offset < len(text) {
		spans = append(spans, storefrontTextSpan{Text: text[offset:]})
	}
	return spans
}

func storefrontResponsivePosition(p viewmodels.CustomerPage, desktop, mobile string) string {
	d, m := p.MediaPositions[desktop], p.MediaPositions[mobile]
	if d == "" && m == "" {
		return ""
	}
	if d == "" {
		d = "50% 50%"
	}
	if m == "" {
		m = d
	}
	return "--sf-media-desktop:" + d + ";--sf-media-mobile:" + m + ";object-position:var(--sf-media-current," + d + ")"
}

func storefrontCollectionImage(image string) string {
	if strings.HasPrefix(image, "/") || strings.HasPrefix(image, "https://") {
		return image
	}
	return "/assets/images/customer/" + image
}

func storefrontTemplateImage(p viewmodels.CustomerPage, fallback string) string {
	if p.StorefrontTemplate.Image != "" {
		return p.StorefrontTemplate.Image
	}
	return fallback
}
func storefrontNativeHeading(p viewmodels.CustomerPage, fallback string) string {
	if p.StorefrontTemplate.Title != "" {
		return p.StorefrontTemplate.Title
	}
	return html.UnescapeString(fallback)
}
func storefrontNativeDescription(p viewmodels.CustomerPage, fallback string) string {
	if p.StorefrontTemplate.Description != "" {
		return p.StorefrontTemplate.Description
	}
	return html.UnescapeString(fallback)
}
