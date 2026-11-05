package pages_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/admin"
	"github.com/wecratfs/commerce/web/assets"
	"github.com/wecratfs/commerce/web/pages"
	"github.com/wecratfs/commerce/web/selleradmin"
	"github.com/wecratfs/commerce/web/supportportal"
	"github.com/wecratfs/commerce/web/workspaces"
)

func TestDocumentsVersionDependencyURLsAndMatchFontPreloads(t *testing.T) {
	fontCSS, err := assets.FS.ReadFile("css/fonts.min.css")
	if err != nil {
		t.Fatal(err)
	}
	versioned := func(name string) string {
		t.Helper()
		data, err := assets.FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		return fmt.Sprintf("/assets/%s?v=%x", name, digest[:8])
	}
	for _, tc := range []struct {
		name      string
		document  templ.Component
		scripts   []string
		preloaded []string
	}{
		{"storefront", pages.Document(viewmodels.CustomerPage{Route: "/products"}), []string{"htmx", "alpine"}, []string{"dm-sans-400", "playfair-display-500", "cormorant-garamond-600"}},
		{"admin", admin.AdminDocument(viewmodels.AdminPage{}), []string{"htmx", "alpine"}, []string{"inter-latin", "roboto-latin"}},
		{"seller", selleradmin.Document(viewmodels.SellerAdminPage{}), []string{"htmx", "alpine"}, []string{"inter-latin", "roboto-latin"}},
		{"support", supportportal.SupportDocument(viewmodels.SupportPortalPage{}), []string{"alpine"}, []string{"inter-latin", "roboto-latin"}},
		{"workspaces", workspaces.Document(viewmodels.WorkspacePage{}), nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := tc.document.Render(context.Background(), &out); err != nil {
				t.Fatal(err)
			}
			html := out.String()
			for _, script := range tc.scripts {
				if !strings.Contains(html, `src="`+versioned("js/"+script+".min.js")+`"`) {
					t.Errorf("%s script must use its current content version", script)
				}
			}
			if !strings.Contains(html, `href="`+versioned("css/fonts.min.css")+`"`) {
				t.Error("font stylesheet must use its current content version")
			}
			for _, font := range tc.preloaded {
				url := versioned("fonts/" + font + ".woff2")
				if !strings.Contains(html, `href="`+url+`"`) || !strings.Contains(string(fontCSS), url) {
					t.Errorf("%s font preload and CSS must request the same content URL", font)
				}
			}
		})
	}
}
