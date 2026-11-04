package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

func requirePaymentReviewer(ctx context.Context, tx pgx.Tx, actor int64) error {
	var id int64
	err := tx.QueryRow(ctx, `SELECT u.id FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.id=$1 AND u.status='active' AND ur.role_slug IN ('admin','super_admin') FOR SHARE OF u,ur`, actor).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.ErrForbidden
	}
	return err
}
func (r *PaymentRepository) ListPaymentReviews(ctx context.Context, actor int64, limit int) ([]ports.PaymentReviewCase, error) {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	cases := []ports.PaymentReviewCase{}
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := requirePaymentReviewer(ctx, tx, actor); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT c.id,c.order_id,o.order_number,o.status,c.provider_payment_id,c.amount_cents,c.currency,c.reason,c.status,c.resolution_reason,c.refund_submission_state,COALESCE(c.provider_refund_id,''),
  (c.status='open' AND o.status IN ('pending_payment','payment_failed') AND pa.status='captured' AND pa.provider_payment_id=c.provider_payment_id AND EXISTS(SELECT 1 FROM inventory_reservations ir WHERE ir.order_id=o.id AND ir.status IN ('released','expired')))
  FROM payment_review_cases c JOIN orders o ON o.id=c.order_id JOIN payment_attempts pa ON pa.id=c.payment_attempt_id
  ORDER BY (c.status IN ('open','refund_pending')) DESC,c.created_at,c.id LIMIT $1`, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c ports.PaymentReviewCase
			if err := rows.Scan(&c.ID, &c.OrderID, &c.OrderNumber, &c.OrderStatus, &c.ProviderPaymentID, &c.AmountCents, &c.Currency, &c.Reason, &c.Status, &c.ResolutionReason, &c.RefundSubmissionState, &c.ProviderRefundID, &c.CanApprove); err != nil {
				return err
			}
			cases = append(cases, c)
		}
		return rows.Err()
	})
	return cases, err
}

type lockedPaymentReview struct {
	ports.PaymentReviewCase
	attemptID                           int64
	refundAttempt                       int
	aggregatePaymentID, aggregateStatus string
}

func lockPaymentReview(ctx context.Context, tx pgx.Tx, id int64) (lockedPaymentReview, error) {
	var c lockedPaymentReview
	var orderID int64
	if err := tx.QueryRow(ctx, `SELECT order_id FROM payment_review_cases WHERE id=$1`, id).Scan(&orderID); err != nil {
		return c, err
	}
	// Match capture/expiry's order-first lock order.
	if err := tx.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&c.OrderStatus); err != nil {
		return c, err
	}
	err := tx.QueryRow(ctx, `SELECT c.id,c.order_id,o.order_number,c.payment_attempt_id,c.provider_payment_id,c.amount_cents,c.currency,c.reason,c.status,c.resolution_reason,c.refund_submission_state,COALESCE(c.provider_refund_id,''),COALESCE(pa.provider_payment_id,''),pa.status,c.refund_attempt
 FROM payment_review_cases c JOIN orders o ON o.id=c.order_id JOIN payment_attempts pa ON pa.id=c.payment_attempt_id WHERE c.id=$1 FOR UPDATE OF c,pa`, id).Scan(&c.ID, &c.OrderID, &c.OrderNumber, &c.attemptID, &c.ProviderPaymentID, &c.AmountCents, &c.Currency, &c.Reason, &c.Status, &c.ResolutionReason, &c.RefundSubmissionState, &c.ProviderRefundID, &c.aggregatePaymentID, &c.aggregateStatus, &c.refundAttempt)
	return c, err
}
func auditPaymentReview(ctx context.Context, tx pgx.Tx, actor, id int64, action, reason string) error {
	var actorID any
	if actor > 0 {
		actorID = fmt.Sprint(actor)
	}
	_, err := tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,$2,'payment_review',$3,$4,jsonb_build_object('reason',$5::text))`, actorID, action, fmt.Sprint(id), ports.RequestID(ctx), reason)
	return err
}
func clearPaymentReviewHold(ctx context.Context, tx pgx.Tx, orderID int64) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(4856203)`); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE orders SET payment_review_required=EXISTS(SELECT 1 FROM payment_review_cases WHERE order_id=$1 AND status IN ('open','refund_pending')),updated_at=NOW() WHERE id=$1`, orderID)
	return err
}

func (r *PaymentRepository) ApprovePaymentReview(ctx context.Context, actor, id int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if len(reason) < 3 || len(reason) > 500 {
		return ports.ErrInvalidState
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := requirePaymentReviewer(ctx, tx, actor); err != nil {
			return err
		}
		c, err := lockPaymentReview(ctx, tx, id)
		if err != nil {
			return err
		}
		if c.Status == "approved" {
			return nil
		}
		if c.Status != "open" || (c.OrderStatus != "payment_failed" && c.OrderStatus != "pending_payment") || c.aggregateStatus != "captured" || c.aggregatePaymentID != c.ProviderPaymentID {
			return ports.ErrPaymentState
		}
		var reservationID int64
		var reservationState string
		if err := tx.QueryRow(ctx, `SELECT id,status FROM inventory_reservations WHERE order_id=$1 FOR UPDATE`, c.OrderID).Scan(&reservationID, &reservationState); err != nil {
			return err
		}
		if reservationState == "committed" || reservationState == "active" {
			return ports.ErrPaymentState
		}
		var expected int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM inventory_reservation_items WHERE reservation_id=$1`, reservationID).Scan(&expected); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT ri.variant_id,ri.quantity,i.available_quantity FROM inventory_reservation_items ri JOIN inventory_stock i ON i.variant_id=ri.variant_id
  JOIN product_variants v ON v.id=ri.variant_id JOIN products p ON p.id=v.product_id JOIN sellers s ON s.id=p.seller_id
  WHERE ri.reservation_id=$1 AND v.status='active' AND p.status='approved' AND s.status='active' ORDER BY ri.variant_id FOR UPDATE OF i FOR SHARE OF v,p,s`, reservationID)
		if err != nil {
			return err
		}
		type item struct {
			id                  int64
			quantity, available int
		}
		items := []item{}
		for rows.Next() {
			var i item
			if err := rows.Scan(&i.id, &i.quantity, &i.available); err != nil {
				rows.Close()
				return err
			}
			items = append(items, i)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if expected == 0 || len(items) != expected {
			return ports.ErrInsufficientStock
		}
		for _, i := range items {
			if i.available < i.quantity {
				return ports.ErrInsufficientStock
			}
		}
		for _, i := range items {
			result, err := tx.Exec(ctx, `UPDATE inventory_stock SET available_quantity=available_quantity-$2,reserved_quantity=reserved_quantity+$2,updated_at=NOW() WHERE variant_id=$1 AND available_quantity>=$2`, i.id, i.quantity)
			if err != nil {
				return err
			}
			if result.RowsAffected() != 1 {
				return ports.ErrInsufficientStock
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE inventory_reservations SET status='active',expires_at=NOW()+INTERVAL '15 minutes',updated_at=NOW() WHERE id=$1`, reservationID); err != nil {
			return err
		}
		if err = commitCapturedOrder(ctx, tx, c.OrderID, reservationID, c.OrderStatus); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE payment_review_cases SET status='approved',resolution_reason=$2,resolved_by=$3,updated_at=NOW() WHERE id=$1`, id, reason, actor); err != nil {
			return err
		}
		if err = clearPaymentReviewHold(ctx, tx, c.OrderID); err != nil {
			return err
		}
		return auditPaymentReview(ctx, tx, actor, id, "payment.review_approved", reason)
	})
}

func reviewRefundRecord(c lockedPaymentReview) ports.PaymentRefundRecord {
	status := "pending"
	if c.Status == "refunded" {
		status = "processed"
	}
	return ports.PaymentRefundRecord{ID: c.ID, OrderID: c.OrderID, OrderNumber: c.OrderNumber, OrderStatus: c.OrderStatus, PaymentStatus: c.aggregateStatus, ProviderPaymentID: c.ProviderPaymentID, ProviderRefundID: c.ProviderRefundID, Status: status, SubmissionState: c.RefundSubmissionState, AmountCents: c.AmountCents, Currency: c.Currency, Reason: c.ResolutionReason, Receipt: paymentReviewRefundReceipt(c.ID, c.refundAttempt)}
}
func paymentReviewRefundReceipt(id int64, attempt int) string {
	if attempt <= 1 {
		return fmt.Sprintf("payment-review-%d", id)
	}
	return fmt.Sprintf("pr-%d-%d", id, attempt)
}
func (r *PaymentRepository) BeginReviewRefund(ctx context.Context, actor, id int64, reason string) (ports.PaymentRefundRecord, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) < 3 || len(reason) > 500 {
		return ports.PaymentRefundRecord{}, ports.ErrInvalidState
	}
	var record ports.PaymentRefundRecord
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := requirePaymentReviewer(ctx, tx, actor); err != nil {
			return err
		}
		c, err := lockPaymentReview(ctx, tx, id)
		if err != nil {
			return err
		}
		if c.Status == "approved" {
			return ports.ErrRefundState
		}
		if c.Status == "open" {
			if _, err = tx.Exec(ctx, `UPDATE payment_review_cases SET status='refund_pending',resolution_reason=$2,resolved_by=$3,updated_at=NOW() WHERE id=$1`, id, reason, actor); err != nil {
				return err
			}
			c.Status = "refund_pending"
			c.ResolutionReason = reason
			if err = auditPaymentReview(ctx, tx, actor, id, "payment.review_refund_authorized", reason); err != nil {
				return err
			}
		}
		record = reviewRefundRecord(c)
		return nil
	})
	return record, err
}

func (r *PaymentRepository) AuthorizeReviewRefundRetry(ctx context.Context, actor, id int64, reason, expectedRefundID string) error {
	reason = strings.TrimSpace(reason)
	if len(reason) < 3 || len(reason) > 500 || expectedRefundID == "" || len(expectedRefundID) > 128 {
		return ports.ErrRefundState
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := requirePaymentReviewer(ctx, tx, actor); err != nil {
			return err
		}
		c, err := lockPaymentReview(ctx, tx, id)
		if err != nil {
			return err
		}
		// An old form/replayed request cannot authorize another provider POST.
		if c.Status != "refund_pending" || c.RefundSubmissionState != "failed" || c.ProviderRefundID != expectedRefundID {
			return ports.ErrRefundState
		}
		if _, err = tx.Exec(ctx, `UPDATE payment_review_cases SET refund_submission_state='queued',provider_refund_id=NULL,refund_attempt=refund_attempt+1,resolution_reason=$2,resolved_by=$3,reconcile_attempts=0,next_reconcile_at=NOW(),updated_at=NOW() WHERE id=$1`, id, reason, actor); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'payment.review_refund_reauthorized','payment_review',$2,$3,jsonb_build_object('reason',$4::text,'previous_failed_refund_id',$5::text,'refund_attempt',$6::int))`, fmt.Sprint(actor), fmt.Sprint(id), ports.RequestID(ctx), reason, expectedRefundID, c.refundAttempt+1)
		return err
	})
}
func (r *PaymentRepository) ClaimReviewRefundSubmission(ctx context.Context, id int64) (bool, error) {
	result, err := r.pool.Exec(ctx, `UPDATE payment_review_cases SET refund_submission_state='submitting',next_reconcile_at=NOW()+INTERVAL '1 minute',updated_at=NOW() WHERE id=$1 AND status='refund_pending' AND refund_submission_state='queued' AND provider_refund_id IS NULL AND resolved_by IS NOT NULL`, id)
	return result.RowsAffected() == 1, err
}
func (r *PaymentRepository) MarkReviewRefundUncertain(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE payment_review_cases SET refund_submission_state='uncertain',updated_at=NOW() WHERE id=$1 AND status='refund_pending' AND refund_submission_state='submitting'`, id)
	return err
}
func (r *PaymentRepository) CompleteReviewRefund(ctx context.Context, id int64, refund ports.PaymentRefund) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		c, err := lockPaymentReview(ctx, tx, id)
		if err != nil {
			return err
		}
		if refund.ProviderPaymentID != c.ProviderPaymentID || refund.AmountCents != c.AmountCents || refund.Currency != c.Currency || refund.ProviderRefundID == "" || (c.ProviderRefundID != "" && c.ProviderRefundID != refund.ProviderRefundID) {
			return ports.ErrPaymentMismatch
		}
		if c.Status == "refunded" {
			return nil
		}
		if c.Status != "refund_pending" {
			return ports.ErrRefundState
		}
		if refund.Status != "pending" && refund.Status != "processed" && refund.Status != "failed" {
			return ports.ErrRefundState
		}
		status := "refund_pending"
		if refund.Status == "processed" {
			status = "refunded"
		}
		result, err := tx.Exec(ctx, `UPDATE payment_review_cases SET status=$2,provider_refund_id=$3,refund_submission_state=$4,next_reconcile_at=NOW()+INTERVAL '1 minute',updated_at=NOW() WHERE id=$1 AND resolved_by IS NOT NULL`, id, status, refund.ProviderRefundID, refund.Status)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ports.ErrForbidden
		}
		if status == "refunded" {
			if _, err = tx.Exec(ctx, `UPDATE provider_payment_attempts SET status='refunded',updated_at=NOW() WHERE payment_attempt_id=$1 AND provider_payment_id=$2`, c.attemptID, c.ProviderPaymentID); err != nil {
				return err
			}
			if c.aggregatePaymentID == c.ProviderPaymentID {
				// Reverse only an existing sale for this same payment (including a
				// historical inconsistent capture). Extra-charge refunds never
				// reverse the original valid sale's earnings.
				if err = recordSellerRefundLedger(ctx, tx, c.OrderID); err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `UPDATE payment_attempts SET status='refunded',updated_at=NOW() WHERE id=$1 AND provider_payment_id=$2`, c.attemptID, c.ProviderPaymentID); err != nil {
					return err
				}
				result, err = tx.Exec(ctx, `UPDATE orders SET status='refunded',updated_at=NOW() WHERE id=$1 AND status IN ('pending_payment','payment_failed','cancelled')`, c.OrderID)
				if err != nil {
					return err
				}
				if result.RowsAffected() == 1 {
					if _, err = tx.Exec(ctx, `INSERT INTO order_status_history(order_id,from_status,to_status,reason) VALUES($1,$2,'refunded','authorized payment review refund confirmed')`, c.OrderID, c.OrderStatus); err != nil {
						return err
					}
				}
			}
			if err = clearPaymentReviewHold(ctx, tx, c.OrderID); err != nil {
				return err
			}
		}
		return auditPaymentReview(ctx, tx, 0, id, "payment.review_refund_"+refund.Status, refund.ProviderRefundID)
	})
}
func (r *PaymentRepository) ClaimReviewRefundReconciliation(ctx context.Context, limit int) ([]ports.PaymentRefundRecord, error) {
	rows, err := r.pool.Query(ctx, `WITH due AS(SELECT id FROM payment_review_cases WHERE status='refund_pending' AND refund_submission_state IN ('submitting','uncertain','pending') AND next_reconcile_at<=NOW() ORDER BY next_reconcile_at,id LIMIT $1 FOR UPDATE SKIP LOCKED), claimed AS(
 UPDATE payment_review_cases c SET next_reconcile_at=NOW()+LEAST(60,power(2,LEAST(reconcile_attempts,6)))*INTERVAL '1 minute',reconcile_attempts=reconcile_attempts+1 FROM due WHERE c.id=due.id RETURNING c.*)
 SELECT c.id,c.order_id,o.order_number,o.status,c.provider_payment_id,c.amount_cents,c.currency,c.resolution_reason,c.refund_submission_state,COALESCE(c.provider_refund_id,''),c.refund_attempt FROM claimed c JOIN orders o ON o.id=c.order_id`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []ports.PaymentRefundRecord{}
	for rows.Next() {
		var record ports.PaymentRefundRecord
		var attempt int
		if err := rows.Scan(&record.ID, &record.OrderID, &record.OrderNumber, &record.OrderStatus, &record.ProviderPaymentID, &record.AmountCents, &record.Currency, &record.Reason, &record.SubmissionState, &record.ProviderRefundID, &attempt); err != nil {
			return nil, err
		}
		record.Receipt = paymentReviewRefundReceipt(record.ID, attempt)
		records = append(records, record)
	}
	return records, rows.Err()
}
