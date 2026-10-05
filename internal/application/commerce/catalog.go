package commerce

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

var ErrInvalidProduct = errors.New("product details are invalid")
var ErrInvalidProductVariant = errors.New("product variant details are invalid")

type CatalogService struct {
	repository  ports.CatalogManagementRepository
	roles       ports.RoleChecker
	permissions ports.SellerPermissionChecker
}

func NewCatalogService(repository ports.CatalogManagementRepository) *CatalogService {
	return &CatalogService{repository: repository}
}

// NewCatalogServiceWithRoles is the production constructor. The legacy
// constructor remains available for small isolated unit fixtures that do not
// model identity storage; the composition root always supplies a RoleChecker.
func NewCatalogServiceWithRoles(repository ports.CatalogManagementRepository, roles ports.RoleChecker) *CatalogService {
	service := &CatalogService{repository: repository, roles: roles}
	if checker, ok := roles.(ports.SellerPermissionChecker); ok {
		service.permissions = checker
	}
	return service
}

func (s *CatalogService) EnsureSeller(ctx context.Context, ownerUserID int64, displayName string) (domaincommerce.Seller, error) {
	displayName = strings.TrimSpace(displayName)
	if ownerUserID <= 0 || len(displayName) < 2 || len(displayName) > 120 || !s.allowed(ctx, ownerUserID, domainidentity.RoleCustomer, domainidentity.RoleSellerOwner) {
		return domaincommerce.Seller{}, ports.ErrForbidden
	}
	return s.repository.EnsureSeller(ctx, ownerUserID, displayName)
}

func (s *CatalogService) SellerProfile(ctx context.Context, userID int64) (domaincommerce.Seller, error) {
	if userID <= 0 || s.repository == nil {
		return domaincommerce.Seller{}, ports.ErrForbidden
	}
	return s.repository.GetSeller(ctx, userID)
}

func (s *CatalogService) UpdateSellerProfile(ctx context.Context, ownerUserID int64, displayName, reason string) error {
	displayName, reason = strings.TrimSpace(displayName), strings.TrimSpace(reason)
	if ownerUserID <= 0 || !s.allowed(ctx, ownerUserID, domainidentity.RoleSellerOwner) {
		return ports.ErrForbidden
	}
	if len(displayName) < 2 || len(displayName) > 120 || len(reason) < 4 || len(reason) > 1000 {
		return ErrInvalidProduct
	}
	return s.repository.UpdateSellerProfile(ctx, ownerUserID, displayName, reason)
}

func (s *CatalogService) CreateDraft(ctx context.Context, ownerUserID int64, input domaincommerce.ProductDraftInput) (domaincommerce.ManagedProduct, error) {
	if !s.sellerAllowed(ctx, ownerUserID, domaincommerce.SellerPermissionProductWrite) {
		return domaincommerce.ManagedProduct{}, ports.ErrForbidden
	}
	input, err := normalizeDraft(input)
	if err != nil || ownerUserID <= 0 {
		return domaincommerce.ManagedProduct{}, ErrInvalidProduct
	}
	if input.InitialStock > 0 && !s.sellerAllowed(ctx, ownerUserID, domaincommerce.SellerPermissionInventoryWrite) {
		return domaincommerce.ManagedProduct{}, ports.ErrForbidden
	}
	product, err := s.repository.CreateDraft(ctx, ownerUserID, input)
	if err != nil {
		return domaincommerce.ManagedProduct{}, fmt.Errorf("create product draft: %w", err)
	}
	return product, nil
}

func (s *CatalogService) GetSellerProduct(ctx context.Context, ownerUserID, productID int64) (domaincommerce.ManagedProduct, error) {
	if productID <= 0 || !s.sellerAllowed(ctx, ownerUserID, domaincommerce.SellerPermissionProductRead) {
		return domaincommerce.ManagedProduct{}, ports.ErrForbidden
	}
	return s.repository.GetSellerProduct(ctx, ownerUserID, productID)
}

func (s *CatalogService) UpdateDraft(ctx context.Context, ownerUserID, productID int64, input domaincommerce.ProductDraftInput) (domaincommerce.ManagedProduct, error) {
	if productID <= 0 || !s.sellerAllowed(ctx, ownerUserID, domaincommerce.SellerPermissionProductWrite) {
		return domaincommerce.ManagedProduct{}, ports.ErrForbidden
	}
	var err error
	input, err = normalizeDraft(input)
	if err != nil {
		return domaincommerce.ManagedProduct{}, err
	}
	product, err := s.repository.UpdateDraft(ctx, ownerUserID, productID, input)
	if err != nil {
		return domaincommerce.ManagedProduct{}, fmt.Errorf("update product draft: %w", err)
	}
	return product, nil
}

func (s *CatalogService) CreateAdminProduct(ctx context.Context, actorID, sellerID int64, input domaincommerce.ProductDraftInput) (domaincommerce.ManagedProduct, error) {
	if s.roles == nil || !s.allowed(ctx, actorID, domainidentity.RoleSuperAdmin, domainidentity.RoleMarketplaceAdmin, domainidentity.RoleSellerOwner, domainidentity.RoleSellerStaff) || sellerID <= 0 {
		return domaincommerce.ManagedProduct{}, ports.ErrForbidden
	}
	input, err := normalizeDraft(input)
	if err != nil {
		return domaincommerce.ManagedProduct{}, err
	}
	return s.repository.CreateAdminProduct(ctx, actorID, sellerID, input)
}

func (s *CatalogService) GetAdminProduct(ctx context.Context, actorID, productID int64) (domaincommerce.ManagedProduct, error) {
	if s.roles == nil || !s.allowed(ctx, actorID, domainidentity.RoleSuperAdmin, domainidentity.RoleMarketplaceAdmin, domainidentity.RoleSellerOwner, domainidentity.RoleSellerStaff) || productID <= 0 {
		return domaincommerce.ManagedProduct{}, ports.ErrForbidden
	}
	return s.repository.GetAdminProduct(ctx, actorID, productID)
}

func (s *CatalogService) UpdateAdminProduct(ctx context.Context, actorID, productID int64, input domaincommerce.ProductDraftInput) (domaincommerce.ManagedProduct, error) {
	if s.roles == nil || !s.allowed(ctx, actorID, domainidentity.RoleSuperAdmin, domainidentity.RoleMarketplaceAdmin, domainidentity.RoleSellerOwner, domainidentity.RoleSellerStaff) || productID <= 0 {
		return domaincommerce.ManagedProduct{}, ports.ErrForbidden
	}
	input, err := normalizeDraft(input)
	if err != nil {
		return domaincommerce.ManagedProduct{}, err
	}
	return s.repository.UpdateAdminProduct(ctx, actorID, productID, input)
}

func (s *CatalogService) SellerProductVariants(ctx context.Context, ownerUserID, productID int64) ([]domaincommerce.ProductVariant, error) {
	if productID <= 0 || !s.sellerAllowed(ctx, ownerUserID, domaincommerce.SellerPermissionProductWrite) {
		return nil, ports.ErrForbidden
	}
	return s.repository.ListSellerProductVariants(ctx, ownerUserID, productID)
}

func (s *CatalogService) CreateSellerProductVariant(ctx context.Context, ownerUserID, productID int64, input domaincommerce.ProductVariantInput) (domaincommerce.ProductVariant, error) {
	if productID <= 0 || !s.sellerAllowed(ctx, ownerUserID, domaincommerce.SellerPermissionProductWrite) {
		return domaincommerce.ProductVariant{}, ports.ErrForbidden
	}
	input, err := normalizeVariant(input, true)
	if err != nil {
		return domaincommerce.ProductVariant{}, err
	}
	if input.InitialStock > 0 && !s.sellerAllowed(ctx, ownerUserID, domaincommerce.SellerPermissionInventoryWrite) {
		return domaincommerce.ProductVariant{}, ports.ErrForbidden
	}
	return s.repository.CreateSellerProductVariant(ctx, ownerUserID, productID, input)
}

func (s *CatalogService) UpdateSellerProductVariant(ctx context.Context, ownerUserID, productID, variantID int64, input domaincommerce.ProductVariantInput) (domaincommerce.ProductVariant, error) {
	if productID <= 0 || variantID <= 0 || !s.sellerAllowed(ctx, ownerUserID, domaincommerce.SellerPermissionProductWrite) {
		return domaincommerce.ProductVariant{}, ports.ErrForbidden
	}
	input, err := normalizeVariant(input, false)
	if err != nil {
		return domaincommerce.ProductVariant{}, err
	}
	return s.repository.UpdateSellerProductVariant(ctx, ownerUserID, productID, variantID, input)
}

func (s *CatalogService) ArchiveSellerProductVariant(ctx context.Context, ownerUserID, productID, variantID int64, reason string) error {
	if productID <= 0 || variantID <= 0 || !s.sellerAllowed(ctx, ownerUserID, domaincommerce.SellerPermissionProductWrite) {
		return ports.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if len(reason) < 4 || len(reason) > 1000 {
		return ErrInvalidProductVariant
	}
	return s.repository.ArchiveSellerProductVariant(ctx, ownerUserID, productID, variantID, reason)
}

func (s *CatalogService) AdminProductVariants(ctx context.Context, actorID, productID int64) ([]domaincommerce.ProductVariant, error) {
	if productID <= 0 || s.roles == nil || !s.allowed(ctx, actorID, domainidentity.RoleSuperAdmin, domainidentity.RoleMarketplaceAdmin, domainidentity.RoleSellerOwner, domainidentity.RoleSellerStaff) {
		return nil, ports.ErrForbidden
	}
	return s.repository.ListAdminProductVariants(ctx, actorID, productID)
}

func (s *CatalogService) CreateAdminProductVariant(ctx context.Context, actorID, productID int64, input domaincommerce.ProductVariantInput) (domaincommerce.ProductVariant, error) {
	if productID <= 0 || s.roles == nil || !s.allowed(ctx, actorID, domainidentity.RoleSuperAdmin, domainidentity.RoleMarketplaceAdmin, domainidentity.RoleSellerOwner, domainidentity.RoleSellerStaff) {
		return domaincommerce.ProductVariant{}, ports.ErrForbidden
	}
	input, err := normalizeVariant(input, true)
	if err != nil {
		return domaincommerce.ProductVariant{}, err
	}
	return s.repository.CreateAdminProductVariant(ctx, actorID, productID, input)
}

func (s *CatalogService) UpdateAdminProductVariant(ctx context.Context, actorID, productID, variantID int64, input domaincommerce.ProductVariantInput) (domaincommerce.ProductVariant, error) {
	if productID <= 0 || variantID <= 0 || s.roles == nil || !s.allowed(ctx, actorID, domainidentity.RoleSuperAdmin, domainidentity.RoleMarketplaceAdmin, domainidentity.RoleSellerOwner, domainidentity.RoleSellerStaff) {
		return domaincommerce.ProductVariant{}, ports.ErrForbidden
	}
	input, err := normalizeVariant(input, false)
	if err != nil {
		return domaincommerce.ProductVariant{}, err
	}
	return s.repository.UpdateAdminProductVariant(ctx, actorID, productID, variantID, input)
}

func (s *CatalogService) ArchiveAdminProductVariant(ctx context.Context, actorID, productID, variantID int64, reason string) error {
	if productID <= 0 || variantID <= 0 || s.roles == nil || !s.allowed(ctx, actorID, domainidentity.RoleSuperAdmin, domainidentity.RoleMarketplaceAdmin, domainidentity.RoleSellerOwner, domainidentity.RoleSellerStaff) {
		return ports.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if len(reason) < 4 || len(reason) > 1000 {
		return ErrInvalidProductVariant
	}
	return s.repository.ArchiveAdminProductVariant(ctx, actorID, productID, variantID, reason)
}

func (s *CatalogService) ArchiveSellerProduct(ctx context.Context, ownerUserID, productID int64, reason string) error {
	if productID <= 0 || !s.sellerAllowed(ctx, ownerUserID, domaincommerce.SellerPermissionProductWrite) {
		return ports.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if len(reason) < 4 || len(reason) > 1000 {
		return ErrInvalidProduct
	}
	return s.repository.ArchiveSellerProduct(ctx, ownerUserID, productID, reason)
}

func (s *CatalogService) RestoreSellerProduct(ctx context.Context, ownerUserID, productID int64, reason string) error {
	if productID <= 0 || !s.sellerAllowed(ctx, ownerUserID, domaincommerce.SellerPermissionProductWrite) {
		return ports.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if len(reason) < 4 || len(reason) > 1000 {
		return ErrInvalidProduct
	}
	return s.repository.RestoreSellerProduct(ctx, ownerUserID, productID, reason)
}

func (s *CatalogService) ArchiveAdminProduct(ctx context.Context, actorID, productID int64, reason string) error {
	if s.roles == nil || !s.allowed(ctx, actorID, domainidentity.RoleSuperAdmin, domainidentity.RoleMarketplaceAdmin, domainidentity.RoleSellerOwner, domainidentity.RoleSellerStaff) || productID <= 0 {
		return ports.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if len(reason) < 4 || len(reason) > 1000 {
		return ErrInvalidProduct
	}
	return s.repository.ArchiveAdminProduct(ctx, actorID, productID, reason)
}

func (s *CatalogService) SubmitProduct(ctx context.Context, ownerUserID, productID int64) error {
	if productID <= 0 || !s.sellerAllowed(ctx, ownerUserID, domaincommerce.SellerPermissionProductWrite) {
		return ports.ErrForbidden
	}
	return s.repository.SubmitProduct(ctx, ownerUserID, productID)
}

func (s *CatalogService) SubmitAdminProduct(ctx context.Context, actorID, productID int64) error {
	if productID <= 0 || s.roles == nil || !s.allowed(ctx, actorID, domainidentity.RoleSuperAdmin, domainidentity.RoleMarketplaceAdmin, domainidentity.RoleSellerOwner, domainidentity.RoleSellerStaff) {
		return ports.ErrForbidden
	}
	return s.repository.SubmitAdminProduct(ctx, actorID, productID)
}

func (s *CatalogService) RestoreAdminProduct(ctx context.Context, actorID, productID int64, reason string) error {
	if productID <= 0 || s.roles == nil || !s.allowed(ctx, actorID, domainidentity.RoleSuperAdmin, domainidentity.RoleMarketplaceAdmin, domainidentity.RoleSellerOwner, domainidentity.RoleSellerStaff) {
		return ports.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if len(reason) < 4 || len(reason) > 1000 {
		return ErrInvalidProduct
	}
	return s.repository.RestoreAdminProduct(ctx, actorID, productID, reason)
}

func (s *CatalogService) ApproveProduct(ctx context.Context, actorID, productID int64, approved bool, reason string) error {
	if productID <= 0 || !s.allowed(ctx, actorID, domainidentity.RoleMarketplaceAdmin, domainidentity.RoleSuperAdmin) {
		return ports.ErrForbidden
	}
	return s.repository.ApproveProduct(ctx, actorID, productID, approved, strings.TrimSpace(reason))
}

func (s *CatalogService) SellerProducts(ctx context.Context, ownerUserID int64) ([]domaincommerce.ManagedProduct, error) {
	if !s.allowed(ctx, ownerUserID, domainidentity.RoleCustomer, domainidentity.RoleSellerOwner) && !s.hasSellerPermission(ctx, ownerUserID, domaincommerce.SellerPermissionProductRead) {
		return nil, ports.ErrForbidden
	}
	return s.repository.ListSellerProducts(ctx, ownerUserID)
}

func (s *CatalogService) PendingProducts(ctx context.Context, actorID int64) ([]domaincommerce.ManagedProduct, error) {
	if !s.allowed(ctx, actorID, domainidentity.RoleMarketplaceAdmin, domainidentity.RoleSuperAdmin) {
		return nil, ports.ErrForbidden
	}
	return s.repository.ListPendingProducts(ctx, actorID)
}

func (s *CatalogService) allowed(ctx context.Context, userID int64, roles ...domainidentity.Role) bool {
	if userID <= 0 {
		return false
	}
	if s.roles == nil {
		return true
	}
	allowed, err := s.roles.HasAnyRole(ctx, userID, roles...)
	return err == nil && allowed
}

func (s *CatalogService) sellerAllowed(ctx context.Context, userID int64, permission string) bool {
	return s.allowed(ctx, userID, domainidentity.RoleSellerOwner) || s.hasSellerPermission(ctx, userID, permission)
}

func (s *CatalogService) hasSellerPermission(ctx context.Context, userID int64, permission string) bool {
	if userID <= 0 || s.permissions == nil {
		return false
	}
	allowed, err := s.permissions.HasSellerPermission(ctx, userID, permission)
	return err == nil && allowed
}

func validSlug(value string) bool {
	if len(value) < 2 || len(value) > 120 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}

func normalizeDraft(input domaincommerce.ProductDraftInput) (domaincommerce.ProductDraftInput, error) {
	input.Slug = strings.TrimSpace(strings.ToLower(input.Slug))
	input.BrandSlug = strings.TrimSpace(strings.ToLower(input.BrandSlug))
	input.BrandName = strings.TrimSpace(input.BrandName)
	input.CategorySlug = strings.TrimSpace(strings.ToLower(input.CategorySlug))
	input.CategoryName = strings.TrimSpace(input.CategoryName)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.SKU = strings.TrimSpace(strings.ToUpper(input.SKU))
	input.DeliveryLabel = strings.TrimSpace(input.DeliveryLabel)
	input.ImageURL = strings.TrimSpace(input.ImageURL)
	if input.DeliveryLabel == "" {
		input.DeliveryLabel = "Dispatch details at checkout"
	}
	if !validSlug(input.Slug) || !validSlug(input.BrandSlug) || !validSlug(input.CategorySlug) ||
		!withinTextLimit(input.BrandName, 120) || !withinTextLimit(input.CategoryName, 120) ||
		!withinTextLimit(input.Name, 180) || !withinTextLimit(input.Description, 10000) ||
		!withinTextLimit(input.SKU, 120) || !withinTextLimit(input.DeliveryLabel, 180) ||
		input.PriceCents <= 0 || input.InitialStock < 0 || input.InitialStock > 100000 {
		return domaincommerce.ProductDraftInput{}, ErrInvalidProduct
	}
	if input.CompareAtCents != nil && *input.CompareAtCents < input.PriceCents {
		return domaincommerce.ProductDraftInput{}, ErrInvalidProduct
	}
	if input.CostPriceCents != nil && *input.CostPriceCents < 0 {
		return domaincommerce.ProductDraftInput{}, ErrInvalidProduct
	}
	return input, nil
}

func withinTextLimit(value string, max int) bool {
	length := utf8.RuneCountInString(value)
	return length > 0 && length <= max
}

func normalizeVariant(input domaincommerce.ProductVariantInput, includeInitialStock bool) (domaincommerce.ProductVariantInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.SKU = strings.TrimSpace(strings.ToUpper(input.SKU))
	if len(input.Name) < 1 || len(input.Name) > 120 || len(input.SKU) < 1 || len(input.SKU) > 120 || input.PriceCents <= 0 {
		return domaincommerce.ProductVariantInput{}, ErrInvalidProductVariant
	}
	if input.CompareAtCents != nil && *input.CompareAtCents < input.PriceCents {
		return domaincommerce.ProductVariantInput{}, ErrInvalidProductVariant
	}
	if input.CostPriceCents != nil && *input.CostPriceCents < 0 {
		return domaincommerce.ProductVariantInput{}, ErrInvalidProductVariant
	}
	if includeInitialStock {
		if input.InitialStock < 0 || input.InitialStock > 100000 {
			return domaincommerce.ProductVariantInput{}, ErrInvalidProductVariant
		}
	} else {
		input.InitialStock = 0
	}
	return input, nil
}
