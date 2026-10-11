package commerce

import (
	"context"
	"errors"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

type storyRepository struct {
	catalogRepository
	saved *domaincommerce.MakerStoryInput
}

func (r *storyRepository) UpdateMakerStory(_ context.Context, _ int64, in domaincommerce.MakerStoryInput) error {
	r.saved = &in
	return nil
}
func TestMakerStoryRequiresOwnerAndKeepsMissingContentEmpty(t *testing.T) {
	r := &storyRepository{}
	s := NewCatalogServiceWithRoles(r, roleChecker{roles: map[int64]map[domainidentity.Role]bool{1: {domainidentity.RoleSeller: true}}})
	in := domaincommerce.MakerStoryInput{DisplayName: " Real studio ", Reason: "Update public name"}
	if err := s.UpdateMakerStory(t.Context(), 2, in); !errors.Is(err, ports.ErrForbidden) || r.saved != nil {
		t.Fatal("non-owner could write public story", err)
	}
	if err := s.UpdateMakerStory(t.Context(), 1, in); err != nil {
		t.Fatal(err)
	}
	if r.saved.DisplayName != "Real studio" || r.saved.Story != "" || r.saved.PortraitURL != "" {
		t.Fatal("invented or untrimmed maker content", r.saved)
	}
	in.PortraitURL = "https://tracking.example/portrait.png"
	if err := s.UpdateMakerStory(t.Context(), 1, in); !errors.Is(err, ErrInvalidProduct) {
		t.Fatal("accepted external portrait", err)
	}
}
