package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *MarketplaceRepository) DueJobs(ctx context.Context, limit int) ([]ports.MarketplaceJob, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,resource_id,kind,attempts FROM marketplace_jobs WHERE available_at<=now() AND (status='queued' OR (status='running' AND claimed_at<now()-interval '3 minutes')) ORDER BY id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []ports.MarketplaceJob
	for rows.Next() {
		var j ports.MarketplaceJob
		if err = rows.Scan(&j.ID, &j.ResourceID, &j.Kind, &j.Attempts); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}
func (r *MarketplaceRepository) JobDispatched(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE outbox_events SET processed_at=COALESCE(processed_at,now()) WHERE event_type='MarketplaceJob' AND aggregate_id=$1`, fmt.Sprint(id))
	return err
}
func (r *MarketplaceRepository) ClaimJob(ctx context.Context, id int64) (ports.MarketplaceJob, bool, error) {
	var j ports.MarketplaceJob
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(727411)`); e != nil {
			return e
		}
		return tx.QueryRow(ctx, `UPDATE marketplace_jobs j SET status='running',claimed_at=now(),attempts=attempts+1 WHERE id=$1 AND available_at<=now() AND (status='queued' OR (status='running' AND claimed_at<now()-interval '3 minutes')) AND NOT EXISTS(SELECT 1 FROM marketplace_jobs x WHERE x.id<>j.id AND x.kind=j.kind AND x.resource_id=j.resource_id AND x.status='running' AND x.claimed_at>=now()-interval '3 minutes') RETURNING id,resource_id,kind,attempts`, id).Scan(&j.ID, &j.ResourceID, &j.Kind, &j.Attempts)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return j, false, nil
	}
	return j, err == nil, err
}
func (r *MarketplaceRepository) FinishJob(ctx context.Context, j ports.MarketplaceJob, workErr error) error {
	status, message := "done", ""
	delay := time.Duration(1<<min(j.Attempts, 8)) * time.Minute
	if workErr != nil {
		status = "queued"
		message = "Operation needs provider reconciliation or configuration review"
		if j.Attempts >= 8 || errors.Is(workErr, ports.ErrCancellationAfterHandover) {
			status = "failed"
		}
	}
	if j.Kind == "cancel_items" && workErr != nil {
		return r.recordCancellationJobFailure(ctx, j, status, delay, workErr)
	}
	_, err := r.pool.Exec(ctx, `UPDATE marketplace_jobs SET status=$2,last_error=$3,available_at=now()+$4::interval,claimed_at=NULL WHERE id=$1 AND status='running' AND attempts=$5`, j.ID, status, message, delay.String(), j.Attempts)
	if err != nil {
		return err
	}
	if status == "failed" && j.Kind == "book_package" {
		_, err = r.pool.Exec(ctx, `INSERT INTO marketplace_cases(order_id,seller_id,package_id,kind,reason) SELECT order_id,seller_id,id,'provider_error',$2 FROM marketplace_packages WHERE id=$1 ON CONFLICT DO NOTHING`, j.ResourceID, message)
	}
	return err
}

func (r *MarketplaceRepository) recordCancellationJobFailure(ctx context.Context, j ports.MarketplaceJob, status string, delay time.Duration, workErr error) error {
	handedOver := errors.Is(workErr, ports.ErrCancellationAfterHandover)
	reason := "Carrier cancellation could not be verified. Check the affected shipment and pickup with the carrier."
	if handedOver {
		reason = "The carrier confirmed handover. Review this request through the delivery exception workflow."
	} else if errors.Is(workErr, ports.ErrShippingRejected) {
		reason = "The carrier declined cancellation. Verify the shipment and pickup before taking further action."
	} else if status == "failed" {
		reason = "Cancellation checks ran out of retries. Verify the shipment and pickup before taking further action."
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var order int64
		if err := tx.QueryRow(ctx, `SELECT o.id FROM orders o JOIN marketplace_cancellations c ON c.order_id=o.id WHERE c.id=$1 FOR UPDATE OF o`, j.ResourceID).Scan(&order); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE marketplace_jobs SET status=$2,last_error=$3,available_at=now()+$4::interval,claimed_at=NULL
			WHERE id=$1 AND status='running' AND attempts=$5`, j.ID, status, reason, delay.String(), j.Attempts)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO marketplace_cases(order_id,seller_id,package_id,kind,reason)
			SELECT c.order_id,oi.seller_id,p.id,'cancellation_review',$2
			FROM marketplace_cancellations c JOIN order_items oi ON oi.id=c.order_item_id
			LEFT JOIN LATERAL (
				SELECT mp.id FROM marketplace_package_items pi JOIN marketplace_packages mp ON mp.id=pi.package_id
				WHERE pi.order_item_id=oi.id AND mp.status<>'cancelled'
			) p ON true
			WHERE c.id=$1 AND c.status='requested' AND ($3=false OR p.id IN
				(SELECT id FROM marketplace_packages WHERE status IN ('picked_up','in_transit','out_for_delivery','delivered','ndr','rto')))
			ON CONFLICT (order_id,(COALESCE(seller_id,0)),(COALESCE(package_id,0)),kind) WHERE status='open'
			DO UPDATE SET reason=EXCLUDED.reason`, j.ResourceID, reason, handedOver); err != nil {
			return err
		}
		if handedOver {
			_, err = tx.Exec(ctx, `UPDATE marketplace_cancellations SET status='rejected' WHERE id=$1 AND status='requested'`, j.ResourceID)
		}
		return err
	})
}

func (r *MarketplaceRepository) ShippingOrder(ctx context.Context, id int64) (ports.ShippingOrder, error) {
	var s ports.ShippingOrder
	err := r.pool.QueryRow(ctx, `SELECT p.order_id,p.seller_id,p.requested_by,o.order_number,p.reference,p.pickup_location,p.weight_grams,p.length_cm,p.width_cm,p.height_cm,COALESCE(p.provider_order_id,0),COALESCE(p.provider_shipment_id,0),p.pending_action,p.provider_status,p.awb,p.courier,p.pickup_reference,p.label_url,p.manifest_url,o.address_snapshot,o.created_at,u.email,COALESCE(o.address_snapshot->>'postal_code','') FROM marketplace_packages p JOIN orders o ON o.id=p.order_id JOIN users u ON u.id=o.user_id WHERE p.id=$1`, id).Scan(&s.OrderID, &s.SellerID, &s.RequestedBy, &s.OrderNumber, &s.Reference, &s.PickupLocation, &s.WeightGrams, &s.LengthCM, &s.WidthCM, &s.HeightCM, &s.ProviderOrderID, &s.ProviderShipmentID, &s.PendingAction, &s.ProviderStatus, &s.AWB, &s.Courier, &s.PickupReference, &s.LabelURL, &s.ManifestURL, &s.Address, &s.PlacedAt, &s.Email, &s.DeliveryPostalCode)
	if err != nil {
		return s, err
	}
	if err = r.pool.QueryRow(ctx, `SELECT rejected_courier_ids FROM marketplace_packages WHERE id=$1`, id).Scan(&s.RejectedCourierIDs); err != nil {
		return s, err
	}
	rows, err := r.pool.Query(ctx, `SELECT oi.product_name,oi.sku,pi.quantity,oi.unit_price_cents,oi.unit_price_cents*pi.quantity,COALESCE(oi.allocated_discount_cents,0)*pi.quantity/oi.quantity FROM marketplace_package_items pi JOIN order_items oi ON oi.id=pi.order_item_id WHERE pi.package_id=$1 ORDER BY oi.id`, id)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.SellerOrderItem
		var discount int64
		if err = rows.Scan(&item.ProductName, &item.SKU, &item.Quantity, &item.UnitPriceCents, &item.LineTotalCents, &discount); err != nil {
			return s, err
		}
		s.Items = append(s.Items, item)
		s.DiscountCents += discount
	}
	return s, rows.Err()
}
func (r *MarketplaceRepository) StartPackageAction(ctx context.Context, id int64, action string) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var order int64
		if err := tx.QueryRow(ctx, `SELECT o.id FROM orders o JOIN marketplace_packages p ON p.order_id=o.id WHERE p.id=$1 FOR UPDATE OF o`, id).Scan(&order); err != nil {
			return err
		}
		var allowed bool
		err := tx.QueryRow(ctx, `SELECT marketplace_order_funded(o.id) AND p.pending_action='' AND p.status NOT IN ('delivered','rto','cancelled')
 AND ($2 NOT IN ('create','assign','pickup','cancel') OR p.status IN ('created','pickup_scheduled'))
 AND ($2 NOT IN ('create','assign','pickup') OR NOT(o.policy_snapshot ? 'return_policy_version') OR EXISTS(SELECT 1 FROM marketplace_policy_settings WHERE id AND shipping_mode<>'off'))
 AND ($2='cancel' OR ((EXISTS(SELECT 1 FROM seller_fulfillments sf WHERE sf.order_id=o.id AND sf.seller_id=p.seller_id AND sf.lifecycle_status IN ('ready_to_ship','shipped')) OR EXISTS(SELECT 1 FROM marketplace_claims c WHERE c.id=p.replacement_claim_id AND c.replacement_package_id=p.id AND c.status='inspected' AND c.resolution='replacement' AND c.replacement_reserved))
 AND NOT EXISTS(SELECT 1 FROM marketplace_cancellations c JOIN marketplace_package_items pi ON pi.order_item_id=c.order_item_id WHERE pi.package_id=p.id AND c.status='requested')))
 FROM marketplace_packages p JOIN orders o ON o.id=p.order_id WHERE p.id=$1 FOR UPDATE OF p`, id, action).Scan(&allowed)
		if err != nil {
			return err
		}
		if !allowed {
			return ports.ErrShippingPending
		}
		_, err = tx.Exec(ctx, `UPDATE marketplace_packages SET pending_action=$2,updated_at=now() WHERE id=$1`, id, action)
		return err
	})
}
func (r *MarketplaceRepository) RejectPackageAction(ctx context.Context, id int64, action string, courier int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE marketplace_packages SET pending_action='',rejected_courier_ids=CASE WHEN $3>0 THEN array_append(rejected_courier_ids,$3) ELSE rejected_courier_ids END,updated_at=now() WHERE id=$1 AND pending_action=$2`, id, action, courier)
	return err
}
func (r *MarketplaceRepository) SavePackage(ctx context.Context, id int64, s ports.Shipment, pending string) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var order, seller int64
		var previous, reference string
		if err := tx.QueryRow(ctx, `SELECT o.id,p.seller_id,p.status,p.reference FROM orders o JOIN marketplace_packages p ON p.order_id=o.id WHERE p.id=$1 FOR UPDATE OF o`, id).Scan(&order, &seller, &previous, &reference); err != nil {
			return err
		}
		if s.Reference != reference {
			return ports.ErrShippingPending
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(4856203)`); err != nil {
			return err
		}
		target, known := domain.MapShipmentStatus(s.ProviderStatus)
		if previous == "delivered" && (target == "rto" || target == "ndr") || previous == "cancelled" && (target == "picked_up" || target == "in_transit" || target == "out_for_delivery" || target == "delivered") {
			if err := marketplaceCase(ctx, tx, order, seller, id, "delivery_dispute", "Carrier correction after terminal confirmation: "+s.ProviderStatus); err != nil {
				return err
			}
		}
		if !known || !domain.CanTransition("shipment", previous, target) {
			target = previous
		}
		result, err := tx.Exec(ctx, `UPDATE marketplace_packages SET provider_order_id=COALESCE(provider_order_id,NULLIF($2,0)),provider_shipment_id=COALESCE(provider_shipment_id,NULLIF($3,0)),provider_status=CASE WHEN status IN ('delivered','cancelled') THEN provider_status ELSE COALESCE(NULLIF($4,''),provider_status) END,status=$5,awb=COALESCE(NULLIF($6,''),awb),courier=COALESCE(NULLIF($7,''),courier),pickup_reference=COALESCE(NULLIF($8,''),pickup_reference),label_url=COALESCE(NULLIF($9,''),label_url),manifest_url=COALESCE(NULLIF($10,''),manifest_url),pending_action=$11,delivered_at=CASE WHEN $5='delivered' THEN COALESCE(delivered_at,now()) ELSE delivered_at END,revision=revision+1,updated_at=now(),next_check_at=now()+interval '5 minutes' WHERE id=$1 AND (provider_order_id IS NULL OR provider_order_id=$2) AND (provider_shipment_id IS NULL OR provider_shipment_id=$3)`, id, s.ProviderOrderID, s.ProviderShipmentID, s.ProviderStatus, target, s.AWB, s.Courier, s.PickupReference, s.LabelURL, s.ManifestURL, pending)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ports.ErrShippingPending
		}
		if previous != target {
			if target == "cancelled" {
				if _, err = tx.Exec(ctx, `INSERT INTO marketplace_cases(order_id,seller_id,package_id,kind,reason)
					SELECT order_id,seller_id,id,'pickup_clearance','Verify that this AWB was removed from the courier pickup or that the courier closed the pickup without collecting it.'
					FROM marketplace_packages WHERE id=$1 AND delivery_method='shiprocket' AND (awb<>'' OR pickup_reference<>'')
					ON CONFLICT DO NOTHING`, id); err != nil {
					return err
				}
			}
			if target == "picked_up" || target == "in_transit" || target == "out_for_delivery" || target == "delivered" {
				if _, err = tx.Exec(ctx, `UPDATE seller_fulfillments SET handover_at=COALESCE(handover_at,now()) WHERE order_id=$1 AND seller_id=$2`, order, seller); err != nil {
					return err
				}
				var expected, committed int
				if err = tx.QueryRow(ctx, `WITH expected AS MATERIALIZED (SELECT oi.variant_id,c.quantity FROM marketplace_claims c JOIN order_items oi ON oi.id=c.order_item_id WHERE c.replacement_package_id=$1 AND c.replacement_reserved AND NOT c.replacement_committed), committed AS (UPDATE inventory_stock i SET reserved_quantity=reserved_quantity-e.quantity FROM expected e WHERE i.variant_id=e.variant_id AND i.reserved_quantity>=e.quantity RETURNING i.variant_id) SELECT (SELECT count(*) FROM expected),(SELECT count(*) FROM committed)`, id).Scan(&expected, &committed); err != nil {
					return err
				}
				if expected != committed {
					return ports.ErrInvalidState
				}
				if _, err = tx.Exec(ctx, `UPDATE marketplace_claims SET replacement_committed=true WHERE replacement_package_id=$1 AND replacement_reserved`, id); err != nil {
					return err
				}
			}
			if target == "delivered" {
				if _, err = tx.Exec(ctx, `UPDATE inventory_stock i SET available_quantity=available_quantity+c.quantity FROM marketplace_claims c JOIN order_items oi ON oi.id=c.order_item_id WHERE c.replacement_package_id=$1 AND c.status='inspected' AND c.restock AND i.variant_id=oi.variant_id`, id); err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `UPDATE order_items oi SET compensated_quantity=compensated_quantity+c.quantity FROM marketplace_claims c WHERE c.replacement_package_id=$1 AND c.status='inspected' AND oi.id=c.order_item_id AND compensated_quantity+c.quantity<=oi.quantity`, id); err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `UPDATE marketplace_claims SET status='completed',updated_at=now() WHERE replacement_package_id=$1 AND status='inspected'`, id); err != nil {
					return err
				}
				if err = resolvePolicyClaimCase(ctx, tx, order, seller); err != nil {
					return err
				}
			}
			if _, err = tx.Exec(ctx, `INSERT INTO marketplace_history(order_id,seller_id,package_id,system,from_status,to_status,reason) VALUES($1,$2,$3,'shipment',$4,$5,$6)`, order, seller, id, previous, target, s.ProviderStatus); err != nil {
				return err
			}
		}
		kind := ""
		switch target {
		case "ndr":
			kind = "ndr"
		case "rto":
			kind = "rto"
		}
		if !known && s.ProviderStatus != "" {
			kind = "unknown_status"
		}
		if kind != "" {
			if err = marketplaceCase(ctx, tx, order, seller, id, kind, "Carrier status: "+s.ProviderStatus); err != nil {
				return err
			}
		}
		switch s.ProviderStatus {
		case "LOST", "DAMAGED", "PICKUP EXCEPTION":
			if err = marketplaceCase(ctx, tx, order, seller, id, "failed_delivery", "Carrier exception: "+s.ProviderStatus); err != nil {
				return err
			}
		case "DELAYED":
			if err = marketplaceCase(ctx, tx, order, seller, id, "delayed_order", "Carrier reports delayed delivery"); err != nil {
				return err
			}
		}
		if target == "delivered" {
			if _, err = tx.Exec(ctx, `UPDATE marketplace_cases SET status='resolved',resolved_at=now(),resolution='Authenticated carrier delivery confirmation' WHERE package_id=$1 AND status='open' AND kind IN ('ndr','delayed_order')`, id); err != nil {
				return err
			}
		}
		if (target == "created" || target == "pickup_scheduled") && s.AWB != "" && s.LabelURL != "" && pending != "cancel" {
			_, err = tx.Exec(ctx, `INSERT INTO notifications(user_id,kind,title,body,link_url,source_key,marketplace_package_id)
				SELECT o.user_id,'order','Package packed','A package for order '||o.order_number||' is packed and awaiting courier handover.',
					'/orders/'||o.order_number||'?tab=packages','marketplace-package:'||p.id||':packed',p.id
				FROM marketplace_packages p JOIN orders o ON o.id=p.order_id
				WHERE p.id=$1 AND NOT EXISTS(SELECT 1 FROM marketplace_package_items pi JOIN marketplace_cancellations c ON c.order_item_id=pi.order_item_id
					WHERE pi.package_id=p.id AND c.status='requested')
				ON CONFLICT (user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING`, id)
			if err != nil {
				return err
			}
		}
		if target == "out_for_delivery" || target == "delivered" {
			_, err = tx.Exec(ctx, `INSERT INTO notifications(user_id,kind,title,body,link_url,marketplace_package_id) SELECT o.user_id,'order',$3,'Package '||$2||' for order '||o.order_number,$4,$5 FROM orders o WHERE o.id=$1 AND NOT EXISTS(SELECT 1 FROM marketplace_history WHERE package_id=$5 AND to_status=$6 AND id<(SELECT max(id) FROM marketplace_history WHERE package_id=$5 AND to_status=$6)) AND $7<>$6`, order, reference, map[string]string{"out_for_delivery": "Package out for delivery", "delivered": "Package delivered"}[target], "/orders/"+s.OrderNumber+"/fulfillment", id, target, previous)
			if err != nil {
				return err
			}
		}
		return refreshMarketplaceAggregate(ctx, tx, order, seller)
	})
}
func refreshMarketplaceAggregate(ctx context.Context, tx pgx.Tx, order, seller int64) error {
	var total, delivered, shipped int
	err := tx.QueryRow(ctx, `SELECT COALESCE(sum(oi.quantity-oi.cancelled_quantity),0),COALESCE(sum((SELECT COALESCE(sum(pi.quantity),0) FROM marketplace_package_items pi JOIN marketplace_packages p ON p.id=pi.package_id WHERE pi.order_item_id=oi.id AND p.status='delivered')),0),COALESCE(sum((SELECT COALESCE(sum(pi.quantity),0) FROM marketplace_package_items pi JOIN marketplace_packages p ON p.id=pi.package_id WHERE pi.order_item_id=oi.id AND p.status IN ('picked_up','in_transit','out_for_delivery','delivered','ndr','rto'))),0) FROM order_items oi WHERE order_id=$1 AND seller_id=$2`, order, seller).Scan(&total, &delivered, &shipped)
	if err != nil {
		return err
	}
	if total == 0 {
		_, err = tx.Exec(ctx, `UPDATE seller_fulfillments SET lifecycle_status='cancelled',revision=revision+1 WHERE order_id=$1 AND seller_id=$2`, order, seller)
	} else if delivered >= total {
		_, err = tx.Exec(ctx, `UPDATE seller_fulfillments SET lifecycle_status='delivered',status='delivered',delivered_at=COALESCE(delivered_at,now()),revision=revision+1 WHERE order_id=$1 AND seller_id=$2 AND lifecycle_status<>'delivered'`, order, seller)
	} else if shipped > 0 {
		_, err = tx.Exec(ctx, `UPDATE seller_fulfillments SET lifecycle_status='shipped',status='shipped',revision=revision+1 WHERE order_id=$1 AND seller_id=$2 AND lifecycle_status NOT IN ('shipped','delivered')`, order, seller)
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE orders o SET lifecycle_status=CASE WHEN NOT EXISTS(SELECT 1 FROM order_items WHERE order_id=o.id AND quantity>cancelled_quantity) THEN 'cancelled' WHEN NOT EXISTS(SELECT 1 FROM seller_fulfillments WHERE order_id=o.id AND lifecycle_status NOT IN ('delivered','cancelled')) THEN 'completed' WHEN EXISTS(SELECT 1 FROM marketplace_packages WHERE order_id=o.id AND status='delivered') THEN 'partially_delivered' WHEN EXISTS(SELECT 1 FROM marketplace_packages WHERE order_id=o.id AND status IN ('picked_up','in_transit','out_for_delivery','ndr','rto')) THEN 'partially_shipped' ELSE 'confirmed' END, status=CASE WHEN NOT EXISTS(SELECT 1 FROM order_items WHERE order_id=o.id AND quantity>cancelled_quantity) THEN 'cancelled' WHEN NOT EXISTS(SELECT 1 FROM seller_fulfillments WHERE order_id=o.id AND lifecycle_status NOT IN ('delivered','cancelled')) AND EXISTS(SELECT 1 FROM seller_fulfillments WHERE order_id=o.id AND lifecycle_status='delivered') THEN 'delivered' ELSE status END,updated_at=now() WHERE id=$1`, order)
	return err
}

func (r *MarketplaceRepository) QueueMaintenance(ctx context.Context) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := r.QueuePolicyMaintenance(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(727412)`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `SELECT marketplace_enqueue('book_package',p.id,'track:'||p.id||':'||to_char(now(),'YYYYMMDDHH24MI')) FROM marketplace_packages p WHERE p.delivery_method='shiprocket' AND p.next_check_at<=now() AND p.status NOT IN ('delivered','cancelled') AND (EXISTS(SELECT 1 FROM seller_fulfillments sf WHERE sf.order_id=p.order_id AND sf.seller_id=p.seller_id AND sf.lifecycle_status IN ('ready_to_ship','shipped')) OR EXISTS(SELECT 1 FROM marketplace_claims c WHERE c.id=p.replacement_claim_id AND c.status='inspected')) ORDER BY p.next_check_at LIMIT 20`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO notifications(user_id,kind,title,body,link_url) SELECT s.owner_user_id,'order','Order acceptance overdue','Please confirm availability for order '||o.order_number,'/seller-admin/orders/'||o.order_number||'/fulfillment' FROM seller_fulfillments sf JOIN orders o ON o.id=sf.order_id JOIN sellers s ON s.id=sf.seller_id WHERE sf.lifecycle_status='new' AND sf.reminded_at IS NULL AND sf.created_at<=now()-interval '24 hours' AND o.status='paid'`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE seller_fulfillments sf SET reminded_at=now() FROM orders o WHERE o.id=sf.order_id AND sf.lifecycle_status='new' AND sf.reminded_at IS NULL AND sf.created_at<=now()-interval '24 hours' AND o.status='paid'`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO marketplace_cases(order_id,seller_id,kind,reason) SELECT sf.order_id,sf.seller_id,'seller_escalation','Seller has not accepted the paid order within 48 hours' FROM seller_fulfillments sf JOIN orders o ON o.id=sf.order_id WHERE sf.lifecycle_status='new' AND sf.created_at<=now()-interval '48 hours' AND o.status='paid' ON CONFLICT DO NOTHING`)
		return err
	})
}
func (r *MarketplaceRepository) EnqueueScheduled(ctx context.Context, kind, key string) error {
	_, err := r.pool.Exec(ctx, `SELECT marketplace_enqueue($1,0,$2)`, kind, key)
	return err
}
func (r *MarketplaceRepository) NotifySellers(ctx context.Context, order int64) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO notifications(user_id,kind,title,body,link_url) SELECT s.owner_user_id,'order','New paid order','Accept and prepare your items for order '||o.order_number,'/seller-admin/orders/'||o.order_number||'/fulfillment' FROM seller_fulfillments sf JOIN sellers s ON s.id=sf.seller_id JOIN orders o ON o.id=sf.order_id WHERE o.id=$1 AND NOT EXISTS(SELECT 1 FROM notifications WHERE user_id=s.owner_user_id AND link_url='/seller-admin/orders/'||o.order_number||'/fulfillment' AND title='New paid order')`, order)
	return err
}

func (r *MarketplaceRepository) ConfigureShippingProfiles(ctx context.Context, locations map[int64]string, pins map[int64]string) error {
	for id, location := range locations {
		if _, err := r.pool.Exec(ctx, `INSERT INTO marketplace_shipping_profiles(seller_id,pickup_location,postal_code,verified) SELECT id,$2,$3,true FROM sellers WHERE id=$1 ON CONFLICT(seller_id) DO UPDATE SET pickup_location=EXCLUDED.pickup_location,postal_code=EXCLUDED.postal_code,verified=true,updated_at=now()`, id, location, pins[id]); err != nil {
			return err
		}
	}
	return nil
}
func (r *MarketplaceRepository) SaveShippingProfile(ctx context.Context, actor, seller int64, location, pin, reason string) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		a, err := marketplaceIdentity(ctx, tx, actor)
		if err != nil {
			return err
		}
		if a.role != "super_admin" || len(reason) < 4 || len(location) > 100 {
			return ports.ErrForbidden
		}
		_, err = tx.Exec(ctx, `INSERT INTO marketplace_shipping_profiles(seller_id,pickup_location,postal_code,verified,verified_by) VALUES($1,$2,$3,true,$4) ON CONFLICT(seller_id) DO UPDATE SET pickup_location=EXCLUDED.pickup_location,postal_code=EXCLUDED.postal_code,verified=true,verified_by=EXCLUDED.verified_by,updated_at=now()`, seller, location, pin, actor)
		if err != nil {
			return err
		}
		return marketplaceAudit(ctx, tx, actor, 0, "shipping_pickup_verified", reason)
	})
}

// WakePackage is called only by the authenticated delivery callback. It records
// a hint; the worker obtains status using the provider's authenticated API.
func (r *MarketplaceRepository) WakePackage(ctx context.Context, provider int64, reference, awb string) error {
	_, err := r.pool.Exec(ctx, `UPDATE marketplace_packages SET next_check_at=LEAST(next_check_at,now()) WHERE provider_order_id=$1 AND reference=$2 AND (awb='' OR awb=$3) AND status NOT IN ('delivered','cancelled')`, provider, reference, awb)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `UPDATE marketplace_claims SET next_check_at=LEAST(next_check_at,now()) WHERE return_order_id=$1 AND return_reference=$2 AND (return_awb='' OR return_awb=$3) AND status IN ('approved','pickup_pending')`, provider, reference, awb)
	return err
}

func (r *MarketplaceRepository) CancellationPackages(ctx context.Context, id int64) ([]ports.MarketplacePackage, error) {
	rows, err := r.pool.Query(ctx, `SELECT p.id FROM marketplace_cancellations c JOIN marketplace_package_items pi ON pi.order_item_id=c.order_item_id JOIN marketplace_packages p ON p.id=pi.package_id WHERE c.id=$1 AND c.status='requested' AND p.status<>'cancelled' ORDER BY p.id`, id)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	err = errors.Join(err, rows.Err())
	rows.Close()
	if err != nil {
		return nil, err
	}
	var result []ports.MarketplacePackage
	for _, id := range ids {
		s, e := r.ShippingOrder(ctx, id)
		if e != nil {
			return nil, e
		}
		result = append(result, ports.MarketplacePackage{ID: id, Shipment: s.Shipment})
	}
	return result, nil
}
func (r *MarketplaceRepository) CompleteCancellation(ctx context.Context, id int64) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var order, item, seller, variant, paid int64
		var qty, total, already int
		var reason, key, status string
		if err := tx.QueryRow(ctx, `SELECT o.id FROM orders o JOIN marketplace_cancellations c ON c.order_id=o.id WHERE c.id=$1 FOR UPDATE OF o`, id).Scan(&order); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(4856203)`); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `SELECT c.order_item_id,oi.seller_id,oi.variant_id,c.quantity,oi.quantity,oi.cancelled_quantity,oi.line_total_cents-COALESCE(oi.allocated_discount_cents,0),c.reason,c.request_key,c.status FROM marketplace_cancellations c JOIN order_items oi ON oi.id=c.order_item_id WHERE c.id=$1 FOR UPDATE OF c,oi`, id).Scan(&item, &seller, &variant, &qty, &total, &already, &paid, &reason, &key, &status)
		if err != nil {
			return err
		}
		if status == "completed" {
			return nil
		}
		if status != "requested" {
			return ports.ErrInvalidState
		}
		var blocked bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_package_items pi JOIN marketplace_packages p ON p.id=pi.package_id WHERE pi.order_item_id=$1 AND p.status<>'cancelled' AND (p.provider_order_id IS NOT NULL OR p.pending_action<>'')) OR EXISTS(SELECT 1 FROM seller_fulfillments WHERE order_id=$2 AND seller_id=$3 AND status IN ('shipped','out_for_delivery','delivered'))`, item, order, seller).Scan(&blocked); err != nil {
			return err
		}
		if blocked {
			return ports.ErrInvalidState
		}
		var refundUnits int
		if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0) FROM marketplace_refunds WHERE order_item_id=$1 AND status<>'rejected'`, item).Scan(&refundUnits); err != nil {
			return err
		}
		amount, e := domain.AllocateRefund(paid, total, refundUnits, qty)
		if e != nil || amount <= 0 {
			return ports.ErrRefundState
		}
		// Locally unsubmitted packages can be cancelled immediately. Provider-backed
		// packages were already confirmed cancelled before entering this transaction.
		if _, err = tx.Exec(ctx, `UPDATE marketplace_packages p SET status='cancelled',provider_status='CANCELLED' WHERE p.provider_order_id IS NULL AND p.pending_action='' AND EXISTS(SELECT 1 FROM marketplace_package_items pi WHERE pi.package_id=p.id AND pi.order_item_id=$1)`, item); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE order_items SET cancelled_quantity=cancelled_quantity+$2 WHERE id=$1`, item, qty); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE inventory_stock SET available_quantity=available_quantity+$2,updated_at=now() WHERE variant_id=$1`, variant, qty); err != nil {
			return err
		}
		var shipping int64
		if err = tx.QueryRow(ctx, `SELECT CASE WHEN NOT EXISTS(SELECT 1 FROM order_items WHERE order_id=o.id AND quantity>cancelled_quantity) AND NOT EXISTS(SELECT 1 FROM marketplace_packages WHERE order_id=o.id AND status IN ('picked_up','in_transit','out_for_delivery','delivered','ndr','rto')) THEN GREATEST(0,o.shipping_cents-COALESCE(o.shipping_discount_cents,0)-COALESCE((SELECT sum(shipping_cents) FROM marketplace_refunds WHERE order_id=o.id AND status<>'rejected'),0)) ELSE 0 END FROM orders o WHERE id=$1`, order).Scan(&shipping); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE marketplace_cancellations SET status='completed' WHERE id=$1`, id); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO marketplace_refunds(order_id,seller_id,order_item_id,quantity,amount_cents,shipping_cents,reason,kind,request_key) VALUES($1,$2,$3,$4,$5,$6,$7,'cancellation',$8)`, order, seller, item, qty, amount, shipping, reason, "cancel-refund:"+key); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE marketplace_cases mc SET status='resolved',resolution='Shipment cancellation verified',resolved_at=now(),next_action=''
			WHERE mc.order_id=$1 AND mc.seller_id=$2 AND mc.kind='cancellation_review' AND mc.status='open'
			AND mc.package_id IN (SELECT package_id FROM marketplace_package_items WHERE order_item_id=$3)
			AND EXISTS (SELECT 1 FROM marketplace_packages WHERE id=mc.package_id AND status='cancelled')`, order, seller, item); err != nil {
			return err
		}
		if err = marketplaceCase(ctx, tx, order, seller, 0, "refund_request", "Cancelled items require refund approval"); err != nil {
			return err
		}
		// Remaining items return to packing for explicit confirmation of the final
		// package contents/dimensions; no stale package is silently reused.
		if _, err = tx.Exec(ctx, `UPDATE seller_fulfillments SET lifecycle_status='packing',status='packing',revision=revision+1 WHERE order_id=$1 AND seller_id=$2 AND lifecycle_status='ready_to_ship'`, order, seller); err != nil {
			return err
		}
		return refreshMarketplaceAggregate(ctx, tx, order, seller)
	})
}

var _ ports.MarketplaceStore = (*MarketplaceRepository)(nil)
var _ = fmt.Sprint
