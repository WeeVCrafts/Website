package handlers

import (
	"fmt"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"net/http"
	"net/url"
	"slices"
	"strconv"
)

func (h *LivePortalHandler) storefrontProductPicker(r *http.Request, m *viewmodels.StorefrontEditor) error {
	selected := m.Section.Selection.Products
	if m.Tab == "promotions" {
		selected = m.Promotion.ProductIDs
	}
	if selected == nil {
		selected = []int64{}
	}
	query := r.URL.Query().Get("product_q")
	page, _ := strconv.Atoi(r.URL.Query().Get("product_page"))
	if page < 1 || page > 100000 {
		page = 1
	}
	var total int
	if e := h.pool.QueryRow(r.Context(), `SELECT count(*) FROM products WHERE status='approved' AND ($1='' OR name ILIKE '%'||$1||'%' OR slug ILIKE '%'||$1||'%')`, query).Scan(&total); e != nil {
		return e
	}
	pages := max(1, (total+24)/25)
	page = min(page, pages)
	rows, e := h.pool.Query(r.Context(), `WITH matching AS (SELECT id FROM products WHERE status='approved' AND ($1='' OR name ILIKE '%'||$1||'%' OR slug ILIKE '%'||$1||'%') ORDER BY name,id LIMIT 25 OFFSET $2)
 SELECT p.id,p.name,p.image_url,p.status,p.price_cents,EXISTS(SELECT 1 FROM product_variants pv JOIN inventory_stock i ON i.variant_id=pv.id WHERE pv.product_id=p.id AND pv.status='active' AND i.available_quantity>0)
 FROM products p WHERE p.id IN(SELECT id FROM matching) OR p.id=ANY($3::bigint[]) ORDER BY p.name,p.id LIMIT 49`, query, (page-1)*25, selected)
	if e != nil {
		return e
	}
	defer rows.Close()
	for rows.Next() {
		var p viewmodels.StorefrontProductChoice
		var price int64
		if e = rows.Scan(&p.ID, &p.Name, &p.Image, &p.Status, &price, &p.InStock); e != nil {
			return e
		}
		p.Price = storefrontMoney(price)
		p.Selected = slices.Contains(selected, p.ID)
		m.ProductChoices = append(m.ProductChoices, p)
	}
	if e = rows.Err(); e != nil {
		return e
	}
	m.ProductQuery = query
	m.ProductTotal = total
	m.ProductPage = page
	link := func(target int) string {
		return m.Admin.PortalRoot + "/storefront?" + url.Values{"draft": {fmt.Sprint(m.Draft.ID)}, "tab": {m.Tab}, "section": {m.Selected}, "product_q": {query}, "product_page": {fmt.Sprint(target)}}.Encode()
	}
	if page > 1 {
		m.PreviousProducts = link(page - 1)
	}
	if page < pages {
		m.NextProducts = link(page + 1)
	}
	return nil
}
