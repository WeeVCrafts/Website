package commerce

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

type supportRepository struct{}

func (supportRepository) CreateSupportTicket(context.Context, int64, string, string, string) (domaincommerce.SupportTicket, error) {
	return domaincommerce.SupportTicket{TicketNumber: "SUP-1"}, nil
}
func (supportRepository) ListCustomerSupportTickets(context.Context, int64) ([]domaincommerce.SupportTicket, error) {
	return []domaincommerce.SupportTicket{{TicketNumber: "SUP-1"}}, nil
}
func (supportRepository) ListSellerSupportTickets(context.Context, int64) ([]domaincommerce.SupportTicket, error) {
	return []domaincommerce.SupportTicket{{TicketNumber: "SUP-1"}}, nil
}
func (supportRepository) ListSupportTickets(context.Context, int) ([]domaincommerce.SupportTicket, error) {
	return []domaincommerce.SupportTicket{{TicketNumber: "SUP-1"}}, nil
}
func (supportRepository) UpdateSupportTicket(context.Context, int64, int64, string, string) error {
	return nil
}
func (supportRepository) ReplySupportTicket(context.Context, int64, int64, string) error {
	return nil
}

func TestSupportServiceSeparatesCustomerAndAgentViews(t *testing.T) {
	checker := roleChecker{roles: map[int64]map[domainidentity.Role]bool{
		30: {domainidentity.RoleCustomer: true},
		31: {domainidentity.RoleSupportAgent: true},
	}}
	service := NewSupportService(supportRepository{}, checker)
	if _, err := service.Create(context.Background(), 30, "WC-1", "Damaged item", "The item arrived damaged and needs help."); err != nil {
		t.Fatalf("customer could not create ticket: %v", err)
	}
	if _, err := service.ListAgent(context.Background(), 30); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("customer could view support queue: %v", err)
	}
	if tickets, err := service.ListAgent(context.Background(), 31); err != nil || len(tickets) != 1 {
		t.Fatalf("support agent denied queue: tickets=%+v err=%v", tickets, err)
	}
}

func TestSupportServiceValidatesTicketContent(t *testing.T) {
	service := NewSupportService(supportRepository{}, roleChecker{roles: map[int64]map[domainidentity.Role]bool{30: {domainidentity.RoleCustomer: true}}})
	if _, err := service.Create(context.Background(), 30, "", "bad", "short"); !errors.Is(err, ErrInvalidSupportTicket) {
		t.Fatalf("invalid ticket returned %v", err)
	}
}

func TestSupportServiceProvidesSellerOwnedSupport(t *testing.T) {
	checker := roleChecker{roles: map[int64]map[domainidentity.Role]bool{
		41: {domainidentity.RoleSellerOwner: true},
		42: {domainidentity.RoleCustomer: true},
	}}
	service := NewSupportService(supportRepository{}, checker)
	if _, err := service.CreateSeller(context.Background(), 41, "WC-41", "Catalogue review", "Please explain the review status for this product."); err != nil {
		t.Fatalf("seller could not create a support ticket: %v", err)
	}
	if tickets, err := service.ListSeller(context.Background(), 41); err != nil || len(tickets) != 1 {
		t.Fatalf("seller could not list owned support tickets: tickets=%+v err=%v", tickets, err)
	}
	if _, err := service.ListSeller(context.Background(), 42); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("customer accessed seller support: %v", err)
	}
}

type supportReplyRecorder struct {
	called bool
	actor  int64
	ticket int64
	text   string
}

func (*supportReplyRecorder) CreateSupportTicket(context.Context, int64, string, string, string) (domaincommerce.SupportTicket, error) {
	return domaincommerce.SupportTicket{}, nil
}
func (*supportReplyRecorder) ListCustomerSupportTickets(context.Context, int64) ([]domaincommerce.SupportTicket, error) {
	return nil, nil
}
func (*supportReplyRecorder) ListSellerSupportTickets(context.Context, int64) ([]domaincommerce.SupportTicket, error) {
	return nil, nil
}
func (*supportReplyRecorder) ListSupportTickets(context.Context, int) ([]domaincommerce.SupportTicket, error) {
	return nil, nil
}
func (*supportReplyRecorder) UpdateSupportTicket(context.Context, int64, int64, string, string) error {
	return nil
}
func (r *supportReplyRecorder) ReplySupportTicket(_ context.Context, actorID, ticketID int64, text string) error {
	r.called, r.actor, r.ticket, r.text = true, actorID, ticketID, text
	return nil
}
func (r *supportReplyRecorder) ReplyCustomerSupportTicket(_ context.Context, actorID, ticketID int64, text string) error {
	r.called, r.actor, r.ticket, r.text = true, actorID, ticketID, text
	return nil
}

func TestSupportServiceRequiresAgentForCustomerReplies(t *testing.T) {
	repository := &supportReplyRecorder{}
	service := NewSupportService(repository, roleChecker{roles: map[int64]map[domainidentity.Role]bool{
		31: {domainidentity.RoleSupportAgent: true},
		30: {domainidentity.RoleCustomer: true},
		32: {domainidentity.RoleSuperAdmin: true},
	}})

	if err := service.Reply(context.Background(), 31, 9, "  We have arranged a replacement.  "); err != nil {
		t.Fatalf("support agent reply failed: %v", err)
	}
	if !repository.called || repository.actor != 31 || repository.ticket != 9 || repository.text != "We have arranged a replacement." {
		t.Fatalf("reply was not normalized and persisted: %+v", repository)
	}

	for _, userID := range []int64{30, 32} {
		if err := service.Reply(context.Background(), userID, 9, "A response"); !errors.Is(err, ports.ErrForbidden) {
			t.Errorf("non-agent %d could reply: %v", userID, err)
		}
	}
	for _, test := range []struct {
		name, text string
		ticketID   int64
	}{{name: "blank", text: "  ", ticketID: 9}, {name: "too long", text: strings.Repeat("x", 4001), ticketID: 9}, {name: "invalid ticket", text: "Response", ticketID: 0}} {
		t.Run(test.name, func(t *testing.T) {
			if err := service.Reply(context.Background(), 31, test.ticketID, test.text); !errors.Is(err, ErrInvalidSupportTicket) {
				t.Fatalf("invalid reply returned %v", err)
			}
		})
	}
}

func TestCustomerCanReplyOnlyThroughOwnedCustomerSupportFlow(t *testing.T) {
	repository := &supportReplyRecorder{}
	service := NewSupportService(repository, roleChecker{roles: map[int64]map[domainidentity.Role]bool{
		30: {domainidentity.RoleCustomer: true},
		31: {domainidentity.RoleSupportAgent: true},
	}})
	if err := service.ReplyCustomer(context.Background(), 30, 9, "  I have added the requested details.  "); err != nil {
		t.Fatalf("customer follow-up failed: %v", err)
	}
	if !repository.called || repository.actor != 30 || repository.ticket != 9 || repository.text != "I have added the requested details." {
		t.Fatalf("customer reply was not normalized and persisted: %+v", repository)
	}
	if err := service.ReplyCustomer(context.Background(), 31, 9, "I am the agent"); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("support agent used the customer reply action: %v", err)
	}
	for _, test := range []struct {
		name, text string
		ticketID   int64
	}{{name: "blank", text: "  ", ticketID: 9}, {name: "too long", text: strings.Repeat("x", 4001), ticketID: 9}, {name: "invalid ticket", text: "Details", ticketID: 0}} {
		t.Run(test.name, func(t *testing.T) {
			if err := service.ReplyCustomer(context.Background(), 30, test.ticketID, test.text); !errors.Is(err, ErrInvalidSupportTicket) {
				t.Fatalf("invalid customer reply returned %v", err)
			}
		})
	}
}
