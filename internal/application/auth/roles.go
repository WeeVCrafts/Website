package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

var ErrInvalidRoleAssignment = errors.New("role assignment request is invalid")

var RoleAdminAssignableRoles = []domainidentity.Role{
	domainidentity.RoleCustomer,
	domainidentity.RoleSellerOwner,
	domainidentity.RoleSellerStaff,
	domainidentity.RoleMarketplaceAdmin,
	domainidentity.RoleSupportAgent,
	domainidentity.RoleFinanceOperator,
	domainidentity.RoleOperations,
}

type RoleAdminService struct {
	repository ports.RoleAdministrationRepository
	roles      ports.RoleChecker
	hasher     ports.PasswordHasher
}

func NewRoleAdminService(repository ports.RoleAdministrationRepository, roles ports.RoleChecker, hashers ...ports.PasswordHasher) *RoleAdminService {
	service := &RoleAdminService{repository: repository, roles: roles}
	if len(hashers) > 0 {
		service.hasher = hashers[0]
	}
	return service
}

func (s *RoleAdminService) List(ctx context.Context, actorID int64) ([]domainidentity.RoleAssignment, error) {
	if !s.allowed(ctx, actorID) {
		return nil, ports.ErrForbidden
	}
	return s.repository.ListRoleAssignments(ctx, actorID)
}

func (s *RoleAdminService) Update(ctx context.Context, actorID, userID, sellerID int64, role string, grant bool, reason string) error {
	if !s.allowed(ctx, actorID) {
		return ports.ErrForbidden
	}
	roleValue := domainidentity.Role(strings.ToLower(strings.TrimSpace(role)))
	reason = strings.TrimSpace(reason)
	if actorID <= 0 || userID <= 0 || len(reason) < 4 || len(reason) > 1000 || !assignableRole(roleValue) || roleValue == domainidentity.RoleSellerStaff && sellerID <= 0 || roleValue != domainidentity.RoleSellerStaff && sellerID != 0 {
		return ErrInvalidRoleAssignment
	}
	if actorID == userID && roleValue == domainidentity.RoleSuperAdmin {
		return ports.ErrRoleState
	}
	return s.repository.UpdateRoleAssignment(ctx, actorID, userID, sellerID, roleValue, grant, reason)
}

func (s *RoleAdminService) CreateUser(ctx context.Context, actorID int64, email, displayName, password, role string, sellerID int64, reason string) error {
	if !s.allowed(ctx, actorID) {
		return ports.ErrForbidden
	}
	if s.hasher == nil {
		return ErrInvalidRoleAssignment
	}
	email = strings.ToLower(strings.TrimSpace(email))
	displayName = strings.TrimSpace(displayName)
	reason = strings.TrimSpace(reason)
	parsedEmail, emailErr := mail.ParseAddress(email)
	roleValue := domainidentity.Role(strings.ToLower(strings.TrimSpace(role)))
	if emailErr != nil || parsedEmail.Address != email || len(email) > 320 || len(displayName) < 2 || len(displayName) > 120 || len(password) < 12 || len(password) > 1024 || len(reason) < 4 || len(reason) > 1000 || !assignableRole(roleValue) || roleValue == domainidentity.RoleSellerStaff && sellerID <= 0 || roleValue != domainidentity.RoleSellerStaff && sellerID != 0 {
		return ErrInvalidRoleAssignment
	}
	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	_, err = s.repository.CreateRoleUser(ctx, actorID, email, displayName, passwordHash, roleValue, sellerID, reason)
	return err
}

func (s *RoleAdminService) DeleteUser(ctx context.Context, actorID, userID int64, reason string) error {
	if !s.allowed(ctx, actorID) {
		return ports.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if actorID <= 0 || userID <= 0 || actorID == userID || len(reason) < 4 || len(reason) > 1000 {
		return ErrInvalidRoleAssignment
	}
	return s.repository.DeleteRoleUser(ctx, actorID, userID, reason)
}

func (s *RoleAdminService) allowed(ctx context.Context, actorID int64) bool {
	if actorID <= 0 || s.repository == nil || s.roles == nil {
		return false
	}
	ok, err := s.roles.HasAnyRole(ctx, actorID, domainidentity.RoleSuperAdmin)
	return err == nil && ok
}

func assignableRole(role domainidentity.Role) bool {
	for _, allowed := range RoleAdminAssignableRoles {
		if role == allowed {
			return true
		}
	}
	return false
}
