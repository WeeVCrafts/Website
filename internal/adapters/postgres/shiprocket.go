package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

const shipmentFields = `sr.order_id,sr.seller_id,sr.requested_by,o.order_number,sr.reference,sr.pickup_location,
	sr.weight_grams,sr.length_cm,sr.width_cm,sr.height_cm,COALESCE(sr.provider_order_id,0),COALESCE(sr.provider_shipment_id,0),
	sr.pending_action,sr.provider_status,sr.awb,sr.courier,sr.pickup_reference,sr.label_url,sr.manifest_url,sr.next_check_at<=now(),COALESCE(o.address_snapshot->>'postal_code',''), EXISTS(SELECT 1 FROM delivery_exceptions e WHERE e.order_id=sr.order_id AND e.seller_id=sr.seller_id AND e.environment=sr.environment AND e.kind='cancelled_parcel' AND e.resolved_at IS NOT NULL)`

func scanShipment(row pgx.Row) (ports.Shipment, error) {
	var s ports.Shipment
	err := row.Scan(&s.OrderID, &s.SellerID, &s.RequestedBy, &s.OrderNumber, &s.Reference, &s.PickupLocation,
		&s.WeightGrams, &s.LengthCM, &s.WidthCM, &s.HeightCM, &s.ProviderOrderID, &s.ProviderShipmentID,
		&s.PendingAction, &s.ProviderStatus, &s.AWB, &s.Courier, &s.PickupReference, &s.LabelURL, &s.ManifestURL, &s.CanReconcile, &s.DeliveryPostalCode, &s.ManualDispatchAllowed)
	return s, err
}

func (r *CommerceRepository) SellerShipment(ctx context.Context, actor, seller int64, number, environment string) (ports.Shipment, error) {
	shipment, err := scanShipment(r.pool.QueryRow(ctx, `SELECT `+shipmentFields+` FROM shiprocket_shipments sr
		JOIN orders o ON o.id=sr.order_id JOIN sellers s ON s.id=sr.seller_id
		WHERE o.order_number=$1 AND s.id=$2 AND s.owner_user_id=$3 AND sr.environment=$4`, number, seller, actor, environment))
	if errors.Is(err, pgx.ErrNoRows) {
		return shipment, ports.ErrOrderNotFound
	}
	return shipment, err
}

func (r *CommerceRepository) ClaimShipment(ctx context.Context, actor, seller int64, number, environment, action, pickup string, parcel ports.Parcel) (ports.ShippingOrder, error) {
	var shipment ports.ShippingOrder
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var orderID int64
		var overall, payment, currency, stage string
		var review bool
		err := tx.QueryRow(ctx, `SELECT o.id,o.status,o.payment_review_required,o.currency,o.created_at,o.address_snapshot,u.email
			FROM orders o JOIN users u ON u.id=o.user_id JOIN sellers s ON s.id=$2 AND s.owner_user_id=$3 AND s.status='active'
			WHERE o.order_number=$1 AND EXISTS(SELECT 1 FROM order_items oi JOIN product_variants v ON v.id=oi.variant_id
			JOIN products p ON p.id=v.product_id WHERE oi.order_id=o.id AND p.seller_id=s.id) FOR UPDATE OF o`, number, seller, actor).
			Scan(&orderID, &overall, &review, &currency, &shipment.PlacedAt, &shipment.Address, &shipment.Email)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrOrderNotFound
		}
		if err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT status FROM payment_attempts WHERE order_id=$1 ORDER BY id DESC LIMIT 1),''),
			COALESCE((SELECT status FROM seller_fulfillments WHERE order_id=$1 AND seller_id=$2),'pending')`, orderID, seller).Scan(&payment, &stage); err != nil {
			return err
		}
		var modern bool
		if err = tx.QueryRow(ctx, `SELECT workflow_version=2 FROM orders WHERE id=$1`, orderID).Scan(&modern); err != nil {
			return err
		}
		if modern {
			return ports.ErrInvalidState
		}
		if !domaincommerce.PrepaidOrderConfirmed(overall, payment, review) || currency != "INR" || shipment.Address.CountryCode != "IN" {
			return ports.ErrInvalidState
		}
		current, err := scanShipment(tx.QueryRow(ctx, `SELECT `+shipmentFields+` FROM shiprocket_shipments sr
			JOIN orders o ON o.id=sr.order_id WHERE sr.order_id=$1 AND sr.seller_id=$2 AND sr.environment=$3 FOR UPDATE OF sr`, orderID, seller, environment))
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if errors.Is(err, pgx.ErrNoRows) {
			var decision string
			decisionErr := tx.QueryRow(ctx, `SELECT decision FROM order_delivery_decisions WHERE order_id=$1 AND seller_id=$2`, orderID, seller).Scan(&decision)
			if decisionErr != nil && !errors.Is(decisionErr, pgx.ErrNoRows) {
				return decisionErr
			}
			if decision != "" && decision != "shiprocket" {
				return ports.ErrInvalidState
			}
			if action != "create" || stage != "packing" || pickup == "" {
				return ports.ErrInvalidState
			}
			shipment.Shipment = ports.Shipment{OrderID: orderID, SellerID: seller, RequestedBy: actor, OrderNumber: number,
				Reference: shipmentReference(number, seller), PickupLocation: pickup, PendingAction: action, Parcel: parcel}
			rows, err := tx.Query(ctx, `SELECT oi.product_name,oi.sku,oi.quantity,oi.unit_price_cents,oi.line_total_cents,COALESCE(oi.allocated_discount_cents,0)
				FROM order_items oi JOIN product_variants v ON v.id=oi.variant_id JOIN products p ON p.id=v.product_id
				WHERE oi.order_id=$1 AND p.seller_id=$2 ORDER BY oi.id`, orderID, seller)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var item domaincommerce.SellerOrderItem
				var discount int64
				if err := rows.Scan(&item.ProductName, &item.SKU, &item.Quantity, &item.UnitPriceCents, &item.LineTotalCents, &discount); err != nil {
					return err
				}
				shipment.Items = append(shipment.Items, item)
				shipment.DiscountCents += discount
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(shipment.Items) == 0 || strings.TrimSpace(shipment.Email) == "" {
				return ports.ErrShippingInput
			}
			_, err = tx.Exec(ctx, `INSERT INTO shiprocket_shipments (order_id,seller_id,requested_by,environment,reference,pickup_location,
				weight_grams,length_cm,width_cm,height_cm,pending_action,next_check_at,pending_since) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'create',now()+interval '2 minutes',now())`,
				orderID, seller, actor, environment, shipment.Reference, pickup, parcel.WeightGrams, parcel.LengthCM, parcel.WidthCM, parcel.HeightCM)
			if err != nil {
				return err
			}
			return openDeliveryException(ctx, tx, orderID, seller, environment, "pending_operation")
		}
		shipment.Shipment = current
		if ports.ShipmentCancelled(current.ProviderStatus) {
			return ports.ErrInvalidState
		}
		if current.PendingAction != "" {
			return ports.ErrShippingPending
		}
		if action == "create" && current.ProviderOrderID > 0 {
			return nil
		}
		if current.ProviderShipmentID <= 0 {
			return ports.ErrInvalidState
		}
		switch action {
		case "assign":
			if current.AWB != "" {
				return nil
			}
			if stage != "packing" {
				return ports.ErrInvalidState
			}
		case "pickup":
			if current.PickupReference != "" {
				return nil
			}
			if current.AWB == "" || stage != "packing" {
				return ports.ErrInvalidState
			}
		case "label":
			if current.LabelURL != "" {
				return nil
			}
			if current.AWB == "" {
				return ports.ErrInvalidState
			}
		case "manifest":
			if current.ManifestURL != "" {
				return nil
			}
			if current.AWB == "" || current.PickupReference == "" {
				return ports.ErrInvalidState
			}
		default:
			return ports.ErrShippingInput
		}
		_, err = tx.Exec(ctx, `UPDATE shiprocket_shipments SET pending_action=$4,pending_since=now(),next_check_at=now()+interval '2 minutes',updated_at=now()
			WHERE order_id=$1 AND seller_id=$2 AND environment=$3`, orderID, seller, environment, action)
		shipment.PendingAction = action
		if err != nil {
			return err
		}
		return openDeliveryException(ctx, tx, orderID, seller, environment, "pending_operation")
	})
	return shipment, err
}

func shipmentReference(number string, seller int64) string {
	hash := sha256.Sum256(fmt.Appendf(nil, "%s:%d", number, seller))
	return fmt.Sprintf("WC-%x", hash[:12])
}

func (r *CommerceRepository) SaveShipment(ctx context.Context, environment string, s ports.Shipment, action string, complete bool) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `UPDATE shiprocket_shipments SET provider_order_id=COALESCE(provider_order_id,NULLIF($5,0)),
			provider_shipment_id=COALESCE(provider_shipment_id,NULLIF($6,0)),
			provider_status=CASE WHEN provider_status IN ('DELIVERED','CANCELED','CANCELLED','RTO DELIVERED') THEN provider_status ELSE COALESCE(NULLIF($7,''),provider_status) END,awb=COALESCE(NULLIF($8,''),awb),
			courier=COALESCE(NULLIF($9,''),courier),pickup_reference=COALESCE(NULLIF($10,''),pickup_reference),
			label_url=COALESCE(NULLIF($11,''),label_url),manifest_url=COALESCE(NULLIF($12,''),manifest_url),pending_action=CASE WHEN $13 THEN '' ELSE pending_action END,pending_since=CASE WHEN $13 THEN NULL ELSE pending_since END,
			next_check_at=now()+interval '5 minutes',updated_at=now()
			WHERE order_id=$1 AND seller_id=$2 AND environment=$3 AND pending_action=$4
			AND (provider_status NOT IN ('DELIVERED','CANCELED','CANCELLED','RTO DELIVERED') OR $7='' OR provider_status=$7)
   AND (provider_order_id IS NULL OR $5=0 OR provider_order_id=$5)
			AND (provider_shipment_id IS NULL OR $6=0 OR provider_shipment_id=$6)`, s.OrderID, s.SellerID, environment, action,
			s.ProviderOrderID, s.ProviderShipmentID, s.ProviderStatus, s.AWB, s.Courier, s.PickupReference, s.LabelURL, s.ManifestURL, complete)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ports.ErrShippingPending
		}
		if (s.LabelURL != "" || s.ManifestURL != "") && complete {
			if _, err := tx.Exec(ctx, `UPDATE delivery_exceptions SET resolved_at=now(),resolution='Documents saved' WHERE order_id=$1 AND seller_id=$2 AND environment=$3 AND kind='document_failed' AND resolved_at IS NULL AND EXISTS (
    SELECT 1 FROM shiprocket_shipments WHERE order_id=$1 AND seller_id=$2 AND environment=$3 AND label_url<>'' AND (pickup_reference='' OR manifest_url<>''))`, s.OrderID, s.SellerID, environment); err != nil {
				return err
			}
		}
		if complete {
			if _, err := tx.Exec(ctx, `UPDATE delivery_exceptions SET resolved_at=now(),resolution='Provider confirmation recorded' WHERE order_id=$1 AND seller_id=$2 AND environment=$3 AND kind='pending_operation' AND resolved_at IS NULL`, s.OrderID, s.SellerID, environment); err != nil {
				return err
			}
		}
		switch s.ProviderStatus {
		case "CANCELLED", "CANCELED":
			return openDeliveryException(ctx, tx, s.OrderID, s.SellerID, environment, "cancelled_parcel")
		case "RTO INITIATED", "RTO DELIVERED", "LOST", "PICKUP EXCEPTION", "UNDELIVERED", "DELAYED", "DAMAGED":
			return openDeliveryException(ctx, tx, s.OrderID, s.SellerID, environment, "provider_exception")
		}
		return nil
	})
}

func (r *CommerceRepository) RejectShipmentCreation(ctx context.Context, environment string, s ports.Shipment) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `DELETE FROM shiprocket_shipments WHERE order_id=$1 AND seller_id=$2 AND environment=$3
   AND pending_action='create' AND provider_order_id IS NULL`, s.OrderID, s.SellerID, environment)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ports.ErrShippingPending
		}
		_, err = tx.Exec(ctx, `UPDATE delivery_exceptions SET resolved_at=now(),resolution='Provider rejected shipment creation' WHERE order_id=$1 AND seller_id=$2 AND environment=$3 AND kind='pending_operation' AND resolved_at IS NULL`, s.OrderID, s.SellerID, environment)
		return err
	})
}

func (r *CommerceRepository) ApplyShipmentStage(ctx context.Context, environment string, s ports.Shipment, stage domaincommerce.FulfillmentStage) error {
	return r.updateSellerFulfillment(ctx, s.RequestedBy, s.SellerID, s.OrderNumber, string(stage), s.Courier, s.AWB, "", -1, environment)
}

func (r *CommerceRepository) ScheduleShipmentCheck(ctx context.Context, environment string, providerOrderID int64, reference, awb string) error {
	// Keep in-flight mutations leased. The worker checks them after the lease expires.
	_, err := r.pool.Exec(ctx, `UPDATE shiprocket_shipments SET next_check_at=LEAST(next_check_at,now())
		WHERE environment=$1 AND provider_order_id=$2 AND reference=$3 AND (awb='' OR awb=$4)
		AND pending_action='' AND provider_status NOT IN ('DELIVERED','CANCELED','CANCELLED','RTO DELIVERED')`,
		environment, providerOrderID, reference, awb)
	return err
}

func (r *CommerceRepository) DueShipments(ctx context.Context, environment string, limit int) ([]ports.Shipment, error) {
	rows, err := r.pool.Query(ctx, `WITH due AS (
		SELECT order_id,seller_id FROM shiprocket_shipments
		WHERE environment=$1 AND next_check_at<=now() AND provider_status NOT IN ('DELIVERED','CANCELED','CANCELLED','RTO DELIVERED')
		ORDER BY next_check_at LIMIT $2 FOR UPDATE SKIP LOCKED
	), claimed AS (UPDATE shiprocket_shipments sr SET next_check_at=now()+interval '5 minutes' FROM due
		WHERE sr.order_id=due.order_id AND sr.seller_id=due.seller_id AND sr.environment=$1 RETURNING sr.*)
	SELECT `+shipmentFields+` FROM claimed sr JOIN orders o ON o.id=sr.order_id`, environment, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var shipments []ports.Shipment
	for rows.Next() {
		shipping, err := scanShipment(rows)
		if err != nil {
			return nil, err
		}
		shipments = append(shipments, shipping)
	}
	return shipments, rows.Err()
}
