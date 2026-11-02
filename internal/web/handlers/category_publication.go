package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wecratfs/commerce/internal/adapters/postgres"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

var categorySlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func (h *LivePortalHandler) prepareCategoryImage(r *http.Request) (*postgres.CategoryImage, func(), error) {
	file, _, err := r.FormFile("image_file")
	if errors.Is(err, http.ErrMissingFile) || errors.Is(err, http.ErrNotMultipart) {
		return nil, func() {}, nil
	}
	if err != nil {
		return nil, func() {}, ports.ErrManagementValidation
	}
	defer file.Close()
	input, err := io.ReadAll(io.LimitReader(file, (8<<20)+1))
	if err != nil {
		return nil, func() {}, err
	}
	// Validate before writing any objects.
	output, err := normalizeStorefrontImage(input, 1920)
	if err != nil {
		return nil, func() {}, err
	}
	writer, ok := h.categoryStorage.(ports.ObjectStorageWriter)
	if !ok {
		return nil, func() {}, errors.New("Image storage is unavailable. Your category has not been saved.")
	}
	asset := &postgres.CategoryImage{ID: editorID(), Variants: map[string]string{}}
	asset.ObjectKey = "storefront/" + asset.ID + ".jpg"
	asset.ByteSize = int64(len(output))
	keys := []string{}
	cleanup := func() {
		if storage, ok := h.categoryStorage.(ports.ObjectStorageReader); ok {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 10*time.Second)
			defer cancel()
			for _, key := range keys {
				_ = storage.DeleteObject(ctx, key)
			}
		}
	}
	for _, width := range []int{1920, 960, 480} {
		key := asset.ObjectKey
		if width != 1920 {
			key = "storefront/" + asset.ID + "-" + strconv.Itoa(width) + ".jpg"
			output, err = normalizeStorefrontImage(input, width)
			if err != nil {
				cleanup()
				return nil, func() {}, err
			}
		}
		keys = append(keys, key)
		if err = writer.PutObject(r.Context(), key, "image/jpeg", int64(len(output)), bytes.NewReader(output)); err != nil {
			cleanup()
			return nil, func() {}, errors.New("The photograph could not be stored. Your published category is unchanged. Keep this selection and retry when storage is available.")
		}
		asset.Variants[strconv.Itoa(width)] = key
	}
	return asset, cleanup, nil
}

func (h *LivePortalHandler) saveCategoryPublication(r *http.Request, actor int64, m *viewmodels.CategoryManagerPage) (int, error) {
	m.EditorOpen = true
	m.EditorAction = strings.TrimPrefix(r.URL.Path, m.Action+"/")
	m.EditorReason = r.Form.Get("reason")
	order, err := strconv.Atoi(r.Form.Get("display_order"))
	m.ExpectedRelease, _ = strconv.ParseInt(r.Form.Get("expected_release"), 10, 64)
	m.Editor = ports.ManagementRecord{ID: strings.TrimSpace(r.Form.Get("id")), Name: strings.TrimSpace(r.Form.Get("name")), DisplayOrder: order, ShowInNavigation: r.Form.Get("show_in_navigation") == "true"}
	if err != nil || len(m.Editor.Name) == 0 || len(m.Editor.Name) > 120 || len(strings.TrimSpace(m.EditorReason)) < 4 || len(m.EditorReason) > 1000 || order < 0 || order > 1000000 || len(m.Editor.ID) > 120 || !categorySlugPattern.MatchString(m.Editor.ID) {
		return 422, ports.ErrManagementValidation
	}
	// Reload only display metadata; no submitted URL or object key is trusted.
	if m.EditorAction == "update" {
		_ = h.pool.QueryRow(r.Context(), `SELECT image_url FROM categories WHERE slug=$1`, m.Editor.ID).Scan(&m.Editor.ImageURL)
	}
	image, cleanup, err := h.prepareCategoryImage(r)
	if err != nil {
		m.Error = err.Error()
		if strings.HasPrefix(m.Error, "Choose ") || strings.HasPrefix(m.Error, "This photograph") {
			return 422, err
		}
		return 503, err
	}
	if m.EditorAction == "create" && image == nil {
		m.Error = "Choose a category photograph before saving."
		return 422, ports.ErrManagementValidation
	}
	err = postgres.NewStorefrontRepository(h.pool).PublishCategory(r.Context(), actor, m.ExpectedRelease, m.EditorAction, storefront.Category{Slug: m.Editor.ID, Name: m.Editor.Name, Order: order, Visible: m.Editor.ShowInNavigation}, image, m.EditorReason)
	if err == nil {
		return 303, nil
	}
	cleanup()
	switch {
	case errors.Is(err, storefront.ErrConflict):
		m.Error = "The storefront changed while you were editing. Reload the latest categories before applying your changes."
		return 409, err
	case errors.Is(err, ports.ErrManagementConflict):
		m.Error = "This URL slug is already used. Choose a different slug or edit the existing category."
		return 409, err
	case errors.Is(err, ports.ErrManagementValidation), errors.Is(err, storefront.ErrValidation):
		return 422, err
	case errors.Is(err, ports.ErrForbidden):
		return 403, err
	case errors.Is(err, ports.ErrNotFound):
		return 404, err
	default:
		return 503, err
	}
}
