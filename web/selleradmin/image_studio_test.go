package selleradmin

import (
	"net/http/httptest"
	"strings"
	"testing"

	domainstudio "github.com/wecratfs/commerce/internal/domain/imagestudio"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestPortalImageStudioRendersEditorControlsAndNavigation(t *testing.T) {
	page := viewmodels.SellerAdminPage{
		Active:          "image-studio",
		Title:           "AI Image Studio",
		Subtitle:        "Remove backgrounds locally",
		DisplayName:     "Asha Rao",
		RoleLabel:       "Seller",
		Initials:        "AR",
		SellerName:      "Asha's Studio",
		SellerStatus:    "active",
		UserEmail:       "asha@example.test",
		CSRFToken:       "csrf-studio-test",
		CanProducts:     true,
		CanProductWrite: true,
		StudioProductID: "12",
		StudioProducts: []domainstudio.ProductOption{
			{ID: 12, Name: "Brass Elephant", CategorySlug: "brass-metalwork", CategoryName: "Brass & Metalwork", Status: "draft"},
		},
	}

	rec := httptest.NewRecorder()
	if err := PortalImageStudio(page).Render(t.Context(), rec); err != nil {
		t.Fatalf("render image studio: %v", err)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"/seller-admin/image-studio",
		"AI Image Studio",
		"data-image-studio",
		"data-csrf-token=\"csrf-studio-test\"",
		"Brass Elephant (Brass &amp; Metalwork)",
		"data-studio-canvas",
		"data-studio-apply",
		"data-studio-regenerate",
		"data-studio-reject-job",
		"image-studio.min.js",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered AI Image Studio missing %q", want)
		}
	}
}

func TestProductFormIncludesAIImageStudioLink(t *testing.T) {
	page := viewmodels.SellerAdminPage{
		Active:          "products",
		Title:           "Edit product",
		DisplayName:     "Asha Rao",
		RoleLabel:       "Seller",
		Initials:        "AR",
		SellerName:      "Asha's Studio",
		SellerStatus:    "active",
		CSRFToken:       "csrf-form",
		CanProducts:     true,
		CanProductWrite: true,
	}
	form := viewmodels.SellerProductForm{
		ID:     "42",
		Name:   "Handloom Saree",
		Action: "/seller-admin/products/42/edit",
		Status: "draft",
	}
	rec := httptest.NewRecorder()
	if err := ProductForm(page, form).Render(t.Context(), rec); err != nil {
		t.Fatalf("render product form: %v", err)
	}
	if !strings.Contains(rec.Body.String(), "/seller-admin/image-studio?product_id=42") {
		t.Fatal("expected ProductForm gallery section to link to AI Image Studio for product 42")
	}
}
