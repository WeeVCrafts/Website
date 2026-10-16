package handlers

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
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
	if err := h.pool.QueryRow(r.Context(), `SELECT image_object_key FROM categories WHERE image_url=$1 AND image_object_key<>''`, r.URL.Path).Scan(&key); err != nil {
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
	model.Admin = viewmodels.AdminPage{Active: "category-management", NavActive: "category-management", Title: "Categories", PortalRoot: root, PortalLabel: portalLabel(role), RoleLabel: roleLabel(role), DisplayName: h.userDisplayName(r, actor), CSRFToken: csrf, CanManageCatalog: true, CanSecurity: role == domainidentity.RoleSuperAdmin, Notice: r.URL.Query().Get("notice")}
	model.Editor = ports.ManagementRecord{DisplayOrder: 1000, ShowInNavigation: true}
	responseStatus := http.StatusOK
	if action != "" {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "Method not allowed", 405)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
		var err error
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			err = r.ParseMultipartForm(6 << 20)
			if r.MultipartForm != nil {
				defer r.MultipartForm.RemoveAll()
			}
		} else {
			err = r.ParseForm()
		}
		if err != nil {
			http.Error(w, "Choose an image smaller than 5 MiB and try again.", 413)
			return
		}
		token := r.Form.Get("csrf_token")
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(csrf)) != 1 {
			http.Error(w, "Invalid CSRF token", 403)
			return
		}
		oldImageKey := ""
		if action == "update" || action == "delete" {
			_ = h.pool.QueryRow(r.Context(), `SELECT image_object_key FROM categories WHERE slug=$1`, r.Form.Get("id")).Scan(&oldImageKey)
		}
		mutation := ports.ManagementMutation{Section: "category-management", Action: action, ID: r.Form.Get("id"), Name: r.Form.Get("name"), Reason: r.Form.Get("reason"), ShowInNavigation: r.Form.Get("show_in_navigation") == "true"}
		mutation.DisplayOrder, err = strconv.Atoi(r.Form.Get("display_order"))
		if action == "delete" {
			err = nil
		}
		if action == "assign-products" {
			err = nil
			values := r.URL.Query()
			values.Set("product_q", r.Form.Get("product_q"))
			values.Set("product_page", r.Form.Get("product_page"))
			r.URL.RawQuery = values.Encode()
			model.SelectedCategory = mutation.ID
			mutation.Reason = strings.TrimSpace(mutation.Reason)
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
			intent := r.Form.Get("intent")
			if len(ids) == 0 || len(ids) > 25 || len(mutation.Reason) < 4 || len(mutation.Reason) > 1000 || (intent != "add" && intent != "remove") {
				err = ports.ErrManagementValidation
			}
			if err == nil {
				err = repository.AssignCategoryProducts(r.Context(), actor, mutation.ID, intent, mutation.Reason, ids)
			}
		} else {
			if err != nil || mutation.DisplayOrder < 0 || mutation.DisplayOrder > 1000000 {
				err = ports.ErrManagementValidation
			}
			var uploadedKey string
			if err == nil && action != "delete" {
				file, _, fileErr := r.FormFile("image_file")
				if fileErr == nil {
					input, readErr := io.ReadAll(io.LimitReader(file, (5<<20)+1))
					file.Close()
					var output []byte
					if readErr == nil {
						output, readErr = normalizeAvatar(input)
					}
					if readErr != nil {
						err = ports.ErrManagementValidation
						model.Error = "Choose a valid JPEG, PNG or WebP image up to 5 MiB and 16 megapixels."
					} else if writer, ok := h.categoryStorage.(ports.ObjectStorageWriter); !ok {
						err = errors.New("storage unavailable")
						model.Error = "Image storage is unavailable. Your category has not been saved; try again when storage is connected."
					} else {
						var version [16]byte
						if _, err = rand.Read(version[:]); err == nil {
							filename := hex.EncodeToString(version[:]) + ".jpg"
							uploadedKey = "categories/" + filename
							err = writer.PutObject(r.Context(), uploadedKey, "image/jpeg", int64(len(output)), bytes.NewReader(output))
							mutation.ImageObjectKey = uploadedKey
							mutation.ImageURL = "/media/categories/" + filename
							mutation.ReplaceImage = true
						}
						if err != nil {
							model.Error = "The image could not be stored. Your previous image is preserved. Choose the image again and retry."
						}
					}
				} else if !errors.Is(fileErr, http.ErrMissingFile) {
					err = fileErr
				}
			}
			if err == nil {
				err = service.Mutate(r.Context(), actor, mutation)
			}
			if err != nil && uploadedKey != "" {
				if storage, ok := h.categoryStorage.(ports.ObjectStorageReader); ok {
					_ = storage.DeleteObject(r.Context(), uploadedKey)
				}
			}
		}
		if err == nil {
			if oldImageKey != "" && (action == "delete" || mutation.ReplaceImage) {
				if storage, ok := h.categoryStorage.(ports.ObjectStorageReader); ok {
					_ = storage.DeleteObject(r.Context(), oldImageKey)
				}
			}
			target := base + "?notice=" + url.QueryEscape("Category changes saved.")
			if model.SelectedCategory != "" {
				target += "&" + url.Values{"category": {model.SelectedCategory}, "product_q": {r.Form.Get("product_q")}, "product_page": {r.Form.Get("product_page")}}.Encode()
			}
			http.Redirect(w, r, target, 303)
			return
		}
		responseStatus = 503
		switch {
		case errors.Is(err, ports.ErrManagementValidation):
			responseStatus = 422
		case errors.Is(err, ports.ErrManagementConflict):
			responseStatus = 409
		case errors.Is(err, ports.ErrNotFound):
			responseStatus = 404
		}
		if errors.Is(err, ports.ErrForbidden) {
			http.Error(w, "Forbidden", 403)
			return
		}
		if model.Error == "" {
			switch {
			case errors.Is(err, ports.ErrManagementConflict):
				model.Error = "This category is still in use, or is a selected product's primary category. Choose a replacement primary category in the product editor before removing it."
			case errors.Is(err, ports.ErrManagementValidation):
				model.Error = "Check the name, unique URL slug, order number, selected products and audit reason."
			default:
				model.Error = "Changes could not be saved. Your entered details are kept below; please retry."
			}
		}
		model.EditorReason = mutation.Reason
		model.Editor = ports.ManagementRecord{ID: mutation.ID, Name: mutation.Name, DisplayOrder: mutation.DisplayOrder, ShowInNavigation: mutation.ShowInNavigation}
		model.EditorAction = action
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
