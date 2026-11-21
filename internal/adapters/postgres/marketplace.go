package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

type MarketplaceRepository struct {
	pool          *pgxpool.Pool
	readTx        pgx.Tx
	renderInvoice func(ports.InvoiceDocument) ([]byte, error)
}

func NewMarketplaceRepository(pool *pgxpool.Pool) *MarketplaceRepository {
	return &MarketplaceRepository{pool: pool}
}
func (r *MarketplaceRepository) WithInvoiceRenderer(renderer func(ports.InvoiceDocument) ([]byte, error)) *MarketplaceRepository {
	r.renderInvoice = renderer
	return r
}

type marketplaceActor struct {
	role   string
	seller int64
}

func marketplaceIdentity(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, actor int64) (marketplaceActor, error) {
	var a marketplaceActor
	err := q.QueryRow(ctx, `SELECT ur.role_slug,COALESCE(s.id,0) FROM users u JOIN user_roles ur ON ur.user_id=u.id LEFT JOIN sellers s ON s.owner_user_id=u.id AND s.status='active' WHERE u.id=$1 AND u.status='active'`, actor).Scan(&a.role, &a.seller)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ports.ErrForbidden
	}
	return a, err
}
func (a marketplaceActor) company() bool { return a.role == "admin" || a.role == "super_admin" }

func (r *MarketplaceRepository) View(ctx context.Context, actor int64, number string) (ports.MarketplaceView, error) {
	return r.ViewPage(ctx, actor, number, 0, 0, "")
}
func (r *MarketplaceRepository) ViewPage(ctx context.Context, actor int64, number string, before, caseBefore int64, queue string) (ports.MarketplaceView, error) {
	var v ports.MarketplaceView
	a, err := marketplaceIdentity(ctx, r.readSource(), actor)
	if err != nil {
		return v, err
	}
	v.Role = a.role
	if err := r.policyView(ctx, actor, a, &v); err != nil {
		return v, err
	}
	if number == "" {
		rows, e := r.readSource().Query(ctx, `SELECT o.id,o.order_number,o.lifecycle_status,
			CASE WHEN $2='seller' AND o.workflow_version=2 THEN
				CASE WHEN o.lifecycle_status='awaiting_payment' THEN 'awaiting_payment'
				     ELSE COALESCE((SELECT sf.lifecycle_status FROM seller_fulfillments sf WHERE sf.order_id=o.id AND sf.seller_id=$4),'') END
			ELSE '' END
			FROM orders o WHERE ($1 OR ($2='customer' AND o.user_id=$3) OR ($2='seller' AND EXISTS(SELECT 1 FROM order_items WHERE order_id=o.id AND seller_id=$4))) AND ($5=0 OR o.id<$5) ORDER BY o.id DESC LIMIT 51`, a.company(), a.role, actor, a.seller, before)
		if e != nil {
			return v, e
		}
		for rows.Next() {
			var x ports.MarketplaceOrderSummary
			if e = rows.Scan(&x.ID, &x.Number, &x.Status, &x.SellerStage); e != nil {
				break
			}
			v.Orders = append(v.Orders, x)
		}
		e = errorsJoinRows(e, rows)
		if e != nil {
			return v, e
		}
		if !a.company() {
			if len(v.Orders) > 50 {
				v.NextOrderID = v.Orders[49].ID
				v.Orders = v.Orders[:50]
			}
			return v, nil
		}
		if len(v.Orders) > 50 {
			v.NextOrderID = v.Orders[49].ID
			v.Orders = v.Orders[:50]
		}
		if queue == "" && caseBefore == 0 {
			v.Queue, e = r.fulfillmentQueue(ctx)
			if e != nil {
				return v, e
			}
		}
		rows, e = r.readSource().Query(ctx, `SELECT c.id,c.order_id,COALESCE(c.seller_id,0),COALESCE(c.package_id,0),o.order_number,c.kind,c.status,c.reason,c.created_at,COALESCE(u.display_name,''),c.next_action
			FROM marketplace_cases c JOIN orders o ON o.id=c.order_id LEFT JOIN users u ON u.id=c.assignee_id
			WHERE c.status='open' AND ($1='' OR c.kind=$1) AND ($2=0 OR c.id<$2) ORDER BY c.id DESC LIMIT 51`, queue, caseBefore)
		if e != nil {
			return v, e
		}
		defer rows.Close()
		for rows.Next() {
			var c ports.MarketplaceCase
			if e = rows.Scan(&c.ID, &c.OrderID, &c.SellerID, &c.PackageID, &c.Number, &c.Kind, &c.Status, &c.Reason, &c.CreatedAt, &c.Assignee, &c.NextAction); e != nil {
				return v, e
			}
			v.Cases = append(v.Cases, c)
		}
		e = errorsJoinRows(e, rows)
		if len(v.Cases) > 50 {
			v.NextCaseID = v.Cases[49].ID
			v.Cases = v.Cases[:50]
		}
		if e != nil {
			return v, e
		}
		rows, e = r.readSource().Query(ctx, `SELECT id,kind,status,last_error,attempts FROM marketplace_jobs WHERE status='failed' ORDER BY id DESC LIMIT 50`)
		if e != nil {
			return v, e
		}
		for rows.Next() {
			var j ports.MarketplaceJobSummary
			if e = rows.Scan(&j.ID, &j.Kind, &j.Status, &j.LastError, &j.Attempts); e != nil {
				break
			}
			v.Jobs = append(v.Jobs, j)
		}
		return v, errorsJoinRows(e, rows)
	}
	err = r.readSource().QueryRow(ctx, `SELECT o.id,o.order_number,o.lifecycle_status,o.currency,o.workflow_version,COALESCE((SELECT status FROM payment_attempts WHERE order_id=o.id ORDER BY id DESC LIMIT 1),'pending') FROM orders o WHERE o.order_number=$1 AND ($2 OR (o.user_id=$3 AND $4='customer') OR ($4='seller' AND EXISTS(SELECT 1 FROM order_items WHERE order_id=o.id AND seller_id=$5)))`, number, a.company(), actor, a.role, a.seller).Scan(&v.OrderID, &v.Number, &v.Status, &v.Currency, &v.WorkflowVersion, &v.Payment)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ports.ErrOrderNotFound
	}
	if err != nil {
		return v, err
	}
	var refunded, total int64
	if err := r.policyView(ctx, actor, a, &v); err != nil {
		return v, err
	}
	if err = r.readSource().QueryRow(ctx, `SELECT refunded_amount_cents,amount_cents FROM payment_attempts WHERE order_id=$1 ORDER BY id DESC LIMIT 1`, v.OrderID).Scan(&refunded, &total); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return v, err
	}
	if refunded > 0 {
		v.Payment = "partially_refunded"
		if refunded >= total {
			v.Payment = "refunded"
		}
	}
	seller := int64(0)
	if a.role == "seller" {
		seller = a.seller
	}
	returnRows, returnErr := r.readSource().Query(ctx, `SELECT id,order_item_id,seller_id,quantity,status,reason,carrier,tracking_number FROM marketplace_returns WHERE order_id=$1 AND ($2=0 OR seller_id=$2) ORDER BY id`, v.OrderID, seller)
	if returnErr != nil {
		return v, returnErr
	}
	for returnRows.Next() {
		var x ports.MarketplaceReturn
		if returnErr = returnRows.Scan(&x.ID, &x.ItemID, &x.SellerID, &x.Quantity, &x.Status, &x.Reason, &x.Carrier, &x.Tracking); returnErr != nil {
			break
		}
		v.Returns = append(v.Returns, x)
	}
	if returnErr = errorsJoinRows(returnErr, returnRows); returnErr != nil {
		return v, returnErr
	}
	rows, err := r.readSource().Query(ctx, `SELECT oi.id,COALESCE(oi.seller_id,0),oi.product_name,oi.sku,oi.quantity,oi.cancelled_quantity,oi.refunded_quantity,oi.line_total_cents-COALESCE(oi.allocated_discount_cents,0),(NOT EXISTS(SELECT 1 FROM orders co WHERE co.id=oi.order_id AND co.policy_snapshot ? 'return_policy_version' AND now()>co.created_at+interval '12 hours')) AND NOT EXISTS(SELECT 1 FROM marketplace_package_items pi JOIN marketplace_packages mp ON mp.id=pi.package_id WHERE pi.order_item_id=oi.id AND (mp.status NOT IN ('created','pickup_scheduled','cancelled') OR mp.pending_action<>'')) AND NOT EXISTS(SELECT 1 FROM seller_fulfillments sf WHERE sf.order_id=oi.order_id AND sf.seller_id=oi.seller_id AND sf.status IN ('shipped','out_for_delivery','delivered')),(EXISTS(SELECT 1 FROM marketplace_package_items pi JOIN marketplace_packages mp ON mp.id=pi.package_id WHERE pi.order_item_id=oi.id AND mp.status='delivered' AND mp.replacement_claim_id IS NULL) OR EXISTS(SELECT 1 FROM seller_fulfillments sf WHERE sf.order_id=oi.order_id AND sf.seller_id=oi.seller_id AND sf.status='delivered')) FROM order_items oi WHERE oi.order_id=$1 AND ($2=0 OR oi.seller_id=$2) ORDER BY oi.id`, v.OrderID, seller)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var x ports.MarketplaceLine
		if err = rows.Scan(&x.ID, &x.SellerID, &x.Name, &x.SKU, &x.Quantity, &x.Cancelled, &x.Refunded, &x.PaidCents, &x.CanCancel, &x.CanReturn); err != nil {
			break
		}
		v.Lines = append(v.Lines, x)
	}
	err = errors.Join(err, rows.Err())
	rows.Close()
	if err != nil {
		return v, err
	}
	rows, err = r.readSource().Query(ctx, `SELECT sf.seller_id,sf.revision,s.display_name,sf.lifecycle_status,COALESCE(d.decision,'manual') FROM seller_fulfillments sf JOIN sellers s ON s.id=sf.seller_id LEFT JOIN order_delivery_decisions d ON d.order_id=sf.order_id AND d.seller_id=sf.seller_id WHERE sf.order_id=$1 AND ($2=0 OR sf.seller_id=$2) ORDER BY sf.seller_id`, v.OrderID, seller)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var x ports.MarketplaceSellerOrder
		if err = rows.Scan(&x.SellerID, &x.Revision, &x.Name, &x.Status, &x.DeliveryMethod); err != nil {
			break
		}
		v.Sellers = append(v.Sellers, x)
	}
	err = errors.Join(err, rows.Err())
	rows.Close()
	if err != nil {
		return v, err
	}
	for i := range v.Sellers {
		if err = r.readSource().QueryRow(ctx, `SELECT CASE WHEN EXISTS(SELECT 1 FROM seller_ledger WHERE order_id=$1 AND seller_id=$2 AND status='settled' AND entry_type='sale') THEN 'paid' WHEN EXISTS(SELECT 1 FROM seller_settlements ss JOIN seller_ledger sl ON sl.settlement_id=ss.id WHERE sl.order_id=$1 AND sl.seller_id=$2 AND ss.status='approved' AND ss.transfer_evidence_key IS NOT NULL) THEN 'processing' WHEN marketplace_payout_eligible($1,$2) THEN 'eligible' WHEN EXISTS(SELECT 1 FROM marketplace_cases WHERE order_id=$1 AND status='open' AND (seller_id IS NULL OR seller_id=$2)) OR EXISTS(SELECT 1 FROM marketplace_refunds WHERE order_id=$1 AND seller_id=$2 AND status NOT IN ('processed','rejected')) OR EXISTS(SELECT 1 FROM orders WHERE id=$1 AND payment_review_required) THEN 'on_hold' WHEN EXISTS(SELECT 1 FROM seller_settlements ss JOIN seller_ledger sl ON sl.settlement_id=ss.id WHERE sl.order_id=$1 AND sl.seller_id=$2 AND ss.status='failed') THEN 'failed' ELSE 'pending' END`, v.OrderID, v.Sellers[i].SellerID).Scan(&v.Sellers[i].SettlementStatus); err != nil {
			return v, err
		}
	}
	rows, err = r.readSource().Query(ctx, `SELECT p.id,p.seller_id,p.reference,p.status,p.provider_status,p.awb,p.courier,p.label_url,p.manifest_url,p.pickup_reference,p.pending_action,p.delivered_at,p.delivery_method,p.weight_grams,p.length_cm,p.width_cm,p.height_cm,
		EXISTS(SELECT 1 FROM marketplace_package_items pi JOIN marketplace_cancellations c ON c.order_item_id=pi.order_item_id WHERE pi.package_id=p.id AND c.status='requested'),
		EXISTS(SELECT 1 FROM marketplace_cases c WHERE c.package_id=p.id AND c.kind='pickup_clearance' AND c.status='resolved')
		AND NOT EXISTS(SELECT 1 FROM marketplace_cases c WHERE c.package_id=p.id AND c.kind IN ('pickup_clearance','delivery_dispute') AND c.status='open'),
		EXISTS(SELECT 1 FROM marketplace_cases c WHERE c.package_id=p.id AND c.kind='cancellation_review' AND c.status='open')
		FROM marketplace_packages p WHERE p.order_id=$1 AND ($2=0 OR p.seller_id=$2) ORDER BY p.id`, v.OrderID, seller)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var x ports.MarketplacePackage
		if err = rows.Scan(&x.ID, &x.SellerID, &x.Reference, &x.Status, &x.ProviderStatus, &x.AWB, &x.Courier, &x.LabelURL, &x.ManifestURL, &x.PickupReference, &x.PendingAction, &x.DeliveredAt, &x.DeliveryMethod, &x.WeightGrams, &x.LengthCM, &x.WidthCM, &x.HeightCM, &x.CancellationRequested, &x.PickupCleared, &x.CancellationNeedsReview); err != nil {
			break
		}
		v.Packages = append(v.Packages, x)
	}
	err = errors.Join(err, rows.Err())
	rows.Close()
	if err != nil {
		return v, err
	}
	for i := range v.Packages {
		contents, e := r.readSource().Query(ctx, `SELECT oi.id,oi.product_name,oi.sku,pi.quantity FROM marketplace_package_items pi JOIN order_items oi ON oi.id=pi.order_item_id WHERE pi.package_id=$1 ORDER BY oi.id`, v.Packages[i].ID)
		if e != nil {
			return v, e
		}
		for contents.Next() {
			var item ports.MarketplacePackageItem
			if e = contents.Scan(&item.OrderItemID, &item.Name, &item.SKU, &item.Quantity); e != nil {
				break
			}
			v.Packages[i].Items = append(v.Packages[i].Items, item)
		}
		if e = errorsJoinRows(e, contents); e != nil {
			return v, e
		}
	}
	rows, err = r.readSource().Query(ctx, `SELECT id,seller_id,COALESCE(order_item_id,0),amount_cents,shipping_cents,quantity,kind,status,reason,COALESCE(provider_refund_id,'') FROM marketplace_refunds WHERE order_id=$1 AND ($2=0 OR seller_id=$2) ORDER BY id`, v.OrderID, seller)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var x ports.MarketplaceRefund
		if err = rows.Scan(&x.ID, &x.SellerID, &x.ItemID, &x.AmountCents, &x.ShippingCents, &x.Quantity, &x.Kind, &x.Status, &x.Reason, &x.ProviderRefundID); err != nil {
			break
		}
		v.Refunds = append(v.Refunds, x)
	}
	err = errors.Join(err, rows.Err())
	rows.Close()
	if err != nil {
		return v, err
	}
	rows, err = r.readSource().Query(ctx, `SELECT id,seller_id,number,snapshot,issued_at,pdf_bytes,pdf_sha256 FROM marketplace_invoices WHERE order_id=$1 AND ($2=0 OR seller_id=$2) ORDER BY id`, v.OrderID, seller)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var x ports.MarketplaceInvoice
		if err = rows.Scan(&x.ID, &x.SellerID, &x.Number, &x.Snapshot, &x.IssuedAt, &x.PDF, &x.SHA256); err != nil {
			break
		}
		v.Invoices = append(v.Invoices, x)
	}
	err = errors.Join(err, rows.Err())
	rows.Close()
	if err != nil {
		return v, err
	}
	notes, noteErr := r.readSource().Query(ctx, `SELECT id,seller_id,number,snapshot,issued_at,pdf_bytes,pdf_sha256 FROM marketplace_credit_notes WHERE order_id=$1 AND ($2=0 OR seller_id=$2) ORDER BY id`, v.OrderID, seller)
	if noteErr != nil {
		return v, noteErr
	}
	for notes.Next() {
		var n ports.MarketplaceInvoice
		if noteErr = notes.Scan(&n.ID, &n.SellerID, &n.Number, &n.Snapshot, &n.IssuedAt, &n.PDF, &n.SHA256); noteErr != nil {
			break
		}
		v.CreditNotes = append(v.CreditNotes, n)
	}
	return v, errorsJoinRows(noteErr, notes)
}

func marketplaceAudit(ctx context.Context, tx pgx.Tx, actor, order int64, action, reason string) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,$2,'order',$3,$4,jsonb_build_object('reason',$5::text))`, fmt.Sprint(actor), "marketplace."+action, fmt.Sprint(order), ports.RequestID(ctx), reason)
	return err
}
func marketplaceCase(ctx context.Context, tx pgx.Tx, order, seller, parcel int64, kind, reason string) error {
	_, err := tx.Exec(ctx, `INSERT INTO marketplace_cases(order_id,seller_id,package_id,kind,reason) VALUES($1,NULLIF($2,0),NULLIF($3,0),$4,$5) ON CONFLICT DO NOTHING`, order, seller, parcel, kind, reason)
	return err
}

func (r *MarketplaceRepository) Act(ctx context.Context, actor int64, c ports.MarketplaceCommand) error {
	if len(c.Number) > 120 || len(c.Reason) > 1000 || len(c.Key) > 120 {
		return ports.ErrInvalidState
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		a, err := marketplaceIdentity(ctx, tx, actor)
		if err != nil {
			return err
		}
		if c.Action == "retry_job" {
			if !a.company() || len(strings.TrimSpace(c.Reason)) < 4 {
				return ports.ErrForbidden
			}
			result, e := tx.Exec(ctx, `UPDATE marketplace_jobs j SET status='queued',attempts=0,available_at=now(),last_error=''
				WHERE id=$1 AND status='failed' AND (kind<>'cancel_items' OR EXISTS(
					SELECT 1 FROM marketplace_cancellations c WHERE c.id=j.resource_id AND c.status='requested'
				))`, c.CaseID)
			if e != nil {
				return e
			}
			if result.RowsAffected() != 1 {
				return ports.ErrInvalidState
			}
			return marketplaceAudit(ctx, tx, actor, 0, c.Action, c.Reason)
		}
		if c.Action == "resolve_case" || c.Action == "assign_case" {
			if !a.company() || len(strings.TrimSpace(c.Reason)) < 4 {
				return ports.ErrForbidden
			}
			if len(c.Reason) > 1000 {
				return ports.ErrInvalidState
			}
			result, e := tx.Exec(ctx, `UPDATE marketplace_cases SET assignee_id=$2,status=CASE WHEN $3 THEN 'resolved' ELSE status END,
				resolution=CASE WHEN $3 THEN $4 ELSE resolution END,next_action=CASE WHEN $3 THEN '' ELSE $4 END,
				resolved_at=CASE WHEN $3 THEN now() ELSE resolved_at END WHERE id=$1 AND status='open'
				AND (kind<>'pickup_clearance' OR NOT $3 OR (length(trim($4))>=10 AND EXISTS(
					SELECT 1 FROM marketplace_packages p WHERE p.id=marketplace_cases.package_id AND p.status='cancelled'
					AND (p.awb<>'' OR p.pickup_reference<>'')
				)))`, c.CaseID, actor, c.Action == "resolve_case", c.Reason)
			if e != nil {
				return e
			}
			if result.RowsAffected() != 1 {
				return ports.ErrInvalidState
			}
			return marketplaceAudit(ctx, tx, actor, 0, c.Action, c.Reason)
		}
		var order, owner int64
		var payment, status string
		var review bool
		err = tx.QueryRow(ctx, `SELECT id,user_id,status,payment_review_required FROM orders WHERE order_number=$1 FOR UPDATE`, c.Number).Scan(&order, &owner, &status, &review)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrOrderNotFound
		}
		if err != nil {
			return err
		}
		if !a.company() && !(a.role == "customer" && owner == actor) && !(a.role == "seller" && a.seller == c.SellerID) {
			return ports.ErrForbidden
		}
		if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT status FROM payment_attempts WHERE order_id=$1 ORDER BY id DESC LIMIT 1),'')`, order).Scan(&payment); err != nil {
			return err
		}
		refundDecision := (c.Action == "approve_refund" || c.Action == "reject_refund" || c.Action == "retry_failed_refund") && a.company() && (payment == "captured" || payment == "funded") && !review
		if !refundDecision && !domain.PrepaidOrderConfirmed(status, payment, review) {
			return ports.ErrInvalidState
		}
		// Shared lock order: order/payment first, then finance, then child rows.
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(4856203)`); err != nil {
			return err
		}
		switch c.Action {
		case "submit_claim", "claim_status", "share_claim_evidence", "prepare_replacement":
			err = policyClaimAction(ctx, tx, actor, order, a, c)
		case "inspect_rto":
			if !a.company() || len(c.Reason) < 10 {
				return ports.ErrForbidden
			}
			err = inspectMarketplaceRTO(ctx, tx, order, c)
		case "manual_dispatch", "manual_delivered":
			if c.Action == "manual_dispatch" && a.role != "seller" || c.Action == "manual_delivered" && !a.company() {
				return ports.ErrForbidden
			}
			if len(c.Reason) < 10 || len(c.Carrier) < 2 || len(c.Carrier) > 80 || len(c.Tracking) < 3 || len(c.Tracking) > 120 {
				return ports.ErrShippingInput
			}
			target := "in_transit"
			if c.Action == "manual_delivered" {
				target = "delivered"
			}
			result, e := tx.Exec(ctx, `UPDATE marketplace_packages p SET status=$3,courier=$4,awb=$5,provider_status=$6,delivered_at=CASE WHEN $3='delivered' THEN now() ELSE delivered_at END,revision=revision+1,updated_at=now() WHERE p.id=$1 AND order_id=$2 AND delivery_method='manual' AND ($7 OR seller_id=$8) AND (($3='in_transit' AND status='created' AND EXISTS(SELECT 1 FROM seller_fulfillments sf WHERE sf.order_id=p.order_id AND sf.seller_id=p.seller_id AND sf.lifecycle_status IN ('ready_to_ship','shipped'))) OR ($3='delivered' AND status IN ('in_transit','out_for_delivery'))) AND NOT EXISTS(SELECT 1 FROM marketplace_cancellations mc JOIN marketplace_package_items pi ON pi.order_item_id=mc.order_item_id WHERE pi.package_id=p.id AND mc.status='requested')`, c.PackageID, order, target, c.Carrier, c.Tracking, c.Reason, a.company(), a.seller)
			if e != nil {
				return e
			}
			if result.RowsAffected() != 1 {
				return ports.ErrInvalidState
			}
			var seller int64
			if err = tx.QueryRow(ctx, `SELECT seller_id FROM marketplace_packages WHERE id=$1`, c.PackageID).Scan(&seller); err != nil {
				return err
			}
			err = refreshMarketplaceAggregate(ctx, tx, order, seller)
		case "ndr_reattempt", "ndr_return":
			if !a.company() || len(c.Reason) < 4 || len(c.Key) < 16 {
				return ports.ErrForbidden
			}
			err = queueNDR(ctx, tx, order, c)
		case "request_return", "return_status":
			var newPolicy bool
			if err := tx.QueryRow(ctx, `SELECT COALESCE(policy_snapshot ? 'return_policy_version',false) FROM orders WHERE id=$1`, order).Scan(&newPolicy); err != nil {
				return err
			}
			if newPolicy {
				return ports.ErrInvalidState
			}
			err = marketplaceReturnAction(ctx, tx, actor, order, a, c)
		case "accept", "pack", "ready":
			if a.role != "seller" || a.seller != c.SellerID {
				return ports.ErrForbidden
			}
			var current string
			var revision int64
			if err = tx.QueryRow(ctx, `SELECT lifecycle_status,revision FROM seller_fulfillments WHERE order_id=$1 AND seller_id=$2 FOR UPDATE`, order, a.seller).Scan(&current, &revision); err != nil {
				return err
			}
			target := map[string]string{"accept": "accepted", "pack": "packing", "ready": "ready_to_ship"}[c.Action]
			if revision != c.Revision || !domain.CanTransition("seller", current, target) {
				return ports.ErrInvalidState
			}
			if target == "ready_to_ship" {
				var policy bool
				if err := tx.QueryRow(ctx, `SELECT policy_snapshot ? 'return_policy_version' FROM orders WHERE id=$1`, order).Scan(&policy); err != nil {
					return err
				}
				if policy && len(strings.TrimSpace(c.QCEvidence)) < 10 {
					return ports.ErrInvalidState
				}
				var missing bool
				err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM order_items oi WHERE order_id=$1 AND seller_id=$2 AND quantity-cancelled_quantity>COALESCE((SELECT sum(pi.quantity) FROM marketplace_package_items pi JOIN marketplace_packages p ON p.id=pi.package_id WHERE pi.order_item_id=oi.id AND p.status<>'cancelled'),0))`, order, a.seller).Scan(&missing)
				if err != nil {
					return err
				}
				if missing {
					return ports.ErrInvalidState
				}
			}
			legacy := "packing"
			if target == "accepted" {
				legacy = "processing"
			}
			_, err = tx.Exec(ctx, `UPDATE seller_fulfillments SET lifecycle_status=$3,status=$4,revision=revision+1,accepted_at=CASE WHEN $3='accepted' THEN now() ELSE accepted_at END,updated_by=$5,updated_at=now() WHERE order_id=$1 AND seller_id=$2`, order, a.seller, target, legacy, actor)
			if err == nil {
				_, err = tx.Exec(ctx, `INSERT INTO marketplace_history(order_id,seller_id,system,from_status,to_status,actor_id) VALUES($1,$2,'seller',$3,$4,$5)`, order, a.seller, current, target, actor)
			}
			if err == nil && target == "ready_to_ship" {
				if _, err = tx.Exec(ctx, `UPDATE seller_fulfillments SET qc_evidence=$3 WHERE order_id=$1 AND seller_id=$2`, order, a.seller, c.QCEvidence); err != nil {
					return err
				}
				_, err = tx.Exec(ctx, `SELECT marketplace_enqueue('book_package',id,'book:'||id) FROM marketplace_packages WHERE order_id=$1 AND seller_id=$2 AND status='created' AND delivery_method='shiprocket'`, order, a.seller)
			}
		case "package":
			err = r.createPackage(ctx, tx, actor, order, a, c)
		case "cancel", "reject":
			if c.Action == "reject" && a.role != "seller" {
				return ports.ErrForbidden
			}
			if c.Action == "cancel" && a.role != "customer" && !a.company() {
				return ports.ErrForbidden
			}
			err = requestMarketplaceCancellation(ctx, tx, actor, order, a, c)
		case "request_refund":
			var newPolicy bool
			if err := tx.QueryRow(ctx, `SELECT COALESCE(policy_snapshot ? 'return_policy_version',false) FROM orders WHERE id=$1`, order).Scan(&newPolicy); err != nil {
				return err
			}
			if newPolicy {
				return ports.ErrInvalidState
			}
			if len(strings.TrimSpace(c.Reason)) < 10 || len(c.Key) < 16 {
				return ports.ErrInvalidState
			}
			if a.role == "seller" {
				return ports.ErrForbidden
			}
			err = requestExceptionRefund(ctx, tx, order, c)
		case "approve_refund", "reject_refund", "retry_failed_refund":
			if !a.company() || len(strings.TrimSpace(c.Reason)) < 4 {
				return ports.ErrForbidden
			}
			target := "approved"
			if c.Action == "reject_refund" {
				target = "rejected"
			}
			if target == "approved" {
				if err = allocateRefundFunding(ctx, tx, order, c.RefundID); err != nil {
					return err
				}
			}
			if c.Action == "retry_failed_refund" {
				if _, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,metadata) SELECT $2,'marketplace.failed_refund_retry','marketplace_refund',id::text,jsonb_build_object('previous_provider_refund_id',provider_refund_id,'previous_attempt',submission_attempt,'reason',$3::text) FROM marketplace_refunds WHERE id=$1 AND status='failed'`, c.RefundID, fmt.Sprint(actor), c.Reason); err != nil {
					return err
				}
			}
			result, e := tx.Exec(ctx, `UPDATE marketplace_refunds SET status=$3,approved_by=$4,updated_at=now(),provider_refund_id=CASE WHEN $5 THEN NULL ELSE provider_refund_id END,submission_attempt=submission_attempt+CASE WHEN $5 THEN 1 ELSE 0 END WHERE id=$2 AND order_id=$1 AND (status='requested' OR ($5 AND status='failed'))`, order, c.RefundID, target, actor, c.Action == "retry_failed_refund")
			err = e
			if err == nil && result.RowsAffected() != 1 {
				return ports.ErrInvalidState
			}
			if err == nil && target == "approved" {
				_, err = tx.Exec(ctx, `SELECT marketplace_enqueue('refund',id,'refund:'||id||':a'||submission_attempt) FROM marketplace_refunds WHERE id=$1`, c.RefundID)
			}
		case "refresh_package", "retry_package", "reschedule_pickup":
			if a.role != "seller" && !a.company() {
				return ports.ErrForbidden
			}
			var id int64
			err = tx.QueryRow(ctx, `SELECT id FROM marketplace_packages WHERE id=$1 AND order_id=$2 AND ($3 OR seller_id=$4) AND status NOT IN ('delivered','cancelled') FOR UPDATE`, c.PackageID, order, a.company(), a.seller).Scan(&id)
			if err != nil {
				return err
			}
			if c.Action == "reschedule_pickup" {
				tag, e := tx.Exec(ctx, `UPDATE marketplace_packages p SET pickup_reference='',next_check_at=now()
					WHERE id=$1 AND status IN ('created','pickup_scheduled') AND pending_action=''
					AND NOT EXISTS(SELECT 1 FROM marketplace_package_items pi JOIN marketplace_cancellations c ON c.order_item_id=pi.order_item_id
						WHERE pi.package_id=p.id AND c.status='requested')`, id)
				if e != nil {
					return e
				}
				if tag.RowsAffected() != 1 {
					return ports.ErrInvalidState
				}
			}
			_, err = tx.Exec(ctx, `SELECT marketplace_enqueue('book_package',$1,$2)`, id, fmt.Sprintf("retry:%d:%s", id, c.Key))
		default:
			return ports.ErrInvalidState
		}
		if err != nil {
			return err
		}
		return marketplaceAudit(ctx, tx, actor, order, c.Action, c.Reason)
	})
}

func (r *MarketplaceRepository) createPackage(ctx context.Context, tx pgx.Tx, actor, order int64, a marketplaceActor, c ports.MarketplaceCommand) error {
	if a.role != "seller" || a.seller != c.SellerID || !c.Parcel.Valid() || c.WeightGrams < 100 || len(c.Items) == 0 || len(c.Items) > 100 || len(c.Key) < 16 {
		return ports.ErrShippingInput
	}
	var stage string
	if err := tx.QueryRow(ctx, `SELECT lifecycle_status FROM seller_fulfillments WHERE order_id=$1 AND seller_id=$2 FOR UPDATE`, order, a.seller).Scan(&stage); err != nil {
		return err
	}
	if stage != "packing" {
		return ports.ErrInvalidState
	}
	// Configured verified pickup locations are read by the application service.
	var pickup string
	var method string
	if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT decision FROM order_delivery_decisions WHERE order_id=$1 AND seller_id=$2),'manual')`, order, a.seller).Scan(&method); err != nil {
		return err
	}
	if method == "shiprocket" {
		if err := tx.QueryRow(ctx, `SELECT pickup_location FROM marketplace_shipping_profiles WHERE seller_id=$1 AND verified`, a.seller).Scan(&pickup); err != nil {
			return ports.ErrShippingInput
		}
	} else {
		method = "manual"
	}
	var id int64
	payload, _ := json.Marshal(struct {
		Items  map[int64]int
		Parcel ports.Parcel
	}{c.Items, c.Parcel})
	hash := fmt.Sprintf("%x", sha256.Sum256(payload))
	err := tx.QueryRow(ctx, `INSERT INTO marketplace_packages(order_id,seller_id,reference,pickup_location,weight_grams,length_cm,width_cm,height_cm,requested_by,request_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(reference) DO NOTHING RETURNING id`, order, a.seller, "WC-"+c.Key, pickup, c.WeightGrams, c.LengthCM, c.WidthCM, c.HeightCM, actor, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		var same bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_packages WHERE reference=$1 AND order_id=$2 AND seller_id=$3 AND request_hash=$4)`, "WC-"+c.Key, order, a.seller, hash).Scan(&same)
		if err != nil {
			return err
		}
		if !same {
			return ports.ErrIdempotencyConflict
		}
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE marketplace_packages SET delivery_method=$2 WHERE id=$1`, id, method); err != nil {
		return err
	}
	for item, qty := range c.Items {
		if qty <= 0 {
			return ports.ErrShippingInput
		}
		var remaining int
		err = tx.QueryRow(ctx, `SELECT quantity-cancelled_quantity-COALESCE((SELECT sum(pi.quantity) FROM marketplace_package_items pi JOIN marketplace_packages p ON p.id=pi.package_id WHERE pi.order_item_id=oi.id AND p.status<>'cancelled'),0) FROM order_items oi WHERE id=$1 AND order_id=$2 AND seller_id=$3 FOR UPDATE`, item, order, a.seller).Scan(&remaining)
		if err != nil {
			return err
		}
		if qty > remaining {
			return ports.ErrShippingInput
		}
		if _, err = tx.Exec(ctx, `INSERT INTO marketplace_package_items(package_id,order_item_id,quantity) VALUES($1,$2,$3)`, id, item, qty); err != nil {
			return err
		}
	}
	return nil
}

func requestMarketplaceCancellation(ctx context.Context, tx pgx.Tx, actor, order int64, a marketplaceActor, c ports.MarketplaceCommand) error {
	if c.Quantity <= 0 || len(c.Key) < 16 || len(strings.TrimSpace(c.Reason)) < 4 {
		return ports.ErrInvalidState
	}
	var qty, cancelled, reserved int
	var seller int64
	err := tx.QueryRow(ctx, `SELECT quantity,cancelled_quantity,COALESCE(seller_id,0),COALESCE((SELECT sum(quantity) FROM marketplace_cancellations WHERE order_item_id=oi.id AND status='requested'),0) FROM order_items oi WHERE id=$1 AND order_id=$2 FOR UPDATE`, c.ItemID, order).Scan(&qty, &cancelled, &seller, &reserved)
	if err != nil {
		return err
	}
	if a.role == "seller" && a.seller != seller {
		return ports.ErrForbidden
	}
	var existing, match bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_cancellations WHERE request_key=$1),EXISTS(SELECT 1 FROM marketplace_cancellations WHERE request_key=$1 AND order_item_id=$2 AND quantity=$3 AND requested_by=$4 AND reason=$5)`, c.Key, c.ItemID, c.Quantity, actor, c.Reason).Scan(&existing, &match)
	if err != nil {
		return err
	}
	if existing {
		if match {
			return nil
		}
		return ports.ErrIdempotencyConflict
	}
	if a.role == "customer" {
		var allowed bool
		if err := tx.QueryRow(ctx, `SELECT (NOT(policy_snapshot ? 'return_policy_version') OR now()<=created_at+interval '12 hours') AND NOT EXISTS(SELECT 1 FROM seller_fulfillments WHERE order_id=$1 AND seller_id=$2 AND handover_at IS NOT NULL) FROM orders WHERE id=$1`, order, seller).Scan(&allowed); err != nil {
			return err
		}
		if !allowed {
			return ports.ErrInvalidState
		}
	}
	var refundUnits int
	if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0) FROM marketplace_refunds WHERE order_item_id=$1 AND status<>'rejected'`, c.ItemID).Scan(&refundUnits); err != nil {
		return err
	}
	cancelled = max(cancelled, refundUnits)
	if c.Quantity > qty-cancelled-reserved {
		return ports.ErrInvalidState
	}
	var dispatched bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_packages p JOIN marketplace_package_items pi ON pi.package_id=p.id WHERE pi.order_item_id=$1 AND (p.status NOT IN ('created','pickup_scheduled','cancelled') OR p.pending_action<>'')) OR EXISTS(SELECT 1 FROM seller_fulfillments WHERE order_id=$2 AND seller_id=$3 AND status IN ('shipped','out_for_delivery','delivered')) OR EXISTS(SELECT 1 FROM shiprocket_shipments WHERE order_id=$2 AND seller_id=$3 AND provider_status NOT IN ('CANCELLED','CANCELED'))`, c.ItemID, order, seller).Scan(&dispatched)
	if err != nil {
		return err
	}
	if dispatched {
		return ports.ErrInvalidState
	}
	var id int64
	if err = tx.QueryRow(ctx, `INSERT INTO marketplace_cancellations(order_id,order_item_id,quantity,reason,request_key,requested_by) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, order, c.ItemID, c.Quantity, c.Reason, c.Key, actor).Scan(&id); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `SELECT marketplace_enqueue('cancel_items',$1,$2)`, id, "cancel:"+c.Key)
	return err
}

func requestExceptionRefund(ctx context.Context, tx pgx.Tx, order int64, c ports.MarketplaceCommand) error {
	var paid, seller int64
	var quantity, refunded, pending int
	err := tx.QueryRow(ctx, `SELECT line_total_cents-COALESCE(allocated_discount_cents,0),seller_id,quantity,financially_reversed_quantity,COALESCE((SELECT sum(quantity) FROM marketplace_refunds WHERE order_item_id=oi.id AND status NOT IN ('processed','rejected')),0) FROM order_items oi WHERE id=$1 AND order_id=$2 FOR UPDATE`, c.ItemID, order).Scan(&paid, &seller, &quantity, &refunded, &pending)
	if err != nil {
		return err
	}
	var exists, match bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_refunds WHERE request_key=$1),EXISTS(SELECT 1 FROM marketplace_refunds WHERE request_key=$1 AND order_id=$2 AND order_item_id=$3 AND quantity=$4 AND reason=$5)`, c.Key, order, c.ItemID, c.Quantity, c.Reason).Scan(&exists, &match); err != nil {
		return err
	}
	if exists {
		if match {
			return nil
		}
		return ports.ErrIdempotencyConflict
	}
	amount, err := domain.AllocateRefund(paid, quantity, refunded+pending, c.Quantity)
	if err != nil || amount <= 0 {
		return ports.ErrRefundState
	}
	_, err = tx.Exec(ctx, `INSERT INTO marketplace_refunds(order_id,seller_id,order_item_id,quantity,amount_cents,reason,kind,request_key) VALUES($1,$2,$3,$4,$5,$6,'exception',$7)`, order, seller, c.ItemID, c.Quantity, amount, c.Reason, c.Key)
	if err != nil {
		return err
	}
	return marketplaceCase(ctx, tx, order, seller, 0, "refund_request", c.Reason)
}
