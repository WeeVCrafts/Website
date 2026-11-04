package commerce

import (
	"context"
	"errors"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	identity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
	"strings"
	"testing"
)

func TestProductTagsAndGeneratedIdentifiers(t *testing.T) {
	input := domain.ProductDraftInput{Name: "Ceramic mug", Description: "Hand painted mug", CategorySlug: "home", CategoryName: "Home", PriceCents: 10000, SearchTags: []string{" Floral  Mug ", "floral mug", "", "CERAMIC"}}
	first, err := normalizeDraft(input)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(first.SearchTags, ",") != "floral mug,ceramic" {
		t.Fatalf("tags=%v", first.SearchTags)
	}
	second, err := normalizeDraft(input)
	if err != nil || first.Slug == second.Slug || first.SKU == second.SKU {
		t.Fatal("generated identifiers collided")
	}
	kept, err := normalizeDraft(first)
	if err != nil || kept.Slug != first.Slug || kept.SKU != first.SKU {
		t.Fatal("existing identifiers changed")
	}
	input.SearchTags = []string{strings.Repeat("x", 41)}
	if _, err := normalizeDraft(input); err == nil {
		t.Fatal("accepted long tag")
	}
	input.SearchTags = nil
	for i := 0; i < 21; i++ {
		input.SearchTags = append(input.SearchTags, strings.Repeat("x", i+1))
	}
	if _, err := normalizeDraft(input); err == nil {
		t.Fatal("accepted more than 20 tags")
	}
}

func (r *inventoryRepository) AdjustAdminInventory(_ context.Context, _ int64, a domain.InventoryAdjustment) error {
	r.adjustment = a
	return nil
}

func TestInventoryAdminAdjustmentRequiresSuperAdmin(t *testing.T) {
	repository := &inventoryRepository{}
	checker := roleChecker{roles: map[int64]map[identity.Role]bool{1: {identity.RoleSuperAdmin: true}, 2: {identity.RoleAdmin: true}, 3: {identity.RoleSeller: true}}}
	service := NewInventoryService(repository, checker)
	adjustment := domain.InventoryAdjustment{VariantID: 7, Delta: 2, Reason: "Stock count"}
	for _, actor := range []int64{2, 3, 99} {
		if err := service.AdjustAdmin(context.Background(), actor, adjustment); !errors.Is(err, ports.ErrForbidden) {
			t.Fatalf("actor %d allowed: %v", actor, err)
		}
	}
	if err := service.AdjustAdmin(context.Background(), 1, adjustment); err != nil {
		t.Fatal(err)
	}
	if repository.adjustment.Delta != 2 {
		t.Fatal("adjustment not saved")
	}
	adjustment.Delta = 0
	if err := service.AdjustAdmin(context.Background(), 1, adjustment); !errors.Is(err, ErrInvalidInventoryAdjustment) {
		t.Fatal("accepted zero adjustment")
	}
}
