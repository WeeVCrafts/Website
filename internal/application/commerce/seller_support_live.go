package commerce

import (
	"context"
	"strings"
	"unicode/utf8"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

// SellerSupportRepository links seller requests to orders containing that seller's items.
type SellerSupportRepository interface {
	CreateSellerSupportTicket(context.Context, int64, string, string, string) (domaincommerce.SupportTicket, error)
	ReplySellerSupportTicketWithID(context.Context, int64, int64, string) (int64, error)
}

func (s *SupportService) CreateSellerRequest(ctx context.Context, userID int64, orderNumber, subject, message string) (domaincommerce.SupportTicket, error) {
	if !s.sellerAllowed(ctx, userID) {
		return domaincommerce.SupportTicket{}, ports.ErrForbidden
	}
	orderNumber, subject, message = strings.TrimSpace(orderNumber), strings.TrimSpace(subject), strings.TrimSpace(message)
	if err := validateSupportTicket(orderNumber, subject, message); err != nil {
		return domaincommerce.SupportTicket{}, err
	}
	repository, ok := s.repository.(SellerSupportRepository)
	if !ok {
		return domaincommerce.SupportTicket{}, ports.ErrForbidden
	}
	return repository.CreateSellerSupportTicket(ctx, userID, orderNumber, subject, message)
}

func (s *SupportService) ReplySellerWithMessageID(ctx context.Context, userID, ticketID int64, message string) (int64, error) {
	if !s.sellerAllowed(ctx, userID) {
		return 0, ports.ErrForbidden
	}
	message = strings.TrimSpace(message)
	if ticketID <= 0 || utf8.RuneCountInString(message) == 0 || utf8.RuneCountInString(message) > 4000 {
		return 0, ErrInvalidSupportTicket
	}
	repository, ok := s.repository.(SellerSupportRepository)
	if !ok {
		return 0, ports.ErrForbidden
	}
	return repository.ReplySellerSupportTicketWithID(ctx, userID, ticketID, message)
}
