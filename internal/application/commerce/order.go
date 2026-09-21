package commerce

import (
	"context"
	"errors"
	"regexp"
	"strings"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

var ErrInvalidAddress = errors.New("shipping address is invalid")

var indiaPostalCode = regexp.MustCompile(`^[1-9][0-9]{5}$`)
var phoneNumber = regexp.MustCompile(`^\+?[0-9][0-9 ()-]{5,22}$`)

type OrderService struct {
	repository ports.OrderRepository
}

func NewOrderService(repository ports.OrderRepository) *OrderService {
	return &OrderService{repository: repository}
}

func NormalizeAddressInput(address domaincommerce.AddressInput) (domaincommerce.AddressInput, error) {
	address.RecipientName = strings.TrimSpace(address.RecipientName)
	address.Phone = strings.TrimSpace(address.Phone)
	address.Line1 = strings.TrimSpace(address.Line1)
	address.Line2 = strings.TrimSpace(address.Line2)
	address.City = strings.TrimSpace(address.City)
	address.State = strings.TrimSpace(address.State)
	address.PostalCode = strings.TrimSpace(address.PostalCode)
	address.CountryCode = strings.ToUpper(strings.TrimSpace(address.CountryCode))
	if address.CountryCode == "" {
		address.CountryCode = "IN"
	}
	if address.RecipientName == "" || address.Line1 == "" || address.City == "" || address.State == "" {
		return domaincommerce.AddressInput{}, ErrInvalidAddress
	}
	if len([]rune(address.RecipientName)) > 100 || len([]rune(address.Line1)) > 250 || len([]rune(address.Line2)) > 250 || len([]rune(address.City)) > 100 || len([]rune(address.State)) > 100 {
		return domaincommerce.AddressInput{}, ErrInvalidAddress
	}
	if address.Phone != "" {
		digits := strings.Map(func(r rune) rune {
			if r < '0' || r > '9' {
				return -1
			}
			return r
		}, address.Phone)
		if !phoneNumber.MatchString(address.Phone) || len(digits) < 7 || len(digits) > 15 {
			return domaincommerce.AddressInput{}, ErrInvalidAddress
		}
	}
	if address.CountryCode != "IN" || !indiaPostalCode.MatchString(address.PostalCode) {
		return domaincommerce.AddressInput{}, ErrInvalidAddress
	}
	return address, nil
}

func (s *OrderService) Create(ctx context.Context, userID, cartID int64, idempotencyKey string, address domaincommerce.AddressInput) (domaincommerce.Order, error) {
	if userID <= 0 || cartID <= 0 || len(strings.TrimSpace(idempotencyKey)) < 16 || len(idempotencyKey) > 200 {
		return domaincommerce.Order{}, ports.ErrForbidden
	}
	var err error
	address, err = NormalizeAddressInput(address)
	if err != nil {
		return domaincommerce.Order{}, err
	}
	return s.repository.CreateOrder(ctx, userID, cartID, idempotencyKey, address)
}

func (s *OrderService) List(ctx context.Context, userID int64, limit int) ([]domaincommerce.Order, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repository.ListOrders(ctx, userID, limit)
}

func (s *OrderService) Get(ctx context.Context, userID int64, orderNumber string) (domaincommerce.OrderDetail, error) {
	if userID <= 0 || strings.TrimSpace(orderNumber) == "" {
		return domaincommerce.OrderDetail{}, ports.ErrOrderNotFound
	}
	return s.repository.GetOrder(ctx, userID, orderNumber)
}

func (s *OrderService) Cancel(ctx context.Context, userID int64, orderNumber string) error {
	if userID <= 0 || strings.TrimSpace(orderNumber) == "" {
		return ports.ErrOrderNotFound
	}
	return s.repository.CancelOrder(ctx, userID, orderNumber)
}
