package commerce

import (
	"context"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	identity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
	"strings"
	"unicode/utf8"
)

type SellerConfirmationRepository interface {
	CaseSellers(context.Context, int64, int64) ([]domain.CaseSeller, error)
	ListSellerConfirmations(context.Context, int64, int64, bool, string, int64) ([]domain.SellerConfirmation, error)
	CreateSellerConfirmation(context.Context, int64, int64, int64, string, string) (int64, error)
	ReplySellerConfirmation(context.Context, int64, int64, string, string) error
	CloseSellerConfirmation(context.Context, int64, int64, string) error
}
type SellerConfirmationService struct {
	repository SellerConfirmationRepository
	roles      ports.RoleChecker
}

func NewSellerConfirmationService(repo SellerConfirmationRepository, roles ports.RoleChecker) *SellerConfirmationService {
	return &SellerConfirmationService{repo, roles}
}
func (s *SellerConfirmationService) company(ctx context.Context, id int64) bool {
	if s.roles == nil {
		return false
	}
	ok, err := s.roles.HasAnyRole(ctx, id, identity.RoleAdmin, identity.RoleSuperAdmin)
	return id > 0 && err == nil && ok
}
func (s *SellerConfirmationService) seller(ctx context.Context, id int64) bool {
	if s.roles == nil {
		return false
	}
	ok, err := s.roles.HasAnyRole(ctx, id, identity.RoleSeller)
	return id > 0 && err == nil && ok
}
func (s *SellerConfirmationService) CaseSellers(ctx context.Context, actor, ticket int64) ([]domain.CaseSeller, error) {
	if !s.company(ctx, actor) {
		return nil, ports.ErrForbidden
	}
	return s.repository.CaseSellers(ctx, actor, ticket)
}
func (s *SellerConfirmationService) List(ctx context.Context, actor, ticket int64, seller bool) ([]domain.SellerConfirmation, error) {
	if seller {
		if !s.seller(ctx, actor) {
			return nil, ports.ErrForbidden
		}
	} else if !s.company(ctx, actor) {
		return nil, ports.ErrForbidden
	}
	return s.Queue(ctx, actor, ticket, seller, "", 0)
}
func validConfirmationText(text, key string) bool {
	return utf8.RuneCountInString(text) > 0 && utf8.RuneCountInString(text) <= 4000 && len(key) >= 16 && len(key) <= 200
}
func (s *SellerConfirmationService) Create(ctx context.Context, actor, ticket, seller int64, question, key string) (int64, error) {
	if !s.company(ctx, actor) {
		return 0, ports.ErrForbidden
	}
	question = strings.TrimSpace(question)
	if ticket <= 0 || seller <= 0 || !validConfirmationText(question, key) {
		return 0, ErrInvalidSupportTicket
	}
	return s.repository.CreateSellerConfirmation(ctx, actor, ticket, seller, question, key)
}
func (s *SellerConfirmationService) Reply(ctx context.Context, actor, id int64, body, key string) error {
	if !s.seller(ctx, actor) {
		return ports.ErrForbidden
	}
	body = strings.TrimSpace(body)
	if id <= 0 || !validConfirmationText(body, key) {
		return ErrInvalidSupportTicket
	}
	return s.repository.ReplySellerConfirmation(ctx, actor, id, body, key)
}
func (s *SellerConfirmationService) Close(ctx context.Context, actor, id int64, reason string) error {
	if !s.company(ctx, actor) {
		return ports.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if id <= 0 || utf8.RuneCountInString(reason) < 4 || utf8.RuneCountInString(reason) > 1000 {
		return ErrInvalidSupportTicket
	}
	return s.repository.CloseSellerConfirmation(ctx, actor, id, reason)
}

func (s *SellerConfirmationService) Queue(ctx context.Context, actor, ticket int64, seller bool, status string, before int64) ([]domain.SellerConfirmation, error) {
	if seller {
		if !s.seller(ctx, actor) {
			return nil, ports.ErrForbidden
		}
	} else if !s.company(ctx, actor) {
		return nil, ports.ErrForbidden
	}
	if before < 0 || (status != "" && status != "open" && status != "answered" && status != "closed") {
		return nil, ErrInvalidSupportTicket
	}
	return s.repository.ListSellerConfirmations(ctx, actor, ticket, seller, status, before)
}
