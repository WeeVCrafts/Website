package commerce

import (
	"context"
	"errors"
	"testing"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

type catalogRepository struct{}

type roleChecker struct {
	roles       map[int64]map[domainidentity.Role]bool
	permissions map[int64]map[string]bool
}

func (r roleChecker) HasAnyRole(_ context.Context, userID int64, roles ...domainidentity.Role) (bool, error) {
	for _, role := range roles {
		if r.roles[userID][role] {
			return true, nil
		}
	}
	return false, nil
}

func (r roleChecker) HasSellerPermission(_ context.Context, userID int64, permission string) (bool, error) {
	return r.permissions[userID][permission], nil
}

func (catalogRepository) EnsureSeller(context.Context, int64, string) (domaincommerce.Seller, error) {
	return domaincommerce.Seller{ID: 1}, nil
}
func (catalogRepository) GetSeller(context.Context, int64) (domaincommerce.Seller, error) {
	return domaincommerce.Seller{ID: 1, Status: "active"}, nil
}
func (catalogRepository) UpdateSellerProfile(context.Context, int64, string, string) error {
	return nil
}
func (catalogRepository) CreateDraft(context.Context, int64, domaincommerce.ProductDraftInput) (domaincommerce.ManagedProduct, error) {
	return domaincommerce.ManagedProduct{ID: 1}, nil
}
func (catalogRepository) GetSellerProduct(context.Context, int64, int64) (domaincommerce.ManagedProduct, error) {
	return domaincommerce.ManagedProduct{ID: 1, Status: "draft"}, nil
}
func (catalogRepository) UpdateDraft(context.Context, int64, int64, domaincommerce.ProductDraftInput) (domaincommerce.ManagedProduct, error) {
	return domaincommerce.ManagedProduct{ID: 1, Status: "draft"}, nil
}
func (catalogRepository) CreateAdminProduct(context.Context, int64, int64, domaincommerce.ProductDraftInput) (domaincommerce.ManagedProduct, error) {
	return domaincommerce.ManagedProduct{ID: 2, Status: "draft"}, nil
}
func (catalogRepository) GetAdminProduct(context.Context, int64, int64) (domaincommerce.ManagedProduct, error) {
	return domaincommerce.ManagedProduct{ID: 1, Status: "approved"}, nil
}
func (catalogRepository) UpdateAdminProduct(context.Context, int64, int64, domaincommerce.ProductDraftInput) (domaincommerce.ManagedProduct, error) {
	return domaincommerce.ManagedProduct{ID: 1, Status: "approved"}, nil
}
func (catalogRepository) ListSellerProductVariants(context.Context, int64, int64) ([]domaincommerce.ProductVariant, error) {
	return nil, nil
}
func (catalogRepository) CreateSellerProductVariant(context.Context, int64, int64, domaincommerce.ProductVariantInput) (domaincommerce.ProductVariant, error) {
	return domaincommerce.ProductVariant{ID: 1, Status: "active"}, nil
}
func (catalogRepository) UpdateSellerProductVariant(context.Context, int64, int64, int64, domaincommerce.ProductVariantInput) (domaincommerce.ProductVariant, error) {
	return domaincommerce.ProductVariant{ID: 1, Status: "active"}, nil
}
func (catalogRepository) ArchiveSellerProductVariant(context.Context, int64, int64, int64, string) error {
	return nil
}
func (catalogRepository) ListAdminProductVariants(context.Context, int64, int64) ([]domaincommerce.ProductVariant, error) {
	return nil, nil
}
func (catalogRepository) CreateAdminProductVariant(context.Context, int64, int64, domaincommerce.ProductVariantInput) (domaincommerce.ProductVariant, error) {
	return domaincommerce.ProductVariant{ID: 2, Status: "active"}, nil
}
func (catalogRepository) UpdateAdminProductVariant(context.Context, int64, int64, int64, domaincommerce.ProductVariantInput) (domaincommerce.ProductVariant, error) {
	return domaincommerce.ProductVariant{ID: 2, Status: "active"}, nil
}
func (catalogRepository) ArchiveAdminProductVariant(context.Context, int64, int64, int64, string) error {
	return nil
}
func (catalogRepository) ArchiveSellerProduct(context.Context, int64, int64, string) error {
	return nil
}
func (catalogRepository) RestoreSellerProduct(context.Context, int64, int64, string) error {
	return nil
}
func (catalogRepository) ArchiveAdminProduct(context.Context, int64, int64, string) error { return nil }
func (catalogRepository) SubmitProduct(context.Context, int64, int64) error               { return nil }
func (catalogRepository) SubmitAdminProduct(context.Context, int64, int64) error          { return nil }
func (catalogRepository) RestoreAdminProduct(context.Context, int64, int64, string) error { return nil }
func (catalogRepository) ApproveProduct(context.Context, int64, int64, bool, string) error {
	return nil
}
func (catalogRepository) ListSellerProducts(context.Context, int64) ([]domaincommerce.ManagedProduct, error) {
	return nil, nil
}

func TestCatalogServiceAppliesRolePolicyBeforeRepository(t *testing.T) {
	checker := roleChecker{roles: map[int64]map[domainidentity.Role]bool{
		10: {domainidentity.RoleCustomer: true},
		11: {domainidentity.RoleSellerOwner: true},
		13: {domainidentity.RoleSellerStaff: true},
		12: {domainidentity.RoleMarketplaceAdmin: true},
	}, permissions: map[int64]map[string]bool{13: {domaincommerce.SellerPermissionProductWrite: true}}}
	service := NewCatalogServiceWithRoles(catalogRepository{}, checker)

	if _, err := service.CreateDraft(context.Background(), 10, validDraft()); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("customer created a draft without seller role: %v", err)
	}
	if _, err := service.CreateDraft(context.Background(), 11, validDraft()); err != nil {
		t.Fatalf("seller owner was denied draft creation: %v", err)
	}
	if _, err := service.CreateDraft(context.Background(), 13, validDraft()); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("product writer seeded stock without inventory permission: %v", err)
	}
	if err := service.ApproveProduct(context.Background(), 11, 1, true, ""); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("seller owner approved a product: %v", err)
	}
	if err := service.ApproveProduct(context.Background(), 12, 1, true, "reviewed"); err != nil {
		t.Fatalf("marketplace admin was denied approval: %v", err)
	}
}

func TestCatalogServiceRestrictsAdministratorProductManagementToSuperAdmin(t *testing.T) {
	checker := roleChecker{roles: map[int64]map[domainidentity.Role]bool{
		12: {domainidentity.RoleMarketplaceAdmin: true},
		14: {domainidentity.RoleSuperAdmin: true},
	}, permissions: map[int64]map[string]bool{}}
	service := NewCatalogServiceWithRoles(catalogRepository{}, checker)
	if _, err := service.CreateAdminProduct(context.Background(), 12, 1, validDraft()); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("marketplace admin created an administrator product: %v", err)
	}
	if _, err := service.CreateAdminProduct(context.Background(), 14, 1, validDraft()); err != nil {
		t.Fatalf("super admin could not create a product: %v", err)
	}
	if _, err := service.UpdateAdminProduct(context.Background(), 12, 1, validDraft()); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("marketplace admin updated a product directly: %v", err)
	}
	if err := service.ArchiveAdminProduct(context.Background(), 12, 1, "remove listing"); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("marketplace admin archived a product: %v", err)
	}
	if err := service.ArchiveAdminProduct(context.Background(), 14, 1, "remove listing"); err != nil {
		t.Fatalf("super admin could not archive a product: %v", err)
	}
}

func TestCatalogServiceRequiresProductWriteToArchiveSellerProducts(t *testing.T) {
	checker := roleChecker{roles: map[int64]map[domainidentity.Role]bool{
		11: {domainidentity.RoleSellerOwner: true},
		12: {domainidentity.RoleSellerStaff: true},
	}, permissions: map[int64]map[string]bool{
		12: {domaincommerce.SellerPermissionProductRead: true},
	}}
	service := NewCatalogServiceWithRoles(catalogRepository{}, checker)
	if err := service.ArchiveSellerProduct(context.Background(), 12, 1, "remove listing"); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("read-only staff archived a product: %v", err)
	}
	if err := service.ArchiveSellerProduct(context.Background(), 11, 1, "remove listing"); err != nil {
		t.Fatalf("seller owner could not archive their product: %v", err)
	}
}

func validDraft() domaincommerce.ProductDraftInput {
	return domaincommerce.ProductDraftInput{
		Slug: "handmade-soap", BrandSlug: "earth-studio", BrandName: "Earth Studio",
		CategorySlug: "natural-care", CategoryName: "Natural Care", Name: "Neem Soap",
		Description: "Handmade botanical soap", PriceCents: 12900, SKU: "SOAP-001", InitialStock: 10,
	}
}
func (catalogRepository) ListPendingProducts(context.Context, int64) ([]domaincommerce.ManagedProduct, error) {
	return nil, nil
}

func TestCatalogServiceRejectsInvalidSlugsAndPrices(t *testing.T) {
	service := NewCatalogService(catalogRepository{})
	_, err := service.CreateDraft(context.Background(), 1, domaincommerce.ProductDraftInput{Name: "Soap", Slug: "Not Valid", BrandSlug: "brand", BrandName: "Brand", CategorySlug: "care", CategoryName: "Care", Description: "details", PriceCents: 100, SKU: "SKU", InitialStock: 1})
	if !errors.Is(err, ErrInvalidProduct) {
		t.Fatalf("expected invalid product, got %v", err)
	}
	compareAt := int64(99)
	_, err = service.CreateDraft(context.Background(), 1, domaincommerce.ProductDraftInput{Name: "Soap", Slug: "soap", BrandSlug: "brand", BrandName: "Brand", CategorySlug: "care", CategoryName: "Care", Description: "details", PriceCents: 100, CompareAtCents: &compareAt, SKU: "SKU", InitialStock: 1})
	if !errors.Is(err, ErrInvalidProduct) {
		t.Fatalf("expected invalid compare-at price, got %v", err)
	}
}

func TestCatalogServiceProtectsSellerDraftEditing(t *testing.T) {
	checker := roleChecker{roles: map[int64]map[domainidentity.Role]bool{
		11: {domainidentity.RoleSellerOwner: true},
		12: {domainidentity.RoleSellerStaff: true},
	}, permissions: map[int64]map[string]bool{
		12: {domaincommerce.SellerPermissionProductRead: true, domaincommerce.SellerPermissionProductWrite: true},
	}}
	service := NewCatalogServiceWithRoles(catalogRepository{}, checker)

	if _, err := service.GetSellerProduct(context.Background(), 12, 1); err != nil {
		t.Fatalf("seller staff with product-read permission was denied: %v", err)
	}
	if _, err := service.UpdateDraft(context.Background(), 12, 1, validDraft()); err != nil {
		t.Fatalf("seller staff with product-write permission was denied: %v", err)
	}
	checker.permissions[12][domaincommerce.SellerPermissionProductWrite] = false
	if _, err := service.UpdateDraft(context.Background(), 12, 1, validDraft()); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("seller staff without product-write permission edited a draft: %v", err)
	}
	if _, err := service.UpdateDraft(context.Background(), 11, 1, validDraft()); err != nil {
		t.Fatalf("seller owner was denied draft edit: %v", err)
	}
	if _, err := service.UpdateDraft(context.Background(), 11, 1, domaincommerce.ProductDraftInput{Slug: "invalid slug"}); !errors.Is(err, ErrInvalidProduct) {
		t.Fatalf("invalid draft edit returned %v", err)
	}
}

func TestCatalogServiceProtectsSellerVariantManagementAndValidatesFields(t *testing.T) {
	checker := roleChecker{roles: map[int64]map[domainidentity.Role]bool{
		11: {domainidentity.RoleSellerOwner: true},
		12: {domainidentity.RoleSellerStaff: true},
	}, permissions: map[int64]map[string]bool{
		12: {domaincommerce.SellerPermissionProductRead: true},
	}}
	service := NewCatalogServiceWithRoles(catalogRepository{}, checker)
	input := domaincommerce.ProductVariantInput{Name: "Peacock Blue", SKU: "blue-001", PriceCents: 2000, InitialStock: 3}
	if _, err := service.CreateSellerProductVariant(context.Background(), 12, 5, input); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("read-only staff created a variant: %v", err)
	}
	checker.permissions[12][domaincommerce.SellerPermissionProductWrite] = true
	if _, err := service.CreateSellerProductVariant(context.Background(), 12, 5, input); err != nil {
		t.Fatalf("product writer could not create a variant: %v", err)
	}
	if _, err := service.UpdateSellerProductVariant(context.Background(), 12, 5, 1, input); err != nil {
		t.Fatalf("product writer could not update a variant: %v", err)
	}
	if err := service.ArchiveSellerProductVariant(context.Background(), 12, 5, 1, "retire blue option"); err != nil {
		t.Fatalf("product writer could not archive a variant: %v", err)
	}
	input.InitialStock = -1
	if _, err := service.CreateSellerProductVariant(context.Background(), 11, 5, input); !errors.Is(err, ErrInvalidProductVariant) {
		t.Fatalf("negative initial stock passed validation: %v", err)
	}
	input.InitialStock, input.PriceCents = 0, 2000
	compare := int64(1999)
	input.CompareAtCents = &compare
	if _, err := service.CreateSellerProductVariant(context.Background(), 11, 5, input); !errors.Is(err, ErrInvalidProductVariant) {
		t.Fatalf("compare price under sale price passed validation: %v", err)
	}
}

func TestCatalogServiceRestrictsAdminVariantManagementToSuperAdmin(t *testing.T) {
	checker := roleChecker{roles: map[int64]map[domainidentity.Role]bool{
		12: {domainidentity.RoleMarketplaceAdmin: true},
		14: {domainidentity.RoleSuperAdmin: true},
	}, permissions: map[int64]map[string]bool{}}
	service := NewCatalogServiceWithRoles(catalogRepository{}, checker)
	input := domaincommerce.ProductVariantInput{Name: "Small", SKU: "tee-small", PriceCents: 2500}
	if _, err := service.CreateAdminProductVariant(context.Background(), 12, 5, input); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("marketplace admin created a direct product variant: %v", err)
	}
	if _, err := service.CreateAdminProductVariant(context.Background(), 14, 5, input); err != nil {
		t.Fatalf("super admin was denied variant creation: %v", err)
	}
	if _, err := service.AdminProductVariants(context.Background(), 12, 5); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("marketplace admin read direct product variants: %v", err)
	}
}

func TestCatalogServiceRequiresActorForPendingProducts(t *testing.T) {
	service := NewCatalogService(catalogRepository{})
	if _, err := service.PendingProducts(context.Background(), 0); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("expected forbidden pending-product read, got %v", err)
	}
}

type orderRepository struct{}

func (orderRepository) CreateOrder(context.Context, int64, int64, string, domaincommerce.AddressInput) (domaincommerce.Order, error) {
	return domaincommerce.Order{}, nil
}
func (orderRepository) ListOrders(context.Context, int64, int) ([]domaincommerce.Order, error) {
	return nil, nil
}
func (orderRepository) GetOrder(context.Context, int64, string) (domaincommerce.OrderDetail, error) {
	return domaincommerce.OrderDetail{}, nil
}
func (orderRepository) CancelOrder(context.Context, int64, string) error { return nil }

func TestOrderServiceRequiresStrongIdempotencyKeyAndAddress(t *testing.T) {
	service := NewOrderService(orderRepository{})
	_, err := service.Create(context.Background(), 1, 1, "short", domaincommerce.AddressInput{RecipientName: "A", Line1: "B", City: "C", State: "D", PostalCode: "E"})
	if !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("expected key validation, got %v", err)
	}
	_, err = service.Create(context.Background(), 1, 1, "idempotency-key-1234", domaincommerce.AddressInput{RecipientName: "", Line1: "B", City: "C", State: "D", PostalCode: "E"})
	if !errors.Is(err, ErrInvalidAddress) {
		t.Fatalf("expected address validation, got %v", err)
	}
}

func TestCartServiceRequiresAuthenticatedWishlistOwner(t *testing.T) {
	service := NewCartService(nil)
	if err := service.AddWishlist(context.Background(), 0, 1); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("expected forbidden wishlist write, got %v", err)
	}
	if _, err := service.ListWishlist(context.Background(), 0); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("expected forbidden wishlist read, got %v", err)
	}
}
