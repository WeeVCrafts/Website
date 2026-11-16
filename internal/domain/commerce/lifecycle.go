package commerce

import (
	"errors"
	"strings"
)

var ErrTransition = errors.New("invalid lifecycle transition")

// Lifecycle transitions are independent. Provider exception states may recover,
// but terminal delivery/cancellation cannot be reversed by an older scan.
func CanTransition(system, from, to string) bool {
	graphs := map[string]map[string][]string{
		"seller":     {"new": {"accepted", "cancelled"}, "accepted": {"packing", "cancelled"}, "packing": {"ready_to_ship", "cancelled"}, "ready_to_ship": {"shipped", "cancelled"}, "shipped": {"delivered"}},
		"shipment":   {"created": {"pickup_scheduled", "picked_up", "in_transit", "out_for_delivery", "delivered", "ndr", "rto", "cancelled"}, "pickup_scheduled": {"picked_up", "in_transit", "out_for_delivery", "delivered", "ndr", "rto", "cancelled"}, "picked_up": {"in_transit", "out_for_delivery", "delivered", "ndr", "rto"}, "in_transit": {"out_for_delivery", "delivered", "ndr", "rto"}, "out_for_delivery": {"delivered", "ndr", "rto"}, "ndr": {"in_transit", "out_for_delivery", "delivered", "rto"}},
		"return":     {"requested": {"approved", "rejected"}, "approved": {"pickup_pending", "received", "rejected"}, "pickup_pending": {"received"}, "received": {"inspected"}, "inspected": {"completed", "rejected"}},
		"settlement": {"pending": {"eligible", "on_hold"}, "eligible": {"on_hold", "processing"}, "on_hold": {"eligible"}, "processing": {"paid", "failed", "on_hold"}, "failed": {"processing", "on_hold"}},
	}
	g, ok := graphs[system]
	if !ok {
		return false
	}
	if from == to {
		_, known := g[from]
		return known || (system == "shipment" && (from == "delivered" || from == "cancelled" || from == "rto")) || (system == "seller" && (from == "delivered" || from == "cancelled")) || (system == "return" && (from == "completed" || from == "rejected")) || (system == "settlement" && from == "paid")
	}
	for _, state := range g[from] {
		if state == to {
			return true
		}
	}
	return false
}

func MapShipmentStatus(raw string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "NEW", "PROCESSING", "PENDING", "READY TO SHIP", "AWB ASSIGNED", "LABEL GENERATED", "MANIFEST GENERATED", "SHIPMENT BOOKED":
		return "created", true
	case "PICKUP SCHEDULED", "PICKUP QUEUED", "PICKUP RESCHEDULED", "OUT FOR PICKUP":
		return "pickup_scheduled", true
	case "PICKED UP":
		return "picked_up", true
	case "SHIPPED", "IN TRANSIT", "IN-TRANSIT", "REACHED AT DESTINATION HUB":
		return "in_transit", true
	case "OUT FOR DELIVERY":
		return "out_for_delivery", true
	case "DELIVERED":
		return "delivered", true
	case "NDR", "UNDELIVERED", "UNDELIVERED - NDR", "DELIVERY FAILED":
		return "ndr", true
	case "RTO", "RTO INITIATED", "RTO IN TRANSIT", "RTO DELIVERED", "RTO ACKNOWLEDGED":
		return "rto", true
	case "CANCELED", "CANCELLED":
		return "cancelled", true
	default:
		return "", false
	}
}

// AllocateRefund preserves every paise across repeated partial-quantity refunds.
// Counts are cumulative; the last unit receives the rounding remainder.
func AllocateRefund(paid int64, total, already, quantity int) (int64, error) {
	if paid < 0 || paid > 100000000000 || total <= 0 || total > 50000 || already < 0 || quantity <= 0 || already+quantity > total {
		return 0, ErrTransition
	}
	return paid*int64(already+quantity)/int64(total) - paid*int64(already)/int64(total), nil
}
