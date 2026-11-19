package admin

import (
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"net/url"
)

func adminRequestURL(p viewmodels.AdminPage, item viewmodels.AdminReturn) string {
	kind := item.Kind
	if kind == "" {
		kind = "legacy_return"
	}
	return p.PortalRoot + "/orders/" + url.PathEscape(item.Product) + "?tab=returns&focus=" + url.QueryEscape(kind+":"+item.ID)
}
