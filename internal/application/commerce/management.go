package commerce

import (
	"context"
	"net/mail"
	"net/url"
	"regexp"
	"strings"

	"github.com/wecratfs/commerce/internal/ports"
)

type ManagementService struct{ repository ports.ManagementRepository }

func NewManagementService(repository ports.ManagementRepository) *ManagementService {
	return &ManagementService{repository: repository}
}
func (s *ManagementService) List(ctx context.Context, actorID int64, section, query string, page int) (ports.ManagementResult, error) {
	if !managementSection(section) || actorID <= 0 {
		return ports.ManagementResult{}, ports.ErrForbidden
	}
	if page < 1 {
		page = 1
	}
	return s.repository.ListManagement(ctx, actorID, section, strings.TrimSpace(query), 25, (page-1)*25)
}
func managementSection(section string) bool {
	return section == "customers" || section == "brands" || section == "category-management" || section == "commission-rules" || section == "settings"
}

var managementSlug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func (s *ManagementService) Mutate(ctx context.Context, actorID int64, m ports.ManagementMutation) error {
	if actorID <= 0 || !managementSection(m.Section) || m.Section == "customers" {
		return ports.ErrForbidden
	}
	m.Name, m.ID, m.Category, m.Reason = strings.TrimSpace(m.Name), strings.TrimSpace(m.ID), strings.TrimSpace(m.Category), strings.TrimSpace(m.Reason)
	if len(m.Reason) < 4 || len(m.Reason) > 1000 {
		return ports.ErrManagementValidation
	}
	if m.Section == "settings" {
		if m.Action != "save" {
			return ports.ErrManagementValidation
		}
		m.Settings.SupportEmail = strings.TrimSpace(m.Settings.SupportEmail)
		m.Settings.ShippingInformation = strings.TrimSpace(m.Settings.ShippingInformation)
		for _, field := range []*string{&m.Settings.InstagramURL, &m.Settings.FacebookURL, &m.Settings.YouTubeURL, &m.Settings.XURL, &m.Settings.WhatsAppURL} {
			*field = strings.TrimSpace(*field)
			if *field == "" {
				continue
			}
			parsed, err := url.Parse(*field)
			if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || len(*field) > 2048 || strings.ContainsAny(*field, "\r\n\\") {
				return ports.ErrManagementValidation
			}
		}
		address, err := mail.ParseAddress(m.Settings.SupportEmail)
		if err != nil || address.Address != m.Settings.SupportEmail || len(m.Settings.SupportEmail) > 254 || len(m.Settings.ShippingInformation) < 10 || len(m.Settings.ShippingInformation) > 3000 {
			return ports.ErrManagementValidation
		}
	} else {
		if m.Action != "create" && m.Action != "update" && m.Action != "delete" {
			return ports.ErrManagementValidation
		}
		if m.Action != "delete" && (m.Name == "" || len(m.Name) > 120) {
			return ports.ErrManagementValidation
		}
		if m.Section == "commission-rules" {
			if m.BasisPoints < 0 || m.BasisPoints > 10000 {
				return ports.ErrManagementValidation
			}
		} else if len(m.ID) > 120 || !managementSlug.MatchString(m.ID) {
			return ports.ErrManagementValidation
		}
	}
	return s.repository.MutateManagement(ctx, actorID, m)
}
