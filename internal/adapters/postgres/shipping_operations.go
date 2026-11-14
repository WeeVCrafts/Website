package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *CommerceRepository) ShipmentForCancellation(ctx context.Context, actor, order, seller int64, environment string) (ports.Shipment, error) {
	var shipment ports.Shipment
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := requireOperationsActor(ctx, tx, actor); err != nil {
			return err
		}
		var err error
		shipment, err = scanShipment(tx.QueryRow(ctx, `SELECT `+shipmentFields+` FROM shiprocket_shipments sr JOIN orders o ON o.id=sr.order_id
   JOIN seller_fulfillments f ON f.order_id=sr.order_id AND f.seller_id=sr.seller_id AND f.status='packing'
   WHERE sr.order_id=$1 AND sr.seller_id=$2 AND sr.environment=$3 AND sr.provider_shipment_id IS NOT NULL AND sr.pending_action=''`, order, seller, environment))
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrInvalidState
		}
		return err
	})
	return shipment, err
}

func (r *CommerceRepository) ClaimShipmentCancellation(ctx context.Context, actor int64, s ports.Shipment, environment string) error {
	if !ports.ShippingPrePickup(s.ProviderStatus) || (s.PreviousProviderStatus != "" && !ports.ShippingPrePickup(s.PreviousProviderStatus)) {
		return ports.ErrInvalidState
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := requireOperationsActor(ctx, tx, actor); err != nil {
			return err
		}
		var order int64
		if err := tx.QueryRow(ctx, `SELECT id FROM orders WHERE id=$1 FOR UPDATE`, s.OrderID).Scan(&order); err != nil {
			return err
		}
		result, err := tx.Exec(ctx, `UPDATE shiprocket_shipments sr SET pending_action='cancel',pending_since=now(),next_check_at=now()+interval '2 minutes',updated_at=now(),provider_status=$6
   WHERE sr.order_id=$1 AND sr.seller_id=$2 AND sr.environment=$3 AND sr.provider_order_id=$4 AND sr.provider_shipment_id=$5 AND sr.pending_action=''
   AND sr.provider_status IS NOT DISTINCT FROM $7
   AND EXISTS(SELECT 1 FROM seller_fulfillments f WHERE f.order_id=$1 AND f.seller_id=$2 AND f.status='packing')`, order, s.SellerID, environment, s.ProviderOrderID, s.ProviderShipmentID, s.ProviderStatus, s.PreviousProviderStatus)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ports.ErrInvalidState
		}
		return openDeliveryException(ctx, tx, order, s.SellerID, environment, "pending_operation")
	})
}

func (r *CommerceRepository) RecordShipmentException(ctx context.Context, s ports.Shipment, environment, kind string) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		return openDeliveryException(ctx, tx, s.OrderID, s.SellerID, environment, kind)
	})
}

func (r *CommerceRepository) ListDeliveryExceptions(ctx context.Context, actor int64, environment string) ([]ports.DeliveryException, error) {
	var cases []ports.DeliveryException
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := requireOperationsActor(ctx, tx, actor); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT e.order_id,e.seller_id,o.order_number,e.kind,COALESCE(e.assignee_id,0),e.opened_at,COALESCE(sr.provider_status,''),
   COALESCE(sr.provider_shipment_id IS NOT NULL AND sr.pending_action='' AND f.status='packing',false),
   (e.kind<>'pending_operation' OR COALESCE(sr.pending_action,'')='')
   FROM delivery_exceptions e JOIN orders o ON o.id=e.order_id
   LEFT JOIN shiprocket_shipments sr ON sr.order_id=e.order_id AND sr.seller_id=e.seller_id AND sr.environment=$1
   LEFT JOIN seller_fulfillments f ON f.order_id=e.order_id AND f.seller_id=e.seller_id
   WHERE e.resolved_at IS NULL AND e.environment=$1 AND (sr.order_id IS NOT NULL OR (e.kind='checkout_fallback' AND NOT o.payment_review_required AND o.status IN ('paid','processing','shipped','delivered') AND (SELECT status FROM payment_attempts p WHERE p.order_id=o.id ORDER BY p.id DESC LIMIT 1)='captured'))
   ORDER BY e.opened_at,e.order_id,e.seller_id,e.kind LIMIT 100`, environment)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c ports.DeliveryException
			if err := rows.Scan(&c.OrderID, &c.SellerID, &c.OrderNumber, &c.Kind, &c.AssigneeID, &c.OpenedAt, &c.ShipmentStatus, &c.CanCancel, &c.CanResolve); err != nil {
				return err
			}
			c.CanCancel = c.CanCancel && ports.ShippingPrePickup(c.ShipmentStatus)
			cases = append(cases, c)
		}
		return rows.Err()
	})
	return cases, err
}

func (r *CommerceRepository) UpdateDeliveryException(ctx context.Context, actor, order, seller int64, environment, kind, action, reason string) error {
	reason = strings.TrimSpace(reason)
	if (action != "assign" && action != "resolve") || (action == "resolve" && (len(reason) < 3 || len(reason) > 500)) {
		return ports.ErrShippingInput
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := requireOperationsActor(ctx, tx, actor); err != nil {
			return err
		}
		result, err := tx.Exec(ctx, `UPDATE delivery_exceptions e SET assignee_id=$1,
   resolved_at=CASE WHEN $6='resolve' THEN now() END,
   resolved_by=CASE WHEN $6='resolve' THEN $1::bigint END,resolution=$7
   WHERE e.order_id=$2 AND e.seller_id=$3 AND e.kind=$4 AND e.environment=$5 AND e.resolved_at IS NULL
   AND (e.assignee_id IS NULL OR e.assignee_id=$1)
   AND (e.kind<>'cancelled_parcel' OR $6<>'resolve' OR EXISTS(SELECT 1 FROM shiprocket_shipments sr
      JOIN seller_fulfillments f ON f.order_id=sr.order_id AND f.seller_id=sr.seller_id AND f.status='packing'
      WHERE sr.order_id=e.order_id AND sr.seller_id=e.seller_id AND sr.environment=$5 AND sr.pending_action='' AND sr.provider_status IN ('CANCELLED','CANCELED')))
   AND (e.kind<>'pending_operation' OR $6<>'resolve' OR NOT EXISTS(SELECT 1 FROM shiprocket_shipments sr
      WHERE sr.order_id=e.order_id AND sr.seller_id=e.seller_id AND sr.environment=$5 AND sr.pending_action<>''))`,
			actor, order, seller, kind, environment, action, reason)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ports.ErrInvalidState
		}
		return nil
	})
}

func (r *CommerceRepository) ShippingBacklog(ctx context.Context, environment string) (int64, time.Time, int64, error) {
	var pending, exceptions int64
	var oldest *time.Time
	err := r.pool.QueryRow(ctx, `SELECT count(*),min(pending_since),(
  SELECT count(*) FROM delivery_exceptions e WHERE e.resolved_at IS NULL AND e.environment=$1 AND
  (EXISTS(SELECT 1 FROM shiprocket_shipments sr WHERE sr.order_id=e.order_id AND sr.seller_id=e.seller_id AND sr.environment=$1)
    OR (e.kind='checkout_fallback' AND EXISTS(SELECT 1 FROM orders o WHERE o.id=e.order_id AND NOT o.payment_review_required AND o.status IN ('paid','processing','shipped','delivered'))))
 ) FROM shiprocket_shipments WHERE environment=$1 AND pending_action<>''`, environment).Scan(&pending, &oldest, &exceptions)
	if oldest == nil {
		return pending, time.Time{}, exceptions, err
	}
	return pending, *oldest, exceptions, err
}
