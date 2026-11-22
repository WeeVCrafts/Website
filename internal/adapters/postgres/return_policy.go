package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *MarketplaceRepository) PolicySettings(ctx context.Context) (ports.PolicySettings, error) {
	var p ports.PolicySettings
	err := r.readSource().QueryRow(ctx, `SELECT published_at,version,shipping_mode,live_catalog_scope,COALESCE(live_catalog_seller_id,0),COALESCE(pilot_user_id,0),COALESCE(pilot_variant_id,0),COALESCE(pilot_order_id,0) FROM marketplace_policy_settings WHERE id`).Scan(&p.PublishedAt, &p.Version, &p.ShippingMode, &p.LiveCatalogScope, &p.LiveCatalogSellerID, &p.PilotUserID, &p.PilotVariantID, &p.PilotOrderID)
	return p, err
}

func (r *MarketplaceRepository) ConfigurePolicy(ctx context.Context, actor int64, action, reason string, user, variant int64) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		a, err := marketplaceIdentity(ctx, tx, actor)
		if err != nil {
			return err
		}
		if a.role != "super_admin" || len(strings.TrimSpace(reason)) < 10 {
			return ports.ErrForbidden
		}
		if _, err = tx.Exec(ctx, `SELECT id FROM marketplace_policy_settings WHERE id FOR UPDATE`); err != nil {
			return err
		}
		switch action {
		case "configure_pilot", "activate_ready_catalog", "activate_pilot_seller", "restore_limited_catalog":
			var automatic bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_checkout_health WHERE id)`).Scan(&automatic); err != nil {
				return err
			}
			if automatic {
				return ports.ErrInvalidState
			}
		}
		switch action {
		case "publish_policy":
			content, _ := json.Marshal(domain.ReturnPolicySections())
			if _, err = tx.Exec(ctx, `INSERT INTO marketplace_policy_versions(version,content,published_at,published_by) VALUES($1,$2,now(),$3) ON CONFLICT(version) DO NOTHING`, domain.ReturnPolicyVersion, content, actor); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE marketplace_policy_settings SET version=$1,published_at=COALESCE(published_at,now()),published_by=$2,updated_at=now() WHERE id`, domain.ReturnPolicyVersion, actor)
		case "pause_shipping":
			_, err = tx.Exec(ctx, `UPDATE marketplace_policy_settings SET shipping_mode='off',checkout_paused=true,updated_at=now() WHERE id`)
		case "resume_checkout":
			result, e := tx.Exec(ctx, `UPDATE marketplace_policy_settings SET shipping_mode='live',checkout_paused=false,updated_at=now()
				WHERE id AND checkout_paused AND EXISTS(SELECT 1 FROM marketplace_checkout_health WHERE id)`)
			if e != nil {
				return e
			}
			if result.RowsAffected() != 1 {
				return ports.ErrInvalidState
			}
		case "activate_ready_catalog":
			result, e := tx.Exec(ctx, `UPDATE marketplace_policy_settings m
				SET shipping_mode='live',live_catalog_scope='ready',live_catalog_seller_id=NULL,updated_at=now()
				WHERE m.id AND m.published_at IS NOT NULL AND EXISTS (`+verifiedDeliveryPilotSQL+`)`)
			err = e
			if err == nil && result.RowsAffected() != 1 {
				return ports.ErrInvalidState
			}
		case "activate_pilot_seller":
			result, e := tx.Exec(ctx, `UPDATE marketplace_policy_settings m
				SET shipping_mode='live',live_catalog_scope='seller',live_catalog_seller_id=target.seller_id,updated_at=now()
				FROM marketplace_ready_variants target
				WHERE m.id AND m.published_at IS NOT NULL AND target.variant_id=$1 AND target.stock>0
				  AND EXISTS (`+verifiedDeliveryPilotSQL+` AND EXISTS (
					SELECT 1 FROM marketplace_packages fulfilled
					WHERE fulfilled.order_id=o.id AND fulfilled.seller_id=target.seller_id
					  AND fulfilled.delivery_method='shiprocket' AND fulfilled.status='delivered'
					  AND fulfilled.provider_shipment_id IS NOT NULL AND fulfilled.awb<>''))`, variant)
			err = e
			if err == nil && result.RowsAffected() != 1 {
				return ports.ErrInvalidState
			}
		case "restore_limited_catalog":
			_, err = tx.Exec(ctx, `UPDATE marketplace_policy_settings SET live_catalog_scope='limited',live_catalog_seller_id=NULL,updated_at=now() WHERE id`)
		case "configure_pilot":
			var ready bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.id=$1 AND u.status='active' AND ur.role_slug='customer')
				AND EXISTS(SELECT 1 FROM marketplace_ready_variants WHERE variant_id=$2 AND stock>0)
				AND EXISTS(SELECT 1 FROM marketplace_policy_settings WHERE id AND published_at IS NOT NULL AND pilot_order_id IS NULL)`, user, variant).Scan(&ready)
			if err != nil {
				return err
			}
			if !ready {
				return ports.ErrInvalidState
			}
			_, err = tx.Exec(ctx, `UPDATE marketplace_policy_settings SET shipping_mode='pilot',pilot_user_id=$1,pilot_variant_id=$2,updated_at=now() WHERE id`, user, variant)
		case "classify_variant":
			if reason != "textile_craft" && reason != "fragile_art" {
				return ports.ErrInvalidState
			}
			result, e := tx.Exec(ctx, `UPDATE product_variants SET return_class=$2 WHERE id=$1`, variant, reason)
			err = e
			if err == nil && result.RowsAffected() != 1 {
				return ports.ErrNotFound
			}
		default:
			return ports.ErrInvalidState
		}
		if err != nil {
			return err
		}
		return marketplaceAudit(ctx, tx, actor, 0, action, reason)
	})
}

const verifiedDeliveryPilotSQL = `SELECT 1 FROM orders o
	WHERE o.id=m.pilot_order_id AND o.status='delivered' AND marketplace_order_funded(o.id)
	AND EXISTS (SELECT 1 FROM payment_attempts p WHERE p.order_id=o.id AND p.provider='razorpay'
		AND p.status='captured' AND p.amount_cents>0 AND COALESCE(p.provider_payment_id,'')<>'')
	AND EXISTS (SELECT 1 FROM marketplace_packages p WHERE p.order_id=o.id AND p.delivery_method='shiprocket'
		AND p.status='delivered' AND p.provider_shipment_id IS NOT NULL AND p.awb<>'')`

func (r *MarketplaceRepository) ConfigureLiveCatalog(ctx context.Context, actor, first, second int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if first <= 0 || second <= 0 || first == second || len(reason) < 10 || len(reason) > 1000 {
		return ports.ErrInvalidState
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		a, err := marketplaceIdentity(ctx, tx, actor)
		if err != nil {
			return err
		}
		if a.role != "super_admin" {
			return ports.ErrForbidden
		}
		var published bool
		if err = tx.QueryRow(ctx, `SELECT published_at IS NOT NULL AND NOT EXISTS(SELECT 1 FROM marketplace_checkout_health WHERE id)
			FROM marketplace_policy_settings WHERE id FOR UPDATE`).Scan(&published); err != nil {
			return err
		}
		var ready bool
		if err = tx.QueryRow(ctx, `SELECT
			(SELECT count(DISTINCT seller_id)=2 FROM marketplace_ready_variants WHERE variant_id IN ($1,$2) AND stock>0)
			AND EXISTS(`+verifiedDeliveryPilotSQL+`) FROM marketplace_policy_settings m WHERE m.id`, first, second).Scan(&ready); err != nil {
			return err
		}
		if !published || !ready {
			return ports.ErrInvalidState
		}
		if _, err = tx.Exec(ctx, `DELETE FROM marketplace_launch_variants WHERE variant_id NOT IN ($1,$2)`, first, second); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO marketplace_launch_variants(variant_id) VALUES($1),($2) ON CONFLICT DO NOTHING`, first, second); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE marketplace_policy_settings SET shipping_mode='live',live_catalog_scope='limited',live_catalog_seller_id=NULL,updated_at=now() WHERE id`); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata)
			VALUES($1,'marketplace.configure_live_catalog','marketplace_policy','1',$2,
				jsonb_build_object('reason',$3::text,'variant_ids',jsonb_build_array($4::bigint,$5::bigint)))`,
			fmt.Sprint(actor), ports.RequestID(ctx), reason, first, second)
		return err
	})
}

func (r *MarketplaceRepository) policyView(ctx context.Context, actor int64, a marketplaceActor, v *ports.MarketplaceView) error {
	var err error
	v.Policy, err = r.PolicySettings(ctx)
	if err != nil {
		return err
	}
	if a.role == "customer" && v.OrderID == 0 {
		v.CreditAvailableCents, err = creditAvailable(ctx, r.readSource(), actor)
		if err != nil {
			return err
		}
		rows, e := r.readSource().Query(ctx, `SELECT id,amount_cents,remaining_cents,expires_at FROM store_credit_grants WHERE user_id=$1 ORDER BY expires_at,id LIMIT 100`, actor)
		if e != nil {
			return e
		}
		for rows.Next() {
			var g ports.StoreCreditGrant
			if e = rows.Scan(&g.ID, &g.AmountCents, &g.RemainingCents, &g.ExpiresAt); e != nil {
				break
			}
			v.CreditGrants = append(v.CreditGrants, g)
		}
		if e = errorsJoinRows(e, rows); e != nil {
			return e
		}
	}
	if v.OrderID == 0 {
		return nil
	}
	if err := r.readSource().QueryRow(ctx, `SELECT COALESCE(policy_snapshot ? 'return_policy_version',false) FROM orders WHERE id=$1`, v.OrderID).Scan(&v.NewReturnPolicy); err != nil {
		return err
	}
	rows, err := r.readSource().Query(ctx, `SELECT id,order_item_id,seller_id,quantity,kind,resolution,status,reason,evidence_exception,inspection_note,return_awb,return_courier,return_status,amount_cents,fee_cents,COALESCE(replacement_package_id,0),submitted_at FROM marketplace_claims WHERE order_id=$1 AND ($2=0 OR seller_id=$2) ORDER BY id`, v.OrderID, a.seller)
	if err != nil {
		return err
	}
	for rows.Next() {
		var c ports.PolicyClaim
		if err = rows.Scan(&c.ID, &c.ItemID, &c.SellerID, &c.Quantity, &c.Kind, &c.Resolution, &c.Status, &c.Reason, &c.EvidenceException, &c.InspectionNote, &c.ReturnAWB, &c.ReturnCourier, &c.ReturnStatus, &c.AmountCents, &c.FeeCents, &c.ReplacementPackageID, &c.SubmittedAt); err != nil {
			break
		}
		v.Claims = append(v.Claims, c)
	}
	if err = errorsJoinRows(err, rows); err != nil {
		return err
	}
	for i := range v.Claims {
		evidence, e := r.readSource().Query(ctx, `SELECT e.id,e.filename,e.content_type FROM marketplace_claim_evidence e JOIN marketplace_claims c ON c.id=e.claim_id WHERE c.id=$1 AND ($2 OR c.evidence_shared) ORDER BY e.id`, v.Claims[i].ID, a.role != "seller")
		if e != nil {
			return e
		}
		for evidence.Next() {
			var item ports.ClaimEvidence
			if e = evidence.Scan(&item.ID, &item.Filename, &item.ContentType); e != nil {
				break
			}
			v.Claims[i].Evidence = append(v.Claims[i].Evidence, item)
		}
		if e = errorsJoinRows(e, evidence); e != nil {
			return e
		}
	}
	return nil
}

func policyClaimAction(ctx context.Context, tx pgx.Tx, actor, order int64, a marketplaceActor, c ports.MarketplaceCommand) error {
	if len(c.Key) < 16 || len(strings.TrimSpace(c.Reason)) < 10 {
		return ports.ErrInvalidState
	}
	if c.Action == "submit_claim" {
		if a.role != "customer" && !a.company() {
			return ports.ErrForbidden
		}
		if c.Quantity < 1 {
			return ports.ErrInvalidState
		}
		if c.Kind != "damage" && c.Kind != "defect" && c.Kind != "wrong_item" && c.Kind != "change_of_mind" {
			return ports.ErrInvalidState
		}
		var existing int64
		var match bool
		if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT id FROM marketplace_claims WHERE request_key=$1),0),EXISTS(SELECT 1 FROM marketplace_claims WHERE request_key=$1 AND order_id=$2 AND order_item_id=$3 AND quantity=$4 AND kind=$5 AND reason=$6 AND delivery_package_id=$7)`, c.Key, order, c.ItemID, c.Quantity, c.Kind, c.Reason, c.PackageID).Scan(&existing, &match); err != nil {
			return err
		}
		if existing > 0 {
			if match {
				return nil
			}
			return ports.ErrIdempotencyConflict
		}
		var delivered *time.Time
		var seller, paid int64
		var total, used int
		var class, version string
		err := tx.QueryRow(ctx, `SELECT oi.seller_id,oi.quantity,oi.line_total_cents-COALESCE(oi.allocated_discount_cents,0),oi.return_class,COALESCE(o.policy_snapshot->>'return_policy_version',''),(SELECT p.delivered_at FROM marketplace_packages p JOIN marketplace_package_items pi ON pi.package_id=p.id WHERE p.id=$3 AND pi.order_item_id=oi.id AND p.order_id=oi.order_id AND p.status='delivered' AND p.replacement_claim_id IS NULL),oi.refunded_quantity+oi.compensated_quantity-COALESCE((SELECT sum(c.quantity) FROM marketplace_claims c JOIN marketplace_refunds mr ON mr.id=c.refund_id AND mr.status='processed' WHERE c.order_item_id=oi.id AND c.status='completed'),0)+COALESCE((SELECT sum(quantity) FROM marketplace_claims WHERE order_item_id=oi.id AND status NOT IN ('completed','rejected')),0)+COALESCE((SELECT sum(quantity) FROM marketplace_refunds mr WHERE order_item_id=oi.id AND status NOT IN ('processed','rejected') AND NOT EXISTS(SELECT 1 FROM marketplace_claims c WHERE c.refund_id=mr.id)),0) FROM order_items oi JOIN orders o ON o.id=oi.order_id LEFT JOIN seller_fulfillments sf ON sf.order_id=o.id AND sf.seller_id=oi.seller_id WHERE oi.id=$1 AND oi.order_id=$2 FOR UPDATE OF oi`, c.ItemID, order, c.PackageID).Scan(&seller, &total, &paid, &class, &version, &delivered, &used)
		if err != nil {
			return err
		}
		if version != domain.ReturnPolicyVersion || delivered == nil || !domain.ClaimWithinWindow(c.Kind, *delivered, time.Now()) || c.Quantity > total-used {
			return ports.ErrInvalidState
		}
		var packageRemaining int
		if err := tx.QueryRow(ctx, `SELECT pi.quantity-COALESCE((SELECT sum(quantity) FROM marketplace_claims WHERE delivery_package_id=pi.package_id AND order_item_id=pi.order_item_id AND status<>'rejected'),0) FROM marketplace_package_items pi WHERE pi.package_id=$1 AND pi.order_item_id=$2`, c.PackageID, c.ItemID).Scan(&packageRemaining); err != nil {
			return err
		}
		if c.Quantity > packageRemaining {
			return ports.ErrInvalidState
		}
		resolution := "replacement"
		fee := int64(0)
		if c.Kind == "change_of_mind" {
			if class != "textile_craft" {
				return ports.ErrInvalidState
			}
			resolution = "credit"
			fee = domain.ReversePickupFeeCents
		}
		amount, err := domain.AllocateRefund(paid, total, used, c.Quantity)
		if err != nil {
			return err
		}
		if resolution == "credit" && amount <= fee {
			return ports.ErrInvalidState
		}
		if _, err = tx.Exec(ctx, `INSERT INTO marketplace_claims(order_id,seller_id,order_item_id,quantity,kind,resolution,reason,request_key,policy_version,amount_cents,fee_cents,delivery_package_id,review_due_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, order, seller, c.ItemID, c.Quantity, c.Kind, resolution, c.Reason, c.Key, version, amount, fee, c.PackageID, domain.BusinessDeadline(time.Now(), 48)); err != nil {
			return err
		}
		return marketplaceCase(ctx, tx, order, seller, 0, "return_dispute", c.Reason)
	}
	var current, resolution, kind string
	var seller, item, amount, fee, variant int64
	var quantity int
	if err := tx.QueryRow(ctx, `SELECT c.status,c.resolution,c.kind,c.seller_id,c.order_item_id,c.quantity,c.amount_cents,c.fee_cents,oi.variant_id FROM marketplace_claims c JOIN order_items oi ON oi.id=c.order_item_id WHERE c.id=$1 AND c.order_id=$2 FOR UPDATE OF c,oi`, c.CaseID, order).Scan(&current, &resolution, &kind, &seller, &item, &quantity, &amount, &fee, &variant); err != nil {
		return err
	}
	if c.Action == "prepare_replacement" {
		return prepareReplacement(ctx, tx, actor, order, a, c, current, resolution, seller, item, quantity, variant)
	}
	if !a.company() {
		return ports.ErrForbidden
	}
	if c.Action == "share_claim_evidence" {
		_, err := tx.Exec(ctx, `UPDATE marketplace_claims SET evidence_shared=true,updated_at=now() WHERE id=$1`, c.CaseID)
		return err
	}
	if c.Status == current {
		return nil
	}
	if !domain.CanTransition("return", current, c.Status) {
		return ports.ErrInvalidState
	}
	if c.Status == "approved" {
		if a.role != "super_admin" {
			return ports.ErrForbidden
		}
		var video bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_claim_evidence WHERE claim_id=$1 AND content_type LIKE 'video/%')`, c.CaseID).Scan(&video); err != nil {
			return err
		}
		if kind != "change_of_mind" && !video && len(strings.TrimSpace(c.EvidenceException)) < 10 {
			return ports.ErrInvalidState
		}
		if resolution == "replacement" {
			var available int
			if err := tx.QueryRow(ctx, `SELECT available_quantity FROM inventory_stock WHERE variant_id=$1 FOR UPDATE`, variant).Scan(&available); err != nil {
				return err
			}
			if available < quantity {
				resolution = "refund"
			} else {
				if _, err := tx.Exec(ctx, `UPDATE inventory_stock SET available_quantity=available_quantity-$2,reserved_quantity=reserved_quantity+$2 WHERE variant_id=$1`, variant, quantity); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `UPDATE marketplace_claims SET replacement_reserved=true WHERE id=$1`, c.CaseID); err != nil {
					return err
				}
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE marketplace_claims SET resolution=$2,evidence_exception=$3,approved_by=$4,pickup_due_at=$5 WHERE id=$1`, c.CaseID, resolution, c.EvidenceException, actor, domain.BusinessDeadline(time.Now(), 40)); err != nil {
			return err
		}
	}
	if c.Status == "pickup_pending" {
		if !c.Parcel.Valid() || c.WeightGrams < 100 {
			return ports.ErrShippingInput
		}
		if _, err := tx.Exec(ctx, `UPDATE marketplace_claims SET weight_grams=$2,length_cm=$3,width_cm=$4,height_cm=$5 WHERE id=$1`, c.CaseID, c.WeightGrams, c.LengthCM, c.WidthCM, c.HeightCM); err != nil {
			return err
		}
		// Reverse booking is a durable operation; provider confirmation owns the status.
		_, err := tx.Exec(ctx, `SELECT marketplace_enqueue('return_pickup',$1,$2)`, c.CaseID, fmt.Sprintf("return:%d", c.CaseID))
		return err
	}
	if c.Status == "received" {
		var verified bool
		if err := tx.QueryRow(ctx, `SELECT return_status IN ('DELIVERED','RETURN DELIVERED') FROM marketplace_claims WHERE id=$1`, c.CaseID).Scan(&verified); err != nil {
			return err
		}
		if !verified {
			return ports.ErrInvalidState
		}
	}
	if c.Status == "inspected" && (len(c.Reason) < 10 || !c.InspectionPassed) {
		return ports.ErrInvalidState
	}
	if c.Status == "completed" {
		if a.role != "super_admin" {
			return ports.ErrForbidden
		}
		if resolution == "replacement" {
			return ports.ErrInvalidState
		}
		var passed, restock bool
		if err := tx.QueryRow(ctx, `SELECT inspection_passed,restock FROM marketplace_claims WHERE id=$1`, c.CaseID).Scan(&passed, &restock); err != nil {
			return err
		}
		if !passed {
			return ports.ErrInvalidState
		}
		c.Restock = restock
		if err := completePolicyClaim(ctx, tx, actor, order, c.CaseID, item, seller, quantity, amount, fee, resolution, c.Key, c.Reason, c.Restock); err != nil {
			return err
		}
	}
	if c.Status == "rejected" && resolution == "replacement" && current != "requested" {
		var booked bool
		if err := tx.QueryRow(ctx, `SELECT replacement_package_id IS NOT NULL FROM marketplace_claims WHERE id=$1`, c.CaseID).Scan(&booked); err != nil {
			return err
		}
		if booked {
			return ports.ErrInvalidState
		}
		if _, err := tx.Exec(ctx, `UPDATE inventory_stock SET available_quantity=available_quantity+$2,reserved_quantity=reserved_quantity-$2 WHERE variant_id=$1 AND reserved_quantity>=$2 AND EXISTS(SELECT 1 FROM marketplace_claims WHERE id=$3 AND replacement_reserved AND NOT replacement_committed AND replacement_package_id IS NULL)`, variant, quantity, c.CaseID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE marketplace_claims SET replacement_reserved=false WHERE id=$1 AND replacement_package_id IS NULL`, c.CaseID); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `UPDATE marketplace_claims SET status=$2,inspection_note=CASE WHEN $2='inspected' THEN $3 ELSE inspection_note END,restock=CASE WHEN $2='inspected' THEN $4 ELSE restock END,inspection_passed=CASE WHEN $2='inspected' THEN $5 ELSE inspection_passed END,resolution_due_at=CASE WHEN $2='inspected' THEN $6 ELSE resolution_due_at END,updated_at=now() WHERE id=$1`, c.CaseID, c.Status, c.Reason, c.Restock, c.InspectionPassed, domain.BusinessDeadline(time.Now(), 70))
	if err == nil && (c.Status == "completed" || c.Status == "rejected") {
		err = resolvePolicyClaimCase(ctx, tx, order, seller)
	}
	return err
}

func resolvePolicyClaimCase(ctx context.Context, tx pgx.Tx, order, seller int64) error {
	_, err := tx.Exec(ctx, `UPDATE marketplace_cases SET status='resolved',resolution='All return claims decided',resolved_at=now()
		WHERE order_id=$1 AND seller_id=$2 AND package_id IS NULL AND status='open' AND kind='return_dispute'
		AND NOT EXISTS(SELECT 1 FROM marketplace_claims WHERE order_id=$1 AND seller_id=$2 AND status NOT IN ('completed','rejected'))
		AND NOT EXISTS(SELECT 1 FROM marketplace_returns WHERE order_id=$1 AND seller_id=$2 AND status NOT IN ('completed','rejected'))`, order, seller)
	return err
}

func completePolicyClaim(ctx context.Context, tx pgx.Tx, actor, order, claim, item, seller int64, quantity int, amount, fee int64, resolution, key, reason string, restock bool) error {
	var user int64
	var number string
	if err := tx.QueryRow(ctx, `SELECT user_id,order_number FROM orders WHERE id=$1`, order).Scan(&user, &number); err != nil {
		return err
	}
	if resolution == "credit" {
		var id int64
		expiry := domain.CreditExpiry(time.Now().In(time.FixedZone("IST", 19800)))
		if err := tx.QueryRow(ctx, `INSERT INTO store_credit_grants(user_id,claim_id,amount_cents,remaining_cents,expires_at) VALUES($1,$2,$3,$3,$4) RETURNING id`, user, claim, amount-fee, expiry).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO store_credit_history(user_id,grant_id,order_id,amount_cents,kind,source_key) VALUES($1,$2,$3,$4,'issued',$5)`, user, id, order, amount-fee, fmt.Sprintf("claim:%d", claim)); err != nil {
			return err
		}
		var commission int64
		var total, previous int
		if err := tx.QueryRow(ctx, `SELECT oi.quantity,oi.financially_reversed_quantity,COALESCE((SELECT (line->>'commission_cents')::bigint FROM orders o CROSS JOIN LATERAL jsonb_array_elements(o.seller_finance_snapshot->'lines') line WHERE o.id=oi.order_id AND (line->>'order_item_id')::bigint=oi.id),0) FROM order_items oi WHERE oi.id=$1`, item).Scan(&total, &previous, &commission); err != nil {
			return err
		}
		commissionRefund, err := domain.AllocateRefund(commission, total, previous, quantity)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO seller_ledger(seller_id,order_id,entry_type,gross_cents,commission_cents,net_cents,status,note,policy_claim_id) VALUES($1,$2,'refund',$3,$4,$5,'open',$6,$7)`, seller, order, -amount, -commissionRefund, -(amount - commissionRefund), fmt.Sprintf("Approved store-credit return %d", claim), claim); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE order_items SET financially_reversed_quantity=financially_reversed_quantity+$2 WHERE id=$1`, item, quantity); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `SELECT marketplace_enqueue('claim_credit_note',$1,$2)`, claim, fmt.Sprintf("claim-credit-note:%d", claim)); err != nil {
			return err
		}
	} else if resolution == "refund" {
		c := ports.MarketplaceCommand{ItemID: item, Quantity: quantity, Reason: reason, Key: fmt.Sprintf("claim-refund:%d", claim)}
		if err := requestExceptionRefund(ctx, tx, order, c); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE marketplace_claims SET refund_id=(SELECT id FROM marketplace_refunds WHERE request_key=$2) WHERE id=$1`, claim, c.Key); err != nil {
			return err
		}
		// Delivery charges are added once, only after every purchased unit has an
		// approved damage/defect/wrong-item refund. Credit returns never qualify.
		if _, err := tx.Exec(ctx, `UPDATE marketplace_refunds mr SET shipping_cents=GREATEST(0,o.shipping_cents-COALESCE(o.shipping_discount_cents,0)-COALESCE((SELECT sum(shipping_cents) FROM marketplace_refunds WHERE order_id=o.id AND status<>'rejected'),0)) FROM orders o WHERE mr.order_id=o.id AND mr.request_key=$1 AND NOT EXISTS(SELECT 1 FROM order_items oi WHERE oi.order_id=o.id AND oi.quantity>COALESCE((SELECT sum(c.quantity) FROM marketplace_claims c WHERE c.order_item_id=oi.id AND c.resolution='refund' AND c.kind<>'change_of_mind' AND (c.status='completed' OR c.id=$2)),0))`, c.Key, claim); err != nil {
			return err
		}
	} else {
		return ports.ErrInvalidState
	}
	if _, err := tx.Exec(ctx, `UPDATE order_items SET compensated_quantity=compensated_quantity+$2 WHERE id=$1 AND compensated_quantity+$2<=quantity`, item, quantity); err != nil {
		return err
	}
	if restock {
		if _, err := tx.Exec(ctx, `UPDATE inventory_stock i SET available_quantity=available_quantity+$2 FROM order_items oi WHERE oi.id=$1 AND i.variant_id=oi.variant_id`, item, quantity); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO notifications(user_id,kind,title,body,link_url) VALUES($1,'order','Return inspection completed',$2,$3)`, user, "Your approved return resolution is recorded. Open your order for the actual credit or refund status.", "/orders/"+number+"/fulfillment")
	return err
}

func prepareReplacement(ctx context.Context, tx pgx.Tx, actor, order int64, a marketplaceActor, c ports.MarketplaceCommand, current, resolution string, seller, item int64, quantity int, variant int64) error {
	if a.role != "seller" || a.seller != seller || resolution != "replacement" || current != "inspected" || !c.Parcel.Valid() || c.WeightGrams < 100 || len(c.QCEvidence) < 10 {
		return ports.ErrInvalidState
	}
	var existing int64
	var reserved bool
	if err := tx.QueryRow(ctx, `SELECT COALESCE(replacement_package_id,0),replacement_reserved FROM marketplace_claims WHERE id=$1`, c.CaseID).Scan(&existing, &reserved); err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	var passed bool
	if err := tx.QueryRow(ctx, `SELECT inspection_passed FROM marketplace_claims WHERE id=$1`, c.CaseID).Scan(&passed); err != nil {
		return err
	}
	if !passed {
		return ports.ErrInvalidState
	}
	if !reserved {
		return ports.ErrInsufficientStock
	}
	var pickup string
	if err := tx.QueryRow(ctx, `SELECT pickup_location FROM marketplace_shipping_profiles WHERE seller_id=$1 AND verified`, seller).Scan(&pickup); err != nil {
		return err
	}
	var parcel int64
	if err := tx.QueryRow(ctx, `INSERT INTO marketplace_packages(order_id,seller_id,reference,pickup_location,weight_grams,length_cm,width_cm,height_cm,requested_by,replacement_claim_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`, order, seller, fmt.Sprintf("WC-RPL-%d", c.CaseID), pickup, c.WeightGrams, c.LengthCM, c.WidthCM, c.HeightCM, actor, c.CaseID).Scan(&parcel); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO marketplace_package_items(package_id,order_item_id,quantity) VALUES($1,$2,$3)`, parcel, item, quantity); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE marketplace_claims SET replacement_package_id=$2,updated_at=now() WHERE id=$1`, c.CaseID, parcel); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE seller_fulfillments SET qc_evidence=$3 WHERE order_id=$1 AND seller_id=$2`, order, seller, c.QCEvidence); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `SELECT marketplace_enqueue('book_package',$1,$2)`, parcel, fmt.Sprintf("replacement:%d", c.CaseID))
	return err
}
