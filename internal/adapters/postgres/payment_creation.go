package postgres

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/wecratfs/commerce/internal/ports"
)

const paymentCreationLookupDelay = "20 seconds"

func (r *PaymentRepository) ClaimPaymentCreation(ctx context.Context, userID int64, number string) (ports.PaymentCreationClaim, error) {
	return r.claimPaymentCreation(ctx, userID, number, true)
}

func (r *PaymentRepository) ClaimExistingPaymentCreation(ctx context.Context, userID int64, number string) (ports.PaymentCreationClaim, error) {
	return r.claimPaymentCreation(ctx, userID, number, false)
}

func (r *PaymentRepository) claimPaymentCreation(ctx context.Context, userID int64, number string, allowSubmit bool) (ports.PaymentCreationClaim, error) {
	claim := ports.PaymentCreationClaim{UserID: userID}
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		order := &claim.Order
		if err := tx.QueryRow(ctx, `SELECT id,order_number,status FROM orders
			WHERE user_id=$1 AND order_number=$2 FOR UPDATE`, userID, number).
			Scan(&order.OrderID, &order.OrderNumber, &order.OrderStatus); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ports.ErrPaymentNotFound
			}
			return err
		}
		var started, lookupDue, review, payable bool
		if err := tx.QueryRow(ctx, `SELECT pa.id,pa.provider,COALESCE(pa.provider_order_id,''),pa.status,pa.amount_cents,pa.currency,
			pa.creation_started_at IS NOT NULL,COALESCE(pa.creation_next_lookup_at<=NOW(),false),pa.creation_review_reason<>'',`+paymentOrderPayableSQL+`
			FROM orders o JOIN payment_attempts pa ON pa.order_id=o.id WHERE o.id=$1
			ORDER BY pa.id DESC LIMIT 1 FOR UPDATE OF pa`, order.OrderID).
			Scan(&claim.AttemptID, &order.Provider, &order.ProviderOrderID, &order.PaymentStatus, &order.AmountCents, &order.Currency,
				&started, &lookupDue, &review, &payable); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ports.ErrPaymentNotFound
			}
			return err
		}
		if review {
			return ports.ErrPaymentCreationReview
		}
		if !allowSubmit && (!started || order.ProviderOrderID != "") {
			claim.AttemptID = 0
			return nil
		}
		if (allowSubmit && !payable) || order.Provider != "razorpay" || order.AmountCents < 100 || order.Currency == "" {
			return ports.ErrPaymentState
		}
		if order.ProviderOrderID != "" {
			return nil
		}
		if order.PaymentStatus != "created" && (allowSubmit || order.PaymentStatus != "failed") {
			return ports.ErrPaymentState
		}
		if started && !lookupDue {
			return ports.ErrPaymentCreationPending
		}
		_, err := tx.Exec(ctx, `UPDATE payment_attempts SET creation_started_at=COALESCE(creation_started_at,NOW()),
			creation_next_lookup_at=NOW()+$2::interval WHERE id=$1`, claim.AttemptID, paymentCreationLookupDelay)
		claim.Submit = !started
		return err
	})
	return claim, err
}

func (r *PaymentRepository) ClaimPaymentCreationRecovery(ctx context.Context, limit int) ([]ports.PaymentCreationClaim, error) {
	rows, err := r.pool.Query(ctx, `WITH due AS (
		SELECT id FROM payment_attempts WHERE provider='razorpay' AND provider_order_id IS NULL
		  AND creation_started_at IS NOT NULL AND creation_review_reason='' AND creation_next_lookup_at<=NOW()
		ORDER BY creation_next_lookup_at,id LIMIT $1 FOR UPDATE SKIP LOCKED
	), claimed AS (
		UPDATE payment_attempts pa SET creation_next_lookup_at=NOW()+CASE
		  WHEN creation_started_at<NOW()-INTERVAL '1 hour' THEN INTERVAL '1 hour' ELSE INTERVAL '1 minute' END
		FROM due WHERE pa.id=due.id RETURNING pa.*
	) SELECT pa.id,o.user_id,o.id,o.order_number,o.status,pa.provider,pa.status,pa.amount_cents,pa.currency
	FROM claimed pa JOIN orders o ON o.id=pa.order_id`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var claims []ports.PaymentCreationClaim
	for rows.Next() {
		var claim ports.PaymentCreationClaim
		order := &claim.Order
		if err := rows.Scan(&claim.AttemptID, &claim.UserID, &order.OrderID, &order.OrderNumber, &order.OrderStatus,
			&order.Provider, &order.PaymentStatus, &order.AmountCents, &order.Currency); err != nil {
			return nil, err
		}
		claims = append(claims, claim)
	}
	return claims, rows.Err()
}

func (r *PaymentRepository) AttachClaimedProviderOrder(ctx context.Context, claim ports.PaymentCreationClaim, order ports.PaymentOrder) (ports.PaymentOrderRecord, error) {
	return r.attachProviderOrder(ctx, claim.UserID, claim.Order.OrderNumber, claim.AttemptID, order)
}

func (r *PaymentRepository) ReviewPaymentCreation(ctx context.Context, claim ports.PaymentCreationClaim, reason string) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `UPDATE payment_attempts SET creation_review_reason=$2,creation_next_lookup_at=NULL
			WHERE id=$1 AND creation_started_at IS NOT NULL AND provider_order_id IS NULL AND creation_review_reason=''`, claim.AttemptID, reason)
		if err != nil || result.RowsAffected() == 0 {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO notifications(user_id,kind,title,body)
			SELECT DISTINCT u.id,'payment_review','Payment order needs review',
			'Check Finance payment reviews for order '||$1::text||'. No capture has been verified.'
			FROM users u JOIN user_roles ur ON ur.user_id=u.id
			WHERE u.status='active' AND ur.role_slug IN ('admin','super_admin')`, claim.Order.OrderNumber); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_logs(action,resource_type,resource_id,request_id,metadata)
			VALUES('payment.creation_review_opened','order',$1,$2,jsonb_build_object('reason',$3::text))`,
			strconv.FormatInt(claim.Order.OrderID, 10), ports.RequestID(ctx), reason)
		return err
	})
}

func (r *PaymentRepository) PaymentReconciliationBacklog(ctx context.Context) (due, oldestSeconds, creationReview int64, err error) {
	err = r.pool.QueryRow(ctx, `WITH pending(created_at,due_at) AS (
		SELECT creation_started_at,creation_next_lookup_at FROM payment_attempts
		WHERE provider='razorpay' AND provider_order_id IS NULL AND creation_started_at IS NOT NULL AND creation_review_reason=''
		UNION ALL
		SELECT created_at,next_reconcile_at FROM payment_attempts
		WHERE provider='razorpay' AND provider_order_id IS NOT NULL AND status IN ('pending','authorized','failed')
	) SELECT COUNT(*) FILTER (WHERE due_at<=NOW()),
		COALESCE(GREATEST(EXTRACT(EPOCH FROM NOW()-MIN(created_at)),0),0)::bigint,
		(SELECT COUNT(*) FROM payment_attempts WHERE provider='razorpay' AND creation_review_reason<>'')
	FROM pending`).Scan(&due, &oldestSeconds, &creationReview)
	return
}
