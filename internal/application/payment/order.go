package payment

import (
	"context"
	"errors"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

var ErrPaymentCreationFailed = errors.New("payment order could not be created")
var ErrPaymentAmountTooSmall = errors.New("payment amount is below the provider minimum")

type OrderService struct {
	gateway    ports.PaymentGateway
	repository ports.PaymentOrderRepository
	observe    func(operation, outcome string)
}

func NewOrderService(gateway ports.PaymentGateway, repository ports.PaymentOrderRepository) *OrderService {
	return &OrderService{gateway: gateway, repository: repository}
}

func (s *OrderService) WithCreationObserver(observe func(operation, outcome string)) *OrderService {
	s.observe = observe
	return s
}

// Create prepares a provider order from the server-side order total. The
// browser never supplies or decides the amount or currency.
func (s *OrderService) Create(ctx context.Context, userID int64, orderNumber string) (domaincommerce.PaymentIntent, error) {
	if userID <= 0 || orderNumber == "" || s.gateway == nil || s.repository == nil {
		return domaincommerce.PaymentIntent{}, ports.ErrPaymentNotFound
	}
	record, err := s.repository.GetPaymentOrder(ctx, userID, orderNumber)
	if err != nil {
		return domaincommerce.PaymentIntent{}, err
	}
	if record.AmountCents < 100 {
		return domaincommerce.PaymentIntent{}, ErrPaymentAmountTooSmall
	}
	if record.Provider == "razorpay" && record.ProviderOrderID == "" && record.PaymentStatus == "created" {
		if gateway, ok := s.gateway.(ports.PaymentOrderLookupGateway); ok {
			repository, ok := s.repository.(ports.PaymentCreationRepository)
			if !ok {
				return domaincommerce.PaymentIntent{}, ErrPaymentCreationFailed
			}
			claim, err := repository.ClaimPaymentCreation(ctx, userID, orderNumber)
			if err != nil {
				return domaincommerce.PaymentIntent{}, err
			}
			if claim.Order.ProviderOrderID == "" {
				claimedOrder, err := s.completePaymentCreation(ctx, repository, gateway, claim)
				if err != nil {
					return domaincommerce.PaymentIntent{}, err
				}
				record = claimedOrder
			} else {
				record = claim.Order
			}
		}
	}
	if record.OrderStatus != "pending_payment" || (record.PaymentStatus != "created" && record.PaymentStatus != "pending") || record.Currency == "" {
		return domaincommerce.PaymentIntent{}, ports.ErrPaymentState
	}
	if record.ProviderOrderID != "" {
		// The provider may already have a payment even when its webhook has not arrived.
		gateway, canLookup := s.gateway.(ports.PaymentStatusGateway)
		webhooks, canRecord := s.repository.(ports.PaymentWebhookRepository)
		if canLookup && canRecord {
			if err := s.recoverProviderOrder(ctx, gateway, webhooks, record); err != nil {
				return domaincommerce.PaymentIntent{}, ports.ErrPaymentCreationPending
			}
		}
	}
	if payable, ok := s.repository.(ports.PaymentPayabilityRepository); ok {
		allowed, err := payable.PaymentOrderPayable(ctx, userID, orderNumber)
		if err != nil {
			return domaincommerce.PaymentIntent{}, err
		}
		if !allowed {
			return domaincommerce.PaymentIntent{}, ports.ErrPaymentState
		}
	}
	if record.ProviderOrderID != "" {
		return mapPaymentIntent(record), nil
	}
	if record.PaymentStatus != "created" {
		return domaincommerce.PaymentIntent{}, ports.ErrPaymentState
	}
	providerOrder, err := s.gateway.CreateOrder(ctx, ports.PaymentOrderRequest{
		Reference:   record.OrderNumber,
		AmountCents: record.AmountCents,
		Currency:    record.Currency,
	})
	if err != nil || providerOrder.ProviderOrderID == "" || providerOrder.AmountCents != record.AmountCents || providerOrder.Currency != record.Currency {
		return domaincommerce.PaymentIntent{}, ErrPaymentCreationFailed
	}
	attached, err := s.repository.AttachProviderOrder(ctx, userID, orderNumber, providerOrder)
	if err != nil {
		return domaincommerce.PaymentIntent{}, err
	}
	return mapPaymentIntent(attached), nil
}

func mapPaymentIntent(record ports.PaymentOrderRecord) domaincommerce.PaymentIntent {
	return domaincommerce.PaymentIntent{
		OrderID:         record.OrderID,
		OrderNumber:     record.OrderNumber,
		Provider:        record.Provider,
		ProviderOrderID: record.ProviderOrderID,
		Status:          record.PaymentStatus,
		AmountCents:     record.AmountCents,
		Currency:        record.Currency,
	}
}
