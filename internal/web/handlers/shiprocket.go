package handlers

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	application "github.com/wecratfs/commerce/internal/application/commerce"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func (h *LivePortalHandler) SetShipping(s *application.ShippingService) { h.shipping = s }

func (h *LivePortalHandler) loadShiprocket(r *http.Request, actor int64, detail *viewmodels.SellerAdminOrderDetail, canFulfill bool) {
	if h.shipping == nil {
		return
	}
	seller, err := strconv.ParseInt(detail.SellerID, 10, 64)
	if err != nil {
		return
	}
	shipment, err := h.shipping.Get(r.Context(), actor, seller, detail.Number)
	if err != nil && !errors.Is(err, ports.ErrOrderNotFound) {
		detail.Shiprocket = &viewmodels.SellerShiprocketView{Unavailable: true}
		return
	}
	pickup := h.shipping.PickupLocation(seller)
	if pickup == "" && shipment.Reference == "" {
		return
	}
	if shipment.PickupLocation != "" {
		pickup = shipment.PickupLocation
	}
	bookings := h.shipping.BookingsEnabled()
	managed := shipment.Reference != "" && !(ports.ShipmentCancelled(shipment.ProviderStatus) && shipment.ManualDispatchAllowed)
	var decision string
	if shipment.Reference == "" && bookings {
		if err := h.queryRow(r, `SELECT COALESCE((SELECT d.decision FROM order_delivery_decisions d JOIN orders o ON o.id=d.order_id WHERE o.order_number=$1 AND d.seller_id=$2),'')`, detail.Number, seller).Scan(&decision); err != nil {
			detail.Shiprocket = &viewmodels.SellerShiprocketView{Unavailable: true}
			return
		}
	}
	managed = managed || (decision == "shiprocket" && detail.Status == "packing")
	canManage := canFulfill && domain.PrepaidOrderConfirmed(detail.OrderStatus, detail.Payment, detail.PaymentReview)
	detail.Shiprocket = &viewmodels.SellerShiprocketView{
		ManualFallback: decision == "manual_fallback", Cancelled: ports.ShipmentCancelled(shipment.ProviderStatus), Sandbox: h.shipping.Environment() == "sandbox", PickupLocation: pickup, Reference: shipment.Reference,
		CanCreate: bookings && (decision == "" || decision == "shiprocket") && canManage && detail.Status == "packing" && shipment.Reference == "",
		Managed:   managed, CanManage: canManage && !ports.ShipmentCancelled(shipment.ProviderStatus), PendingAction: shipment.PendingAction, Status: shipment.ProviderStatus,
		AWB: shipment.AWB, Courier: shipment.Courier, PickupReference: shipment.PickupReference, LabelURL: shipment.LabelURL, ManifestURL: shipment.ManifestURL,
		CanPickup:        bookings && canManage && !ports.ShipmentCancelled(shipment.ProviderStatus),
		CanChooseCourier: bookings && canManage && !ports.ShipmentCancelled(shipment.ProviderStatus) && detail.Status == "packing" && shipment.ProviderOrderID > 0 && shipment.PendingAction == "" && shipment.AWB == "",
	}
	if !detail.Shiprocket.CanChooseCourier || r.URL.Query().Get("couriers") != "1" {
		return
	}
	detail.Shiprocket.CourierOptionsRequested = true
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	options, err := h.shipping.CourierOptions(ctx, shipment)
	if err != nil {
		detail.Shiprocket.CourierOptionsUnavailable = true
		return
	}
	for _, option := range options {
		detail.Shiprocket.CourierOptions = append(detail.Shiprocket.CourierOptions, viewmodels.SellerCourierOption{
			ID: strconv.FormatInt(option.ID, 10), Name: option.Name, Rate: option.EstimatedRate, Days: option.EstimatedDays,
		})
	}
}

func (h *LivePortalHandler) shiprocketAction(w http.ResponseWriter, r *http.Request, actor int64, csrf string) {
	w.Header().Set("Cache-Control", "private, no-store")
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.shipping == nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid shipment request", http.StatusBadRequest)
		return
	}
	if csrf == "" || subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")), []byte(csrf)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	fields := r.PostForm
	seller, _ := strconv.ParseInt(fields.Get("seller_id"), 10, 64)
	number := fields.Get("order_number")
	if !sellerOrderDetailPath("orders/"+number) || len(number) > 120 {
		http.Error(w, "invalid order number", http.StatusBadRequest)
		return
	}
	weight, _ := strconv.Atoi(fields.Get("weight_grams"))
	length, _ := strconv.Atoi(fields.Get("length_cm"))
	width, _ := strconv.Atoi(fields.Get("width_cm"))
	height, _ := strconv.Atoi(fields.Get("height_cm"))
	courier, courierErr := strconv.ParseInt(fields.Get("courier_id"), 10, 64)
	if fields.Get("courier_id") != "" && courierErr != nil {
		courier = -1
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	action := fields.Get("action")
	err := h.shipping.Act(ctx, actor, seller, number, action, ports.Parcel{WeightGrams: weight, LengthCM: length, WidthCM: width, HeightCM: height}, courier)
	var notice string
	switch {
	case errors.Is(err, ports.ErrOrderNotFound):
		http.NotFound(w, r)
		return
	case errors.Is(err, ports.ErrShippingInput):
		notice = "Check the parcel weight, dimensions or courier selection."
	case errors.Is(err, ports.ErrShippingPending):
		notice = "Waiting for Shiprocket to confirm. Check the shipment status again after reconciliation."
	case errors.Is(err, ports.ErrShippingRejected):
		notice = "Shiprocket declined this action. Check the parcel or courier in Shiprocket before trying again."
	case errors.Is(err, ports.ErrInvalidState):
		notice = "This shipment action is unavailable at the current order stage."
	case err != nil:
		notice = "Shiprocket is temporarily unavailable. Check the saved shipment before retrying."
	case action == "refresh":
		notice = "Shipment status checked."
	default:
		notice = "Shipment updated."
	}
	if err != nil && r.Header.Get("X-UI-Form") == "true" && !errors.Is(err, ports.ErrShippingPending) {
		status := http.StatusServiceUnavailable
		if errors.Is(err, ports.ErrShippingInput) || errors.Is(err, ports.ErrShippingRejected) {
			status = http.StatusUnprocessableEntity
		} else if errors.Is(err, ports.ErrInvalidState) {
			status = http.StatusConflict
		}
		http.Error(w, notice, status)
		return
	}
	query := url.Values{"notice": {notice}}
	if action == "assign" && err != nil && !errors.Is(err, ports.ErrShippingPending) && fields.Get("couriers") == "1" {
		query.Set("couriers", "1")
	}
	for _, pair := range [][2]string{{"q", "return_query"}, {"tab", "return_tab"}, {"page", "return_page"}} {
		if value := fields.Get(pair[1]); value != "" && len(value) <= 200 {
			query.Set(pair[0], value)
		}
	}
	http.Redirect(w, r, "/seller-admin/orders/"+url.PathEscape(number)+"?"+query.Encode(), http.StatusSeeOther)
}
