package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

// Proposed previews use the same renderer and authorization as saved previews,
// but never call the draft repository's Save or Transition methods.
func (h *LivePortalHandler) renderProposedStorefront(w http.ResponseWriter, r *http.Request, m viewmodels.StorefrontEditor) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	fail := func(status int, message string) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
	}
	if m.Draft.State != "draft" {
		fail(409, "This draft is locked. Open the latest saved version before editing.")
		return
	}
	if m.Draft.Version != formInt64(r, "version") {
		fail(409, "This draft changed elsewhere. Your edits remain here; compare the latest saved version before saving.")
		return
	}
	if r.FormValue("operation") != "" {
		fail(422, "Save structural changes before previewing their content.")
		return
	}
	if err := applyEditorForm(r, &m); err != nil {
		fail(422, err.Error())
		return
	}
	editorSelection(&m)
	h.storefrontPreviewTarget(r, &m)
	target, _ := url.Parse(m.PreviewURL)
	path, err := h.validStorefrontPreviewPath(target.Query().Get("path"))
	if err != nil {
		fail(422, err.Error())
		return
	}
	clone := r.Clone(storefront.WithRevision(r.Context(), storefront.Revision{ID: m.Draft.ID, Config: m.Draft.Config, Preview: true}))
	clone.Method = http.MethodGet
	clone.Body = nil
	clone.Form, clone.PostForm = nil, nil
	clone.URL = path
	clone.Header = r.Header.Clone()
	clone.Header.Del("HX-Request")
	clone.Header.Del("HX-Target")
	buffer := &storefrontPreviewBuffer{header: make(http.Header)}
	h.storefrontPreview.ServeHTTP(buffer, clone)
	if buffer.status >= 400 {
		fail(buffer.status, "Preview could not be rendered. Your changes remain in the editor.")
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"html": buffer.String(), "version": m.Draft.Version})
}

type storefrontPreviewBuffer struct {
	bytes.Buffer
	header http.Header
	status int
}

func (b *storefrontPreviewBuffer) Header() http.Header { return b.header }
func (b *storefrontPreviewBuffer) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}
func (b *storefrontPreviewBuffer) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = 200
	}
	return b.Buffer.Write(p)
}

func (h *LivePortalHandler) validStorefrontPreviewPath(target string) (*url.URL, error) {
	if target == "" {
		target = "/"
	}
	parsed, e := url.Parse(target)
	if e != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") || strings.ContainsAny(target, "\\\r\n") || h.storefrontPreview == nil || !h.storefrontPreview.ownsRoute(parsed.Path) || strings.HasPrefix(parsed.Path, "/account/profile/photo") || strings.Contains(parsed.Path, "/label") || strings.Contains(parsed.Path, "/invoice") || strings.HasPrefix(parsed.Path, "/payments/") {
		return nil, fmt.Errorf("Choose a customer page to preview.")
	}
	return parsed, nil
}

func editorStructuredRows(r *http.Request) error {
	// Structured controls are additive. Native legacy forms retain their parser.
	for _, key := range []string{"navigation", "policy_links", "social"} {
		if raw, ok := r.Form[key+"_rows"]; ok {
			var rows []storefront.Link
			if len(raw) != 1 || json.Unmarshal([]byte(raw[0]), &rows) != nil || len(rows) > 60 {
				return storefront.ErrValidation
			}
			// Parsed separately after the legacy form has been applied.
			r.Form.Set(key, "")
		}
	}
	for _, key := range []string{"items", "trust", "footer"} {
		if raw, ok := r.Form[key+"_rows"]; ok {
			if len(raw) != 1 || !json.Valid([]byte(raw[0])) {
				return storefront.ErrValidation
			}
			r.Form.Set(key, "")
		}
	}
	return nil
}

func applyEditorStructuredRows(r *http.Request, m *viewmodels.StorefrontEditor) error {
	c := &m.Draft.Config
	decode := func(key string, target any) error {
		if raw, ok := r.Form[key+"_rows"]; ok {
			return json.Unmarshal([]byte(raw[0]), target)
		}
		return nil
	}
	if m.Tab == "navigation" {
		if err := decode("navigation", &c.Navigation); err != nil {
			return err
		}
		if err := decode("policy_links", &c.PolicyLinks); err != nil {
			return err
		}
		if err := decode("trust", &c.Trust); err != nil {
			return err
		}
		if err := decode("footer", &c.FooterGroups); err != nil {
			return err
		}
	}
	if m.Tab == "settings" {
		if err := decode("social", &c.Settings.Social); err != nil {
			return err
		}
	}
	if _, ok := r.Form["items_rows"]; ok {
		var rows []storefront.Tile
		if err := decode("items", &rows); err != nil {
			return err
		}
		parts := strings.SplitN(m.Selected, ":", 2)
		update := func(sections []storefront.Section, id string) {
			for i := range sections {
				if sections[i].ID == id {
					sections[i].Items = rows
				}
			}
		}
		switch m.Tab {
		case "homepage":
			update(c.Homepage, m.Selected)
		case "template-sections":
			if len(parts) == 2 {
				for i := range c.Templates {
					if c.Templates[i].ID == parts[0] {
						update(c.Templates[i].Sections, parts[1])
					}
				}
			}
		case "page-sections":
			if len(parts) == 2 {
				for i := range c.Pages {
					if c.Pages[i].Slug == parts[0] {
						update(c.Pages[i].Sections, parts[1])
					}
				}
			}
		}
	}
	return c.Validate()
}

func storefrontGuidance(m *viewmodels.StorefrontEditor) {
	c := m.Draft.Config
	add := func(label, hint, tab, id string, warning bool) {
		m.Readiness = append(m.Readiness, viewmodels.StorefrontFinding{Label: label, Hint: hint, Tab: tab, Selected: id, Warning: warning})
	}
	if err := c.Validate(); err != nil {
		add("Check required content", err.Error(), "homepage", "", false)
	}
	for _, s := range c.Slides {
		if !s.Hidden && strings.TrimSpace(s.Alt) == "" {
			add("Describe the hero photograph", s.Title, "hero", s.ID, false)
		}
	}
	check := func(sections []storefront.Section, tab, owner string) {
		for _, s := range sections {
			if s.Hidden {
				continue
			}
			id := owner + s.ID
			if s.Image != "" && strings.TrimSpace(s.Alt) == "" {
				add("Describe the section photograph", s.Title, tab, id, false)
			}
			if s.Type == "products" && s.Selection.Mode == "selected" && len(s.Selection.Products) == 0 {
				add("Choose products for this collection", s.Title+" will show its empty state until products are selected.", tab, id, true)
			}
			for _, tile := range s.Items {
				if tile.Image != "" && strings.TrimSpace(tile.Alt) == "" {
					add("Describe a collection photograph", tile.Title, tab, id, false)
				}
			}
		}
	}
	check(c.Homepage, "homepage", "")
	for _, t := range c.Templates {
		check(t.Sections, "template-sections", t.ID+":")
	}
	for _, p := range c.Pages {
		if !p.Hidden {
			check(p.Sections, "page-sections", p.Slug+":")
		}
	}
	m.Destinations = []storefront.Link{{Label: "Home", URL: "/"}, {Label: "All products", URL: "/products"}, {Label: "New arrivals", URL: "/products?sort=newest"}, {Label: "Makers", URL: "/makers"}, {Label: "Shopping cart", URL: "/cart"}}
	for _, cat := range c.Categories {
		m.Destinations = append(m.Destinations, storefront.Link{Label: cat.Name, URL: "/products?category=" + url.QueryEscape(cat.Slug)})
	}
	for _, p := range c.Pages {
		path := "/pages/" + p.Slug
		if isInfoRoute("/" + p.Slug) {
			path = "/" + p.Slug
		}
		m.Destinations = append(m.Destinations, storefront.Link{Label: p.Title, URL: path})
	}
}

func storefrontPreset(kind string, order int) storefront.Section {
	s := storefront.Section{ID: editorID(), Type: kind, Title: "New section", Order: order, Style: "grid", Selection: storefront.Selection{Mode: "featured", Limit: 6, Available: true}, Responsive: storefront.Responsive{DesktopColumns: 6, TabletColumns: 3, MobileColumns: 2, Spacing: 24, MobileSpacing: 20}}
	switch kind {
	case "curated":
		s.Type = "products"
		s.Title = "Curated collection"
		s.Selection.Mode = "selected"
	case "gifts":
		s.Type = "products"
		s.Title = "Gifts under ₹1,500"
		s.Style = "band"
		s.Selection.MaxPrice = 149999
	case "story":
		s.Type = "makers"
		s.Title = "Meet the makers"
		s.Button = storefront.Link{Label: "Explore makers", URL: "/makers"}
	case "promotion":
		s.Type = "banner"
		s.Title = "Explore a collection"
		s.Button = storefront.Link{Label: "Browse products", URL: "/products"}
	case "questions":
		s.Type = "faq"
		s.Title = "Questions about this collection"
	}
	return s
}

func editorContextURL(root string, id int64, tab, selected string) string {
	return root + "/storefront?" + url.Values{"draft": {strconv.FormatInt(id, 10)}, "tab": {tab}, "section": {selected}}.Encode()
}

func (h *LivePortalHandler) storefrontEditorLibrary(w http.ResponseWriter, r *http.Request, actor int64, root string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	if r.URL.Query().Get("kind") == "destinations" {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if len(q) > 100 {
			http.Error(w, "Search is too long.", 422)
			return
		}
		rows, err := h.pool.Query(r.Context(), `SELECT name, '/products/'||slug FROM products WHERE status='approved' AND ($1='' OR name ILIKE '%'||$1||'%') UNION ALL SELECT display_name,'@maker' FROM sellers WHERE status='active' AND ($1='' OR display_name ILIKE '%'||$1||'%') ORDER BY 1 LIMIT 30`, q)
		if err != nil {
			http.Error(w, "Destinations could not be loaded.", 503)
			return
		}
		defer rows.Close()
		links := []storefront.Link{}
		for rows.Next() {
			var l storefront.Link
			if err = rows.Scan(&l.Label, &l.URL); err != nil {
				http.Error(w, "Destinations could not be loaded.", 503)
				return
			}
			if l.URL == "@maker" {
				l.URL = "/makers/" + liveSlug(l.Label)
			}
			links = append(links, l)
		}
		if rows.Err() != nil {
			http.Error(w, "Destinations could not be loaded.", 503)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"links": links})
		return
	}
	m := viewmodels.StorefrontEditor{}
	m.Admin.PortalRoot = root
	m.Draft.ID = 1
	media, err := h.storefrontMediaList(r, &m)
	if err != nil {
		http.Error(w, "Photographs could not be loaded.", 503)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"media": media, "page": m.MediaPage, "previous": m.PreviousMedia != "", "next": m.NextMedia != ""})
}
