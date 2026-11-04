package handlers

import (
	"crypto/subtle"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	adminpages "github.com/wecratfs/commerce/web/admin"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (h *LivePortalHandler) SetCategoryStorage(storage ports.ObjectStorage) {
	h.categoryStorage = storage
}

// Versioned public references never expose the private bucket or object key.
func (h *LivePortalHandler) CategoryMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", 405)
		return
	}
	var key string
	if err := h.pool.QueryRow(r.Context(), `SELECT image_object_key FROM categories WHERE image_url=$1 AND image_object_key<>'' UNION ALL SELECT c->>'object_key' FROM storefront_releases r CROSS JOIN LATERAL jsonb_array_elements(r.config->'categories') c WHERE c->>'image'=$1 AND COALESCE(c->>'object_key','')<>'' LIMIT 1`, r.URL.Path).Scan(&key); err != nil {
		http.NotFound(w, r)
		return
	}
	reader, ok := h.categoryStorage.(ports.ObjectStorageContentReader)
	if !ok {
		http.Error(w, "Image storage unavailable", 503)
		return
	}
	body, info, err := reader.OpenObject(r.Context(), key)
	if err != nil {
		http.Error(w, "Image unavailable", 503)
		return
	}
	defer body.Close()
	if info.ContentType != "image/jpeg" || info.ByteSize <= 0 || info.ByteSize > 5<<20 {
		http.Error(w, "Image unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Length", strconv.FormatInt(info.ByteSize, 10))
	if r.Method == http.MethodGet {
		_, _ = io.Copy(w, io.LimitReader(body, info.ByteSize))
	}
}

func (h *LivePortalHandler) categoryManagement(w http.ResponseWriter, r *http.Request, actor int64, csrf, root, action string, role domainidentity.Role) {
	repository := postgres.NewManagementRepository(h.pool)
	service := applicationcommerce.NewManagementService(repository)
	base := root + "/category-management"
	model := viewmodels.CategoryManagerPage{ManagementPage: viewmodels.ManagementPage{Section: "category-management", Action: base, Page: 1}}
	model.Admin = viewmodels.AdminPage{Active: "category-management", NavActive: "products", Title: "Categories", PortalRoot: root, PortalLabel: portalLabel(role), RoleLabel: roleLabel(role), DisplayName: h.userDisplayName(r, actor), CSRFToken: csrf, CanManageCatalog: true, CanSecurity: role == domainidentity.RoleSuperAdmin, Notice: r.URL.Query().Get("notice")}
	model.Editor = ports.ManagementRecord{DisplayOrder: 1000, ShowInNavigation: true}
	responseStatus := http.StatusOK
	if action != "" {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "Method not allowed", 405)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 9<<20)
		var err error
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			err = r.ParseMultipartForm(9 << 20)
			if r.MultipartForm != nil {
				defer r.MultipartForm.RemoveAll()
			}
		} else {
			err = r.ParseForm()
		}
		if err != nil {
			http.Error(w, "Choose a photograph up to 8 MiB and try again.", 413)
			return
		}
		token := r.Form.Get("csrf_token")
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(csrf)) != 1 {
			http.Error(w, "Your session changed. Reload before saving.", 403)
			return
		}
		if action == "assign-products" {
			model.SelectedCategory = r.Form.Get("id")
			values := r.URL.Query()
			values.Set("product_q", r.Form.Get("product_q"))
			values.Set("product_page", r.Form.Get("product_page"))
			r.URL.RawQuery = values.Encode()
			ids := []int64{}
			seen := map[int64]bool{}
			for _, raw := range r.Form["product_id"] {
				id, e := strconv.ParseInt(raw, 10, 64)
				if e != nil || id <= 0 {
					err = ports.ErrManagementValidation
					break
				}
				if !seen[id] {
					ids = append(ids, id)
					seen[id] = true
				}
			}
			model.SelectedIDs = seen
			reason, intent := strings.TrimSpace(r.Form.Get("reason")), r.Form.Get("intent")
			if len(ids) == 0 || len(ids) > 25 || len(reason) < 4 || len(reason) > 1000 || (intent != "add" && intent != "remove") {
				err = ports.ErrManagementValidation
			}
			if err == nil {
				err = repository.AssignCategoryProducts(r.Context(), actor, model.SelectedCategory, intent, reason, ids)
			}
			responseStatus = 503
			if errors.Is(err, ports.ErrManagementValidation) {
				responseStatus = 422
			}
			if errors.Is(err, ports.ErrManagementConflict) {
				responseStatus = 409
				model.Error = "A selected product uses this primary category. Choose a replacement in its editor before removing membership."
			}
		} else if role == domainidentity.RoleSuperAdmin && (action == "create" || action == "update") {
			responseStatus, err = h.saveCategoryPublication(r, actor, &model)
		} else {
			http.Error(w, "Forbidden", 403)
			return
		}
		if err == nil {
			v := url.Values{"q": {r.URL.Query().Get("q")}, "page": {r.URL.Query().Get("page")}, "notice": {"Category saved and published."}}
			if action == "assign-products" {
				v.Set("notice", "Product assignments saved.")
				v.Set("category", model.SelectedCategory)
				v.Set("product_q", r.Form.Get("product_q"))
				v.Set("product_page", r.Form.Get("product_page"))
			}
			http.Redirect(w, r, base+"?"+v.Encode(), 303)
			return
		}
		if responseStatus == 403 {
			http.Error(w, "Forbidden", 403)
			return
		}
		if model.Error == "" {
			model.Error = "Changes could not be saved. Check the name, unique URL slug, order and audit reason. Your entries are retained; select your photograph again if using the basic form."
		}
	}
	if action == "" && r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	model.Query = strings.TrimSpace(r.URL.Query().Get("q"))
	model.Page = positivePage(r.URL.Query().Get("page"))
	result, err := service.List(r.Context(), actor, "category-management", model.Query, model.Page)
	if err != nil {
		http.Error(w, "Categories could not be loaded", 503)
		return
	}
	model.Records = result.Records
	model.Total = result.Total
	pageURL := func(page int) string {
		v := url.Values{"q": {model.Query}, "page": {strconv.Itoa(page)}}
		return base + "?" + v.Encode()
	}
	if model.Page > 1 {
		model.PreviousURL = pageURL(model.Page - 1)
	}
	if model.Page*25 < model.Total {
		model.NextURL = pageURL(model.Page + 1)
	}
	model.ReturnURL = base + "?" + url.Values{"q": {model.Query}, "page": {strconv.Itoa(model.Page)}}.Encode()
	model.NewURL = model.ReturnURL + "&new=1"
	if role == domainidentity.RoleSuperAdmin {
		if !model.EditorOpen {
			revision, e := postgres.ReadPublishedStorefront(r.Context(), h.pool)
			if e != nil {
				http.Error(w, "Categories could not be loaded", 503)
				return
			}
			model.ExpectedRelease = revision.ID
			if slug := r.URL.Query().Get("edit"); slug != "" {
				e = h.pool.QueryRow(r.Context(), `SELECT slug,name,image_url,display_order,show_in_navigation FROM categories WHERE slug=$1`, slug).Scan(&model.Editor.ID, &model.Editor.Name, &model.Editor.ImageURL, &model.Editor.DisplayOrder, &model.Editor.ShowInNavigation)
				if e != nil {
					if errors.Is(e, pgx.ErrNoRows) {
						http.NotFound(w, r)
					} else {
						http.Error(w, "Categories could not be loaded", 503)
					}
					return
				}
				model.EditorOpen = true
				model.EditorAction = "update"
			} else if r.URL.Query().Get("new") == "1" {
				model.EditorOpen = true
				model.EditorAction = "create"
			}
		}
	}
	if model.SelectedCategory == "" {
		model.SelectedCategory = r.URL.Query().Get("category")
	}
	if model.SelectedCategory != "" {
		model.ProductQuery = r.URL.Query().Get("product_q")
		page := positivePage(r.URL.Query().Get("product_page"))
		model.ProductPage = page
		model.Products, model.ProductTotal, err = repository.CategoryProducts(r.Context(), actor, model.SelectedCategory, model.ProductQuery, page)
		if err != nil {
			http.Error(w, "Products could not be loaded", 503)
			return
		}
		productURL := func(n int) string {
			return base + "?" + url.Values{"category": {model.SelectedCategory}, "product_q": {model.ProductQuery}, "product_page": {strconv.Itoa(n)}}.Encode() + "#category-products"
		}
		if page > 1 {
			model.ProductPreviousURL = productURL(page - 1)
		}
		if page*25 < model.ProductTotal {
			model.ProductNextURL = productURL(page + 1)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(responseStatus)
	h.renderAdminComponent(w, r, adminpages.CategoryManager(model), model.Admin)
}

func positivePage(raw string) int {
	page, err := strconv.Atoi(raw)
	if err != nil || page < 1 || page > 1000000 {
		return 1
	}
	return page
}
