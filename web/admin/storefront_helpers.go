package admin

import (
	"fmt"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"net/url"
	"strings"
	"time"
)

func editorURL(m viewmodels.StorefrontEditor, tab, id string) string {
	return m.Admin.PortalRoot + "/storefront?" + url.Values{"draft": {fmt.Sprint(m.Draft.ID)}, "tab": {tab}, "section": {id}}.Encode()
}
func editorLinks(links []storefront.Link) string {
	var lines []string
	for _, l := range links {
		lines = append(lines, l.Label+" | "+l.URL)
	}
	return strings.Join(lines, "\n")
}
func editorItems(items []storefront.Tile) string {
	var lines []string
	for _, l := range items {
		lines = append(lines, l.Title+" | "+l.Text+" | "+l.Image+" | "+l.URL+" | "+l.Alt)
	}
	return strings.Join(lines, "\n")
}
func editorProducts(ids []int64) string {
	var result []string
	for _, id := range ids {
		result = append(result, fmt.Sprint(id))
	}
	return strings.Join(result, ", ")
}
func editorPageText(p storefront.Page) string {
	if len(p.Sections) > 0 {
		return p.Sections[0].Text
	}
	return ""
}
func editorTemplateText(p storefront.Template) string {
	if len(p.Sections) > 0 {
		return p.Sections[0].Text
	}
	return ""
}

func contains(values []string, s string) bool {
	for _, v := range values {
		if v == s {
			return true
		}
	}
	return false
}

func editorDate(t *time.Time, end bool) string {
	if t == nil {
		return ""
	}
	v := t.In(time.FixedZone("IST", 19800))
	if end {
		v = v.AddDate(0, 0, -1)
	}
	return v.Format("2006-01-02")
}
func editorCoupon(p storefront.Promotion) storefront.Coupon {
	if len(p.Coupons) > 0 {
		return p.Coupons[0]
	}
	return storefront.Coupon{}
}

func editorSections(m viewmodels.StorefrontEditor) []storefront.Section {
	if m.Tab == "template-sections" {
		return m.Template.Sections
	}
	if m.Tab == "page-sections" {
		return m.Page.Sections
	}
	return m.Draft.Config.Homepage
}
func editorSectionID(m viewmodels.StorefrontEditor, id string) string {
	if m.Tab == "template-sections" || m.Tab == "page-sections" {
		return strings.SplitN(m.Selected, ":", 2)[0] + ":" + id
	}
	return id
}

func editorFooter(groups []storefront.LinkGroup) string {
	var lines []string
	for _, g := range groups {
		lines = append(lines, "# "+g.Title)
		if g.Text != "" {
			lines = append(lines, "> "+g.Text)
		}
		lines = append(lines, editorLinks(g.Links), "")
	}
	return strings.Join(lines, "\n")
}

func editorINR(cents int64) string { return fmt.Sprintf("%d.%02d", cents/100, cents%100) }
func editorPromotionValue(p storefront.Promotion) string {
	if p.Kind == "fixed" {
		return editorINR(p.Value)
	}
	return fmt.Sprint(p.Value)
}

func editorGroup(tab string) string {
	switch tab {
	case "navigation", "theme", "media":
		return "brand"
	case "pages", "page-sections":
		return "content"
	case "settings", "promotions":
		return "settings"
	case "publishing":
		return "publishing"
	default:
		return "design"
	}
}
func editorSectionName(s storefront.Section) string {
	if s.Title != "" {
		return s.Title
	}
	return editorKindName(s.Type)
}
func editorKindName(kind string) string {
	names := map[string]string{"hero": "Hero slideshow", "categories": "Shop by category", "products": "Product collection", "makers": "Maker stories", "collections": "Curated collections", "banner": "Promotional banner", "image-text": "Image and story", "rich-text": "Text section", "faq": "Frequently asked questions", "trust": "Shopping information"}
	if name := names[kind]; name != "" {
		return name
	}
	return kind
}
func editorKindHint(kind string) string {
	hints := map[string]string{"hero": "Introduce a collection with photographs and clear shopping links.", "categories": "Help customers discover the craft categories you offer.", "products": "Curate real catalog products or choose an automatic selection.", "makers": "Tell verified maker stories and link to their stores.", "collections": "Guide customers by occasion, craft, or a curated theme.", "faq": "Answer common questions using your own verified information."}
	if hint := hints[kind]; hint != "" {
		return hint
	}
	return "Add content, choose its destination, and check the responsive preview."
}
func editorHasOutline(m viewmodels.StorefrontEditor) bool {
	return m.Tab == "homepage" || m.Tab == "template-sections" || m.Tab == "page-sections"
}
func editorBlocking(m viewmodels.StorefrontEditor) bool {
	for _, f := range m.Readiness {
		if !f.Warning {
			return true
		}
	}
	return false
}
func editorKindIcon(kind string) string {
	switch kind {
	case "hero", "banner":
		return "image"
	case "products":
		return "shopping_bag"
	case "makers":
		return "storefront"
	case "categories", "collections":
		return "category"
	case "faq":
		return "help"
	default:
		return "edit_note"
	}
}
func editorPresetName(kind string) string {
	switch kind {
	case "curated":
		return "Curated collection · selected products"
	case "gifts":
		return "Gift selection · under ₹1,500"
	case "story":
		return "Maker story · verified catalog profiles"
	case "promotion":
		return "Promotional banner · image and destination"
	case "questions":
		return "FAQ · collection questions"
	default:
		return editorKindName(kind)
	}
}
func editorDraftState(state string) string {
	switch state {
	case "submitted":
		return "Submitted for review"
	case "published":
		return "Published"
	default:
		return "Saved draft"
	}
}
func editorAssignment(match string) string {
	if match == "" {
		return "Default page"
	}
	return match
}
func editorDraftDestination(m viewmodels.StorefrontEditor, id int64, state string) string {
	m.Draft.ID = id
	tab := "homepage"
	if state == "submitted" {
		tab = "publishing"
	}
	return editorURL(m, tab, "")
}
func editorContinueLabel(state string) string {
	if state == "submitted" {
		return "Review draft"
	}
	return "Continue editing"
}
func editorFindingLabel(warning bool) string {
	if warning {
		return "Check"
	}
	return "Action needed"
}
func editorGlobalTarget(tab string) string {
	switch tab {
	case "navigation":
		return "global:navigation"
	case "theme":
		return "global:theme"
	default:
		return ""
	}
}

func editorImageValue(value string) bool {
	return strings.HasPrefix(value, "/media/storefront/") || strings.HasPrefix(value, "/assets/images/") || strings.HasSuffix(strings.ToLower(value), ".jpg") || strings.HasSuffix(strings.ToLower(value), ".webp") || strings.HasSuffix(strings.ToLower(value), ".png")
}

func editorStructuredContent(m viewmodels.StorefrontEditor) map[string]any {
	return map[string]any{"navigation": m.Draft.Config.Navigation, "policy_links": m.Draft.Config.PolicyLinks, "social": m.Draft.Config.Settings.Social, "trust": m.Draft.Config.Trust, "footer": m.Draft.Config.FooterGroups, "items": m.Section.Items}
}

func editorPageName(kind string) string {
	switch kind {
	case "category":
		return "Category page"
	case "search":
		return "Search and catalog"
	case "product":
		return "Product page"
	case "maker":
		return "Maker page"
	case "brand":
		return "Brand page"
	case "wishlist":
		return "Wishlist"
	case "cart":
		return "Cart"
	case "checkout":
		return "Checkout"
	case "account":
		return "Customer account"
	case "orders":
		return "Customer orders"
	case "authentication":
		return "Sign-in and registration"
	default:
		return kind
	}
}
