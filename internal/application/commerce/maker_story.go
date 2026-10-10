package commerce

import (
	"context"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
	"strings"
	"unicode/utf8"
)

// Keep existing profile clients compatible while making public content explicit.
type makerStoryRepository interface {
	UpdateMakerStory(context.Context, int64, domaincommerce.MakerStoryInput) error
}

func (s *CatalogService) UpdateMakerStory(ctx context.Context, ownerID int64, input domaincommerce.MakerStoryInput) error {
	if ownerID <= 0 || !s.allowed(ctx, ownerID, domainidentity.RoleSellerOwner) {
		return ports.ErrForbidden
	}
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Location = strings.TrimSpace(input.Location)
	input.Story = strings.TrimSpace(input.Story)
	input.Technique = strings.TrimSpace(input.Technique)
	input.PortraitURL = strings.TrimSpace(input.PortraitURL)
	input.Reason = strings.TrimSpace(input.Reason)
	if utf8.RuneCountInString(input.DisplayName) < 2 || utf8.RuneCountInString(input.DisplayName) > 120 || utf8.RuneCountInString(input.Location) > 160 || utf8.RuneCountInString(input.Story) > 4000 || utf8.RuneCountInString(input.Technique) > 500 || len(input.PortraitURL) > 500 || len(input.Reason) < 4 || len(input.Reason) > 1000 {
		return ErrInvalidProduct
	}
	if input.PortraitURL != "" && !strings.HasPrefix(input.PortraitURL, "/assets/images/uploads/upload_") {
		return ErrInvalidProduct
	}
	repository, ok := s.repository.(makerStoryRepository)
	if !ok {
		return ErrInvalidProduct
	}
	return repository.UpdateMakerStory(ctx, ownerID, input)
}
