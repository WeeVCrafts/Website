package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	adminpages "github.com/wecratfs/commerce/web/admin"
)

func editorID() string {
	b := make([]byte, 12)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return "item-" + hex.EncodeToString(b)
}
func formInt(r *http.Request, key string) int { return int(formInt64(r, key)) }
func formInt64(r *http.Request, key string) int64 {
	n, _ := strconv.ParseInt(r.FormValue(key), 10, 64)
	return n
}
func formBool(r *http.Request, key string) bool { return r.FormValue(key) == "true" }
func formLink(r *http.Request, prefix string) storefront.Link {
	return storefront.Link{Label: strings.TrimSpace(r.FormValue(prefix + "label")), URL: strings.TrimSpace(r.FormValue(prefix + "url"))}
}
func parseEditorLinks(raw string) ([]storefront.Link, error) {
	result := []storefront.Link{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("Use one link per line: Label | /destination")
		}
		result = append(result, storefront.Link{Label: strings.TrimSpace(parts[0]), URL: strings.TrimSpace(parts[1])})
	}
	return result, nil
}
func editorSection(r *http.Request, old storefront.Section) storefront.Section {
	s := old
	s.Title = r.FormValue("title")
	s.Text = r.FormValue("text")
	s.Order = formInt(r, "order")
	s.Hidden = formBool(r, "hidden")
	s.Image = r.FormValue("image")
	s.MobileImage = r.FormValue("mobile_image")
	s.Alt = r.FormValue("alt")
	s.Button = formLink(r, "button_")
	s.Style = r.FormValue("style")
	s.Selection = storefront.Selection{Mode: r.FormValue("mode"), Category: r.FormValue("category"), Limit: formInt(r, "limit"), Available: formBool(r, "available"), MinPrice: formInt64(r, "min_price"), MaxPrice: formInt64(r, "max_price")}
	for _, raw := range strings.FieldsFunc(r.FormValue("products"), func(c rune) bool { return c == ',' || c == ' ' || c == '\n' }) {
		id, e := strconv.ParseInt(raw, 10, 64)
		if e == nil && id > 0 {
			s.Selection.Products = append(s.Selection.Products, id)
		}
	}
	for _, raw := range r.Form["product_id"] {
		id, e := strconv.ParseInt(raw, 10, 64)
		if e == nil && id > 0 && !slices.Contains(s.Selection.Products, id) {
			s.Selection.Products = append(s.Selection.Products, id)
		}
	}
	s.Responsive = storefront.Responsive{DesktopColumns: formInt(r, "desktop_columns"), TabletColumns: formInt(r, "tablet_columns"), MobileColumns: formInt(r, "mobile_columns"), Spacing: formInt(r, "spacing"), MobileSpacing: formInt(r, "mobile_spacing"), HideDesktop: formBool(r, "hide_desktop"), HideTablet: formBool(r, "hide_tablet"), HideMobile: formBool(r, "hide_mobile")}
	// Structured rows are plain text; no HTML is accepted or rendered.
	s.Items = nil
	for _, line := range strings.Split(r.FormValue("items"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		v := strings.SplitN(line, "|", 5)
		for len(v) < 5 {
			v = append(v, "")
		}
		for i := range v {
			v[i] = strings.TrimSpace(v[i])
		}
		s.Items = append(s.Items, storefront.Tile{Title: v[0], Text: v[1], Image: v[2], URL: v[3], Alt: v[4]})
	}
	return s
}
func editorSelection(m *viewmodels.StorefrontEditor) {
	if m.Tab == "template-sections" || m.Tab == "page-sections" {
		parts := strings.SplitN(m.Selected, ":", 2)
		owner := parts[0]
		block := ""
		if len(parts) > 1 {
			block = parts[1]
		}
		for _, t := range m.Draft.Config.Templates {
			if t.ID == owner {
				m.Template = t
				for _, s := range t.Sections {
					if s.ID == block {
						m.Section = s
					}
				}
			}
		}
		for _, p := range m.Draft.Config.Pages {
			if p.Slug == owner {
				m.Page = p
				for _, s := range p.Sections {
					if s.ID == block {
						m.Section = s
					}
				}
			}
		}
	}

	for _, p := range m.Draft.Config.Promotions {
		if p.Key == m.Selected {
			m.Promotion = p
		}
	}
	for _, s := range m.Draft.Config.Homepage {
		if s.ID == m.Selected {
			m.Section = s
		}
	}
	for _, s := range m.Draft.Config.Slides {
		if s.ID == m.Selected {
			m.Slide = s
		}
	}
	for _, s := range m.Draft.Config.Categories {
		if s.Slug == m.Selected {
			m.Category = s
		}
	}
	for _, s := range m.Draft.Config.Templates {
		if s.ID == m.Selected {
			m.Template = s
		}
	}
	for _, s := range m.Draft.Config.Pages {
		if s.Slug == m.Selected {
			m.Page = s
		}
	}
}
func (h *LivePortalHandler) serveStorefrontEditor(w http.ResponseWriter, r *http.Request) bool {
	root := ""
	for _, candidate := range []string{"/admin", "/super-admin"} {
		if r.URL.Path == candidate+"/storefront" || strings.HasPrefix(r.URL.Path, candidate+"/storefront/") {
			root = candidate
			break
		}
	}
	if root == "" {
		return false
	}
	session, ok := h.session(r)
	if !ok {
		if r.Method == "POST" {
			http.Error(w, "Your session expired. Sign in again before saving this draft.", http.StatusUnauthorized)
		} else {
			redirectToLogin(w, r)
		}
		return true
	}
	if !h.hasRole(r, session.UserID, "admin", "super_admin") {
		denyPortalAccess(w, r)
		return true
	}
	super := h.hasRole(r, session.UserID, "super_admin")
	if root == "/super-admin" && !super {
		denyPortalAccess(w, r)
		return true
	}
	role := domainidentity.RoleAdmin
	if super {
		role = domainidentity.RoleSuperAdmin
	}
	if r.Method == http.MethodGet && r.URL.Query().Get("destination") != "" {
		if destination, err := url.Parse(r.URL.Query().Get("destination")); err == nil && !destination.IsAbs() && destination.Host == "" && destination.Path == root+"/storefront" && !strings.ContainsAny(destination.String(), "\\\r\n") {
			http.Redirect(w, r, destination.String(), http.StatusSeeOther)
			return true
		}
		http.Error(w, "Choose a storefront page.", 422)
		return true
	}
	if r.Method == http.MethodGet && strings.TrimPrefix(r.URL.Path, root+"/storefront") == "/library" {
		h.storefrontEditorLibrary(w, r, session.UserID, root)
		return true
	}
	repo := postgres.NewStorefrontRepository(h.pool)
	m := viewmodels.StorefrontEditor{Tab: r.URL.Query().Get("tab"), Selected: r.URL.Query().Get("section")}
	if m.Tab == "" {
		m.Tab = "homepage"
	}
	m.Admin = viewmodels.AdminPage{ActorID: session.UserID, Active: "storefront", NavActive: "storefront", Title: "Storefront", Subtitle: "Prepare changes, preview every screen and publish after review.", PortalRoot: root, PortalLabel: portalLabel(role), RoleLabel: roleLabel(role), DisplayName: h.userDisplayName(r, session.UserID), CSRFToken: session.CSRFToken, CanSecurity: super, CanManageCatalog: true, Notice: r.URL.Query().Get("notice")}
	if h.paymentMode != "" {
		m.PaymentOptions = append(m.PaymentOptions, h.paymentMode)
	}
	if h.storefrontPreview != nil && h.storefrontPreview.noPaymentDemo {
		m.PaymentOptions = append(m.PaymentOptions, "no-payment")
	}
	action := strings.TrimPrefix(r.URL.Path, root+"/storefront")
	id, _ := strconv.ParseInt(r.URL.Query().Get("draft"), 10, 64)
	status := 200
	var err error
	if action != "" {
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			http.Error(w, "Method not allowed", 405)
			return true
		}
		if action == "/upload" {
			h.storefrontUpload(w, r, session.UserID, session.CSRFToken, root)
			return true
		}
		r.Body = http.MaxBytesReader(w, r.Body, 512<<10)
		var parseErr error
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			parseErr = r.ParseMultipartForm(512 << 10)
			if r.MultipartForm != nil {
				defer r.MultipartForm.RemoveAll()
			}
		} else {
			parseErr = r.ParseForm()
		}
		if parseErr != nil {
			http.Error(w, "This form is too large. Save fewer sections at a time.", 413)
			return true
		}
		token := r.FormValue("csrf_token")
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
			http.Error(w, "Your session changed. Reload before saving.", 403)
			return true
		}
		id = formInt64(r, "draft")
		m.Tab = r.FormValue("tab")
		m.Selected = r.FormValue("section")
		if values := r.Form["section"]; len(values) > 1 {
			m.Selected = values[len(values)-1]
		}
		if action == "/new" {
			id, err = repo.NewDraft(r.Context(), session.UserID, formInt64(r, "restore"))
		} else {
			m.Draft, err = repo.Draft(r.Context(), session.UserID, id)
			if err == nil {
				if action == "/render-preview" {
					h.renderProposedStorefront(w, r, m)
					return true
				} else if action == "/save" {
					err = applyEditorForm(r, &m)
					if err == nil && m.Tab == "settings" && m.Draft.Config.Settings.Policy.RestrictPayments {
						for _, mode := range m.Draft.Config.Settings.Policy.Payments {
							if !slices.Contains(m.PaymentOptions, mode) {
								err = fmt.Errorf("Choose only payment options currently configured for this application")
								break
							}
						}
					}
					if err == nil {
						err = repo.Save(r.Context(), session.UserID, id, formInt64(r, "version"), m.Draft.Config)
					}
				} else if slices.Contains([]string{"/submit", "/request-changes", "/publish"}, action) {
					err = repo.Transition(r.Context(), session.UserID, id, formInt64(r, "version"), strings.TrimPrefix(action, "/"), strings.TrimSpace(r.FormValue("note")))
				} else {
					http.NotFound(w, r)
					return true
				}
			}
		}
		if err == nil {
			notice := "Draft saved. Customers still see the published storefront."
			if action == "/submit" {
				notice = "Submitted for Super Admin review."
			}
			if action == "/publish" {
				notice = "Published. Customers now see this release."
			}
			if action == "/request-changes" {
				notice = "Changes requested. The draft is editable again."
			}
			if action == "/save" && r.FormValue("return_to") != "" {
				if target, e := url.Parse(r.FormValue("return_to")); e == nil && !target.IsAbs() && target.Host == "" && target.Path == root+"/storefront" && target.Query().Get("draft") == fmt.Sprint(id) {
					q := target.Query()
					q.Set("notice", notice)
					target.RawQuery = q.Encode()
					http.Redirect(w, r, target.String(), 303)
					return true
				}
			}
			q := url.Values{"draft": {fmt.Sprint(id)}, "tab": {m.Tab}, "section": {m.Selected}, "notice": {notice}}
			http.Redirect(w, r, root+"/storefront?"+q.Encode(), 303)
			return true
		}
		m.Error = err.Error()
		if errors.Is(err, storefront.ErrConflict) {
			m.Draft.Version = formInt64(r, "version")
		}
		status = 422
		if errors.Is(err, storefront.ErrConflict) {
			status = 409
		}
		if errors.Is(err, ports.ErrForbidden) {
			status = 403
		}
	} else if r.Method != "GET" {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return true
	}
	if m.Draft.ID == 0 && id > 0 {
		m.Draft, err = repo.Draft(r.Context(), session.UserID, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.NotFound(w, r)
			} else {
				http.Error(w, "Draft unavailable. Reload or return to Publishing.", 503)
			}
			return true
		}
	}
	m.HistoryPage, _ = strconv.Atoi(r.URL.Query().Get("history_page"))
	if m.HistoryPage < 1 {
		m.HistoryPage = 1
	}
	m.Drafts, m.Releases, err = repo.ListAtPage(r.Context(), session.UserID, m.HistoryPage)
	if err != nil {
		http.Error(w, "Publishing history could not be loaded. Try again.", 503)
		return true
	}
	if id == 0 {
		m.Tab = "overview"
	}
	if m.Draft.ID > 0 && (m.Tab == "publishing" || strings.Contains(m.Error, "changed")) {
		live, e := postgres.ReadPublishedStorefront(r.Context(), h.pool)
		if e == nil {
			m.Differences = storefrontDifferences(live.Config, m.Draft.Config)
		}
	}
	editorSelection(&m)
	storefrontGuidance(&m)
	if len(m.Releases) > 0 {
		m.Published = m.Releases[0]
	}
	if live, e := postgres.ReadPublishedStorefront(r.Context(), h.pool); e == nil {
		if m.Published.ID != live.ID {
			m.Published = storefront.Release{ID: live.ID}
		}
	}
	h.storefrontPreviewTarget(r, &m)
	if m.Section.Type == "products" || (m.Tab == "promotions" && m.Promotion.Key != "") {
		if e := h.storefrontProductPicker(r, &m); e != nil {
			m.Error = "The product picker could not be loaded. Reload before saving selections."
			status = 503
		}
	}
	if m.Tab == "media" {
		m.Media, err = h.storefrontMediaList(r, &m)
		if err != nil {
			m.Error = "The media library could not be loaded. Reload to try again."
			status = 503
		}
	}
	if m.Tab == "settings" && r.URL.Query().Get("subtotal") != "" {
		m.Simulation = h.simulateStorefront(r, m.Draft.Config)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	h.renderAdminComponent(w, r, adminpages.Storefront(m), m.Admin)
	return true
}
func applyEditorForm(r *http.Request, m *viewmodels.StorefrontEditor) error {
	return applyEditorFormValues(r, m, false)
}
func applyEditorFormValues(r *http.Request, m *viewmodels.StorefrontEditor, moneyNormalized bool) error {
	if !moneyNormalized {
		if err := editorStructuredRows(r); err != nil {
			return err
		}
		moneyFields := []string{}
		if m.Tab == "settings" {
			moneyFields = []string{"shipping_cents", "free_shipping_cents"}
		} else if m.Tab == "promotions" && r.FormValue("kind") == "fixed" {
			moneyFields = []string{"value"}
		} else if r.FormValue("type") != "" || m.Tab == "homepage" || m.Tab == "page-sections" || m.Tab == "template-sections" {
			for _, key := range []string{"min_price", "max_price"} {
				if _, present := r.Form[key]; present {
					moneyFields = append(moneyFields, key)
				}
			}
		}
		for _, key := range moneyFields {
			cents, e := parseStorefrontMoney(r.FormValue(key))
			if e != nil {
				return e
			}
			r.Form.Set(key, strconv.FormatInt(cents, 10))
		}
	}
	for _, key := range []string{"order", "limit", "min_price", "max_price", "desktop_columns", "tablet_columns", "mobile_columns", "spacing", "mobile_spacing", "shipping_cents", "free_shipping_cents", "return_days", "value", "usage_limit", "coupon_limit"} {
		if raw := r.FormValue(key); raw != "" {
			if _, e := strconv.ParseInt(raw, 10, 64); e != nil {
				return fmt.Errorf("%w: %s must be a whole number", storefront.ErrValidation, key)
			}
		}
	}
	c := &m.Draft.Config
	op := r.FormValue("operation")
	switch m.Tab {
	case "template-sections", "page-sections":
		parts := strings.SplitN(m.Selected, ":", 2)
		owner := parts[0]
		block := ""
		if len(parts) > 1 {
			block = parts[1]
		}
		originalTab := m.Tab
		originalHome := c.Homepage
		var target *[]storefront.Section
		if originalTab == "template-sections" {
			for i := range c.Templates {
				if c.Templates[i].ID == owner {
					target = &c.Templates[i].Sections
					break
				}
			}
		} else {
			for i := range c.Pages {
				if c.Pages[i].Slug == owner {
					c.Pages[i].Legacy = false
					target = &c.Pages[i].Sections
					break
				}
			}
		}
		if target == nil {
			return storefront.ErrValidation
		}
		c.Homepage = *target
		m.Tab = "homepage"
		m.Selected = block
		e := applyEditorFormValues(r, m, true)
		updated := c.Homepage
		c.Homepage = originalHome
		m.Tab = originalTab
		m.Selected = owner + ":" + m.Selected
		if e != nil {
			return e
		}
		*target = updated
	case "homepage":
		if op == "reorder" {
			ids := strings.Split(r.FormValue("section_order"), ",")
			if len(ids) != len(c.Homepage) {
				return storefront.ErrValidation
			}
			seen := map[string]bool{}
			for n, id := range ids {
				if seen[id] {
					return storefront.ErrValidation
				}
				seen[id] = true
				found := false
				for i := range c.Homepage {
					if c.Homepage[i].ID == id {
						c.Homepage[i].Order = (n + 1) * 10
						found = true
					}
				}
				if !found {
					return storefront.ErrValidation
				}
			}
			break
		}
		if op == "add" {
			kind := r.FormValue("type")
			s := storefrontPreset(kind, (len(c.Homepage)+1)*10)
			if kind == "edits" {
				s.Type = "collections"
				s.Title = "Explore our collections"
				for _, category := range c.OrderedCategories() {
					if category.Visible && len(s.Items) < 4 {
						s.Items = append(s.Items, storefront.Tile{Title: category.Name, Text: category.Description, Image: category.Image, Alt: category.Name, URL: "/products?category=" + url.QueryEscape(category.Slug)})
					}
				}
			}
			c.Homepage = append(c.Homepage, s)
			m.Selected = s.ID
			break
		}
		for i, s := range c.Homepage {
			if s.ID != m.Selected {
				continue
			}
			switch op {
			case "remove":
				c.Homepage = slices.Delete(c.Homepage, i, i+1)
				m.Selected = ""
			case "duplicate":
				s.ID = editorID()
				s.Order++
				c.Homepage = append(c.Homepage, s)
				m.Selected = s.ID
			case "up", "down":
				ordered := storefront.OrderedSections(c.Homepage)
				idx := slices.IndexFunc(ordered, func(v storefront.Section) bool { return v.ID == s.ID })
				j := idx - 1
				if op == "down" {
					j = idx + 1
				}
				if j >= 0 && j < len(ordered) {
					ordered[idx], ordered[j] = ordered[j], ordered[idx]
					for k := range ordered {
						ordered[k].Order = (k + 1) * 10
					}
					c.Homepage = ordered
				}
			case "hide":
				c.Homepage[i].Hidden = !s.Hidden
			default:
				c.Homepage[i] = editorSection(r, s)
			}
			break
		}
	case "hero":
		if op == "add" {
			s := storefront.Slide{ID: editorID(), Order: (len(c.Slides) + 1) * 10, Title: "New collection", Image: "/assets/images/customer/craft-banner.webp", Alt: "Indian craft collection", Button: storefront.Link{Label: "Explore", URL: "/products"}}
			c.Slides = append(c.Slides, s)
			m.Selected = s.ID
			break
		}
		for i, s := range c.Slides {
			if s.ID != m.Selected {
				continue
			}
			if op == "remove" {
				c.Slides = slices.Delete(c.Slides, i, i+1)
				m.Selected = ""
			} else {
				s.Order = formInt(r, "order")
				s.Hidden = formBool(r, "hidden")
				s.Title = r.FormValue("title")
				s.Text = r.FormValue("text")
				s.Kicker = r.FormValue("kicker")
				s.Image = r.FormValue("image")
				s.MobileImage = r.FormValue("mobile_image")
				s.Alt = r.FormValue("alt")
				s.Button = formLink(r, "button_")
				s.Secondary = formLink(r, "secondary_")
				c.Slides[i] = s
			}
			break
		}
	case "categories":
		if op == "add" {
			s := storefront.Category{Slug: r.FormValue("slug"), Name: r.FormValue("name"), Visible: true, Order: 1000}
			c.Categories = append(c.Categories, s)
			m.Selected = s.Slug
			break
		}
		for i, s := range c.Categories {
			if s.Slug != m.Selected {
				continue
			}
			if op == "remove" {
				c.Categories = slices.Delete(c.Categories, i, i+1)
				m.Selected = ""
			} else {
				s.Name = r.FormValue("name")
				s.Description = r.FormValue("text")
				if s.Image != r.FormValue("image") {
					s.ObjectKey = ""
				}
				s.Image = r.FormValue("image")
				s.Order = formInt(r, "order")
				s.Visible = formBool(r, "visible")
				c.Categories[i] = s
			}
			break
		}
	case "navigation":
		c.Trust = parseEditorTiles(r.FormValue("trust"))
		var policyErr error
		c.PolicyLinks, policyErr = parseEditorLinks(r.FormValue("policy_links"))
		if policyErr != nil {
			return policyErr
		}
		var e error
		c.Navigation, e = parseEditorLinks(r.FormValue("navigation"))
		if e != nil {
			return e
		}
		c.FooterGroups, e = parseEditorFooter(r.FormValue("footer"))
		if e != nil {
			return e
		}
	case "theme":
		c.Theme = storefront.Theme{Logo: r.FormValue("logo"), Primary: r.FormValue("primary"), Accent: r.FormValue("accent"), Background: r.FormValue("background"), BodyFont: r.FormValue("body_font"), HeadingFont: r.FormValue("heading_font")}
	case "pages":
		if op == "add" {
			p := storefront.Page{Slug: r.FormValue("slug"), Title: r.FormValue("title")}
			c.Pages = append(c.Pages, p)
			m.Selected = p.Slug
			break
		}
		for i, p := range c.Pages {
			if p.Slug != m.Selected {
				continue
			}
			p.Legacy = false
			p.Title = r.FormValue("title")
			p.Description = r.FormValue("description")
			p.SharingImage = r.FormValue("image")
			p.Hidden = formBool(r, "hidden")
			if len(p.Sections) == 0 {
				p.Sections = []storefront.Section{{ID: "content", Type: "rich-text", Title: p.Title, Text: r.FormValue("text")}}
			} else {
				p.Sections[0].Text = r.FormValue("text")
			}
			c.Pages[i] = p
			break
		}
	case "templates":
		if op == "add" {
			t := storefront.Template{ID: editorID(), Kind: r.FormValue("kind"), Match: r.FormValue("match")}
			c.Templates = append(c.Templates, t)
			m.Selected = t.ID
			break
		}
		for i, t := range c.Templates {
			if t.ID == m.Selected && op == "remove" {
				if t.Match == "" {
					return fmt.Errorf("%w: default templates remain available; clear their optional sections instead", storefront.ErrValidation)
				}
				c.Templates = slices.Delete(c.Templates, i, i+1)
				m.Selected = ""
				break
			}
			if t.ID != m.Selected {
				continue
			}
			t.Title = r.FormValue("title")
			t.Description = r.FormValue("description")
			t.Image = r.FormValue("image")
			t.Match = r.FormValue("match")
			t.HideDefaultIntro = formBool(r, "hide_default_intro")
			if len(t.Sections) == 0 && r.FormValue("text") != "" {
				t.Sections = []storefront.Section{{ID: "introduction", Type: "banner", Title: "Introduction", Text: r.FormValue("text"), Image: t.Image}}
			} else if len(t.Sections) > 0 && t.Sections[0].Type == "banner" {
				if t.HideDefaultIntro {
					t.Sections[0].Title = t.Title
				}
				t.Sections[0].Text = r.FormValue("text")
				t.Sections[0].Image = t.Image
			}
			c.Templates[i] = t
			break
		}
	case "promotions":
		if op == "add" {
			p := storefront.Promotion{Key: editorID(), Name: "New promotion", Kind: "percentage", Value: 10, Status: "draft", TargetMode: "all", Coupons: []storefront.Coupon{{Code: "NEW-" + strings.ToUpper(editorID()[5:17]), Active: true}}}
			c.Promotions = append(c.Promotions, p)
			m.Selected = p.Key
			break
		}
		for i, p := range c.Promotions {
			if p.Key != m.Selected {
				continue
			}
			p.Name = r.FormValue("name")
			p.Description = r.FormValue("description")
			p.Kind = r.FormValue("kind")
			p.Value = formInt64(r, "value")
			p.Status = r.FormValue("status")
			p.TargetMode = r.FormValue("target_mode")
			p.UsageLimit = formInt64(r, "usage_limit")
			p.StartsAt = nil
			p.EndsAt = nil
			if date := r.FormValue("starts_at"); date != "" {
				v, e := parsePromotionDate(date, false)
				if e != nil {
					return e
				}
				p.StartsAt = v
			}
			if date := r.FormValue("ends_at"); date != "" {
				v, e := parsePromotionDate(date, true)
				if e != nil {
					return e
				}
				p.EndsAt = v
			}
			p.ProductIDs = nil
			for _, raw := range strings.FieldsFunc(r.FormValue("products"), func(c rune) bool { return c == ',' || c == ' ' }) {
				n, e := strconv.ParseInt(raw, 10, 64)
				if e != nil || n <= 0 {
					return storefront.ErrValidation
				}
				p.ProductIDs = append(p.ProductIDs, n)
			}
			for _, raw := range r.Form["product_id"] {
				id, e := strconv.ParseInt(raw, 10, 64)
				if e != nil || id <= 0 {
					return storefront.ErrValidation
				}
				if !slices.Contains(p.ProductIDs, id) {
					p.ProductIDs = append(p.ProductIDs, id)
				}
			}
			if len(p.Coupons) == 0 {
				p.Coupons = []storefront.Coupon{{}}
			}
			p.Coupons[0].Code = strings.ToUpper(strings.TrimSpace(r.FormValue("code")))
			p.Coupons[0].UsageLimit = formInt64(r, "coupon_limit")
			p.Coupons[0].Active = formBool(r, "coupon_active")
			c.Promotions[i] = p
			break
		}
	case "settings":
		c.Settings.SupportEmail = r.FormValue("support_email")
		c.Settings.ShippingInformation = r.FormValue("shipping_information")
		var e error
		c.Settings.Social, e = parseEditorLinks(r.FormValue("social"))
		if e != nil {
			return e
		}
		c.Settings.Policy = storefront.Policy{ShippingCents: formInt64(r, "shipping_cents"), FreeShippingCents: formInt64(r, "free_shipping_cents"), ReturnDays: formInt(r, "return_days"), PINRestricted: formBool(r, "pin_restricted"), PINCodes: strings.FieldsFunc(r.FormValue("pin_codes"), func(c rune) bool { return c == ',' || c == '\n' || c == ' ' }), Payments: r.Form["payments"]}
		c.Settings.Policy.RestrictPayments = formBool(r, "restrict_payments")
		c.Settings.Features = storefront.Features{Wishlist: formBool(r, "wishlist"), QuickView: formBool(r, "quick_view"), Reviews: formBool(r, "reviews"), Coupons: formBool(r, "coupons")}
	default:
		return storefront.ErrValidation
	}
	for i := range c.Pages {
		if c.Pages[i].Slug == "shipping" && len(c.Pages[i].Sections) > 0 {
			if m.Tab == "settings" {
				c.Pages[i].Sections[0].Text = c.Settings.ShippingInformation
			} else if (m.Tab == "pages" && m.Selected == "shipping") || (m.Tab == "page-sections" && strings.HasPrefix(m.Selected, "shipping:")) {
				c.Settings.ShippingInformation = c.Pages[i].Sections[0].Text
			}
		}
	}
	return applyEditorStructuredRows(r, m)
}

func parseEditorFooter(raw string) ([]storefront.LinkGroup, error) {
	groups := []storefront.LinkGroup{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# ") {
			groups = append(groups, storefront.LinkGroup{Title: strings.TrimPrefix(line, "# ")})
			continue
		}
		if len(groups) == 0 {
			return nil, fmt.Errorf("Start footer groups with # Heading")
		}
		i := len(groups) - 1
		if strings.HasPrefix(line, "> ") {
			groups[i].Text = strings.TrimPrefix(line, "> ")
			continue
		}
		links, e := parseEditorLinks(line)
		if e != nil {
			return nil, e
		}
		groups[i].Links = append(groups[i].Links, links...)
	}
	return groups, nil
}

func parseEditorTiles(raw string) []storefront.Tile {
	items := []storefront.Tile{}
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		v := strings.SplitN(line, "|", 5)
		for len(v) < 5 {
			v = append(v, "")
		}
		for i := range v {
			v[i] = strings.TrimSpace(v[i])
		}
		items = append(items, storefront.Tile{Title: v[0], Text: v[1], Image: v[2], URL: v[3], Alt: v[4]})
	}
	return items
}

var storefrontMoneyInput = regexp.MustCompile(`^[0-9]{1,8}(?:\.[0-9]{1,2})?$`)

func parseStorefrontMoney(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if !storefrontMoneyInput.MatchString(raw) {
		return 0, fmt.Errorf("%w: enter an INR amount with at most two decimal places", storefront.ErrValidation)
	}
	parts := strings.SplitN(raw, ".", 2)
	whole, _ := strconv.ParseInt(parts[0], 10, 64)
	fraction := int64(0)
	if len(parts) > 1 {
		digits := parts[1]
		if len(digits) == 1 {
			digits += "0"
		}
		fraction, _ = strconv.ParseInt(digits, 10, 64)
	}
	return whole*100 + fraction, nil
}
