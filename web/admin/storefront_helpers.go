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
