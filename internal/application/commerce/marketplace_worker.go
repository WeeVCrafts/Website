package commerce

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

// MarketplaceProcessor executes only durable, claimed jobs. Every provider POST
// is preceded by a persisted intent; uncertain operations recover by read first.
type MarketplaceProcessor struct {
	Store           ports.MarketplaceStore
	Shipping        ports.ShippingGateway
	Refunds         ports.PaymentRefundGateway
	BookingsEnabled bool
	Handlers        map[string]func(context.Context) error
	NotifyEmail     func(context.Context, int64) error
}

func (p *MarketplaceProcessor) Process(ctx context.Context, id int64) error {
	job, claimed, err := p.Store.ClaimJob(ctx, id)
	if err != nil || !claimed {
		return err
	}
	runCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	switch job.Kind {
	case "book_package":
		err = p.book(runCtx, job.ResourceID)
	case "return_pickup":
		err = p.returnPickup(runCtx, job.ResourceID)
	case "cancel_items":
		err = p.cancelItems(runCtx, job.ResourceID)
	case "refund":
		err = p.refund(runCtx, job.ResourceID)
	case "notify_sellers":
		err = p.Store.NotifySellers(runCtx, job.ResourceID)
	case "transactional_email":
		if p.NotifyEmail != nil {
			err = p.NotifyEmail(runCtx, job.ResourceID)
		}
	case "invoice":
		err = p.Store.IssueInvoices(runCtx, job.ResourceID)
	case "credit_note":
		err = p.Store.IssueCreditNote(runCtx, job.ResourceID)
	case "claim_credit_note":
		if store, ok := p.Store.(interface {
			IssueClaimCreditNote(context.Context, int64) error
		}); ok {
			err = store.IssueClaimCreditNote(runCtx, job.ResourceID)
		} else {
			err = ports.ErrInvalidState
		}
	case "ndr":
		err = p.ndr(runCtx, job.ResourceID)
	default:
		if handler := p.Handlers[job.Kind]; handler != nil {
			err = handler(runCtx)
		} else {
			err = ports.ErrInvalidState
		}
	}
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer finishCancel()
	return errors.Join(err, p.Store.FinishJob(finishCtx, job, err))
}

func (p *MarketplaceProcessor) returnPickup(ctx context.Context, id int64) error {
	store, ok := p.Store.(ports.ReturnShippingStore)
	gateway, supported := p.Shipping.(ports.ReturnShippingGateway)
	if !ok || !supported {
		return ports.ErrShippingUnavailable
	}
	order, err := store.ReturnShippingOrder(ctx, id)
	if err != nil {
		return err
	}
	var shipment ports.Shipment
	if order.ProviderOrderID > 0 {
		shipment, err = p.Shipping.Status(ctx, order.Shipment)
	} else if order.PendingAction != "" {
		shipment, err = gateway.FindReturn(ctx, order.Reference)
	} else {
		if !p.BookingsEnabled {
			return ports.ErrShippingUnavailable
		}
		if err = store.StartReturnShipping(ctx, id); err != nil {
			return err
		}
		shipment, err = gateway.CreateReturn(ctx, order)
		if errors.Is(err, ports.ErrShippingRejected) || errors.Is(err, ports.ErrShippingUnavailable) || errors.Is(err, ports.ErrShippingInput) {
			confirm, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			err = errors.Join(err, store.RejectReturnShipping(confirm, id))
		}
	}
	if err != nil {
		return err
	}
	return store.SaveReturnShipping(ctx, id, shipment)
}

func (p *MarketplaceProcessor) ndr(ctx context.Context, id int64) error {
	gateway, ok := p.Shipping.(ports.NDRGateway)
	if !ok {
		return ports.ErrShippingUnavailable
	}
	s, action, date, comments, submit, err := p.Store.NDRAction(ctx, id)
	if err != nil {
		return err
	}
	if submit {
		if err = gateway.ActNDR(ctx, s, action, date, comments); err != nil {
			return err
		}
		return p.Store.ConfirmNDRAction(ctx, id)
	}
	fresh, err := p.Shipping.Status(ctx, s)
	if err != nil {
		return err
	}
	progressed := action == "return" && strings.HasPrefix(fresh.ProviderStatus, "RTO") || action == "re-attempt" && (fresh.ProviderStatus == "OUT FOR DELIVERY" || fresh.ProviderStatus == "DELIVERED" || fresh.ProviderStatus == "IN TRANSIT")
	if !progressed {
		return ports.ErrShippingPending
	}
	return p.Store.ConfirmNDRAction(ctx, id)
}
func (p *MarketplaceProcessor) book(ctx context.Context, id int64) error {
	if p.Shipping == nil {
		return ports.ErrShippingUnavailable
	}
	order, err := p.Store.ShippingOrder(ctx, id)
	if err != nil {
		return err
	}
	s := order.Shipment
	// Reconcile ambiguous create/assign/pickup before permitting any new POST.
	if s.PendingAction != "" || s.ProviderOrderID > 0 {
		var fresh ports.Shipment
		if s.ProviderOrderID == 0 {
			fresh, err = p.Shipping.Find(ctx, s.Reference)
		} else {
			fresh, err = p.Shipping.Status(ctx, s)
		}
		if err != nil {
			return err
		}
		if fresh.Reference != s.Reference || fresh.ProviderOrderID <= 0 || fresh.ProviderShipmentID <= 0 {
			return ports.ErrShippingPending
		}
		complete := s.PendingAction == "" || s.PendingAction == "create" || (s.PendingAction == "assign" && fresh.AWB != "") || (s.PendingAction == "pickup" && fresh.PickupReference != "") || s.PendingAction == "label" || s.PendingAction == "manifest"
		if s.PendingAction == "cancel" {
			complete = ports.ShipmentCancelled(fresh.ProviderStatus)
		}
		action := s.PendingAction
		if complete {
			action = ""
		}
		if err = p.Store.SavePackage(ctx, id, fresh, action); err != nil {
			return err
		}
		if !complete {
			return ports.ErrShippingPending
		}
		order, err = p.Store.ShippingOrder(ctx, id)
		if err != nil {
			return err
		}
		s = order.Shipment
	}
	if ports.ShipmentCancelled(s.ProviderStatus) || s.ProviderStatus == "DELIVERED" || strings.HasPrefix(s.ProviderStatus, "RTO") {
		return nil
	}
	if !p.BookingsEnabled {
		return nil
	} // Reconciliation continues while bookings are paused.
	stage, _ := domain.MapShipmentStatus(s.ProviderStatus)
	dispatched := stage == "picked_up" || stage == "in_transit" || stage == "out_for_delivery" || stage == "ndr"
	if dispatched && s.AWB == "" {
		return ports.ErrShippingPending
	}
	if s.ProviderOrderID == 0 {
		if err = p.Store.StartPackageAction(ctx, id, "create"); err != nil {
			return err
		}
		fresh, e := p.Shipping.Create(ctx, order)
		if e != nil {
			p.reject(ctx, id, "create", 0, e)
			return e
		}
		if err = p.Store.SavePackage(ctx, id, fresh, ""); err != nil {
			return err
		}
		s = fresh
	}
	if s.AWB == "" {
		courier := int64(0)
		if g, ok := p.Shipping.(ports.CourierAvailabilityGateway); ok {
			options, e := g.CourierOptions(ctx, s)
			if e != nil {
				return e
			}
			if len(options) == 0 {
				return ports.ErrShippingUnavailable
			}
			allowed := options[:0]
			for _, option := range options {
				rejected := false
				for _, id := range s.RejectedCourierIDs {
					if id == option.ID {
						rejected = true
						break
					}
				}
				if !rejected && (option.Recommended || option.RateCents > 0) {
					allowed = append(allowed, option)
				}
			}
			options = allowed
			if len(options) == 0 {
				return ports.ErrShippingUnavailable
			}
			sort.SliceStable(options, func(i, j int) bool {
				if options[i].Recommended != options[j].Recommended {
					return options[i].Recommended
				}
				if options[i].RateCents != options[j].RateCents {
					if options[i].RateCents <= 0 {
						return false
					}
					if options[j].RateCents <= 0 {
						return true
					}
					return options[i].RateCents < options[j].RateCents
				}
				return options[i].ID < options[j].ID
			})
			courier = options[0].ID
		}
		if err = p.Store.StartPackageAction(ctx, id, "assign"); err != nil {
			return err
		}
		fresh, e := p.Shipping.Assign(ctx, s, courier)
		if e != nil {
			p.reject(ctx, id, "assign", courier, e)
			return e
		}
		if err = p.Store.SavePackage(ctx, id, fresh, ""); err != nil {
			return err
		}
		s = fresh
	}
	for _, action := range []string{"label", "pickup", "manifest"} {
		if action == "pickup" && dispatched {
			continue
		}
		if action == "label" && s.LabelURL != "" || action == "pickup" && s.PickupReference != "" || action == "manifest" && s.ManifestURL != "" {
			continue
		}
		if err = p.Store.StartPackageAction(ctx, id, action); err != nil {
			return err
		}
		var fresh ports.Shipment
		switch action {
		case "label":
			fresh, err = p.Shipping.Label(ctx, s)
		case "pickup":
			fresh, err = p.Shipping.Pickup(ctx, s)
		case "manifest":
			fresh, err = p.Shipping.Manifest(ctx, s)
		}
		if err != nil {
			p.reject(ctx, id, action, 0, err)
			return err
		}
		if err = p.Store.SavePackage(ctx, id, fresh, ""); err != nil {
			return err
		}
		s = fresh
	}
	return nil
}

func (p *MarketplaceProcessor) reject(ctx context.Context, id int64, action string, courier int64, err error) {
	if !errors.Is(err, ports.ErrShippingRejected) && !errors.Is(err, ports.ErrShippingUnavailable) && !errors.Is(err, ports.ErrShippingInput) {
		return
	}
	if !errors.Is(err, ports.ErrShippingRejected) {
		courier = 0
	}
	confirm, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = p.Store.RejectPackageAction(confirm, id, action, courier)
}
func (p *MarketplaceProcessor) cancelItems(ctx context.Context, id int64) error {
	packages, err := p.Store.CancellationPackages(ctx, id)
	if err != nil {
		return err
	}
	for _, parcel := range packages {
		if parcel.ProviderOrderID == 0 && parcel.PendingAction == "" {
			continue
		}
		if p.Shipping == nil {
			return ports.ErrShippingUnavailable
		}
		g, ok := p.Shipping.(ports.ShipmentCancellationGateway)
		if !ok {
			return ports.ErrShippingUnavailable
		}
		s := parcel.Shipment
		if s.ProviderOrderID == 0 {
			if s.PendingAction == "" {
				continue
			}
			s, err = p.Shipping.Find(ctx, s.Reference)
		} else {
			s, err = p.Shipping.Status(ctx, s)
		}
		if err != nil {
			return err
		}
		if s.Reference != parcel.Reference || s.ProviderOrderID <= 0 {
			return ports.ErrShippingPending
		}
		if !ports.ShipmentCancelled(s.ProviderStatus) {
			if !ports.ShippingPrePickup(s.ProviderStatus) {
				return ports.ErrInvalidState
			}
			if parcel.PendingAction == "cancel" {
				return ports.ErrShippingPending
			}
			if err = p.Store.StartPackageAction(ctx, parcel.ID, "cancel"); err != nil {
				return err
			}
			if err = g.Cancel(ctx, s); err != nil {
				return err
			}
			s, err = p.Shipping.Status(ctx, s)
			if err != nil {
				return err
			}
			if !ports.ShipmentCancelled(s.ProviderStatus) {
				return ports.ErrShippingPending
			}
		}
		if err = p.Store.SavePackage(ctx, parcel.ID, s, ""); err != nil {
			return err
		}
	}
	return p.Store.CompleteCancellation(ctx, id)
}
func (p *MarketplaceProcessor) refund(ctx context.Context, id int64) error {
	record, submit, err := p.Store.RefundRecord(ctx, id)
	if err != nil {
		return err
	}
	if record.Status == "processed" {
		return nil
	}
	if record.AmountCents == 0 {
		return p.Store.SavePartialRefund(ctx, id, ports.PaymentRefund{ProviderPaymentID: record.ProviderPaymentID, AmountCents: 0, Currency: record.Currency, Status: "processed"})
	}
	if p.Refunds == nil {
		return ports.ErrRefundState
	}
	var result ports.PaymentRefund
	if submit {
		result, err = p.Refunds.CreateRefund(ctx, ports.PaymentRefundRequest{ProviderPaymentID: record.ProviderPaymentID, AmountCents: record.AmountCents, Currency: record.Currency, Receipt: record.Receipt})
	} else {
		g, ok := p.Refunds.(ports.RefundRecoveryGateway)
		if !ok {
			return ports.ErrRefundState
		}
		result, err = g.LookupRefund(ctx, record)
	}
	if err != nil {
		return fmt.Errorf("refund confirmation pending: %w", err)
	}
	if err = p.Store.SavePartialRefund(ctx, id, result); err != nil {
		return err
	}
	if result.Status != "processed" {
		return ports.ErrShippingPending
	}
	return nil
}
