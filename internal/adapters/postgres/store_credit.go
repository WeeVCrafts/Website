package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

func creditAvailable(ctx context.Context, q catalogProductQuerier, user int64) (int64, error) {
	var amount int64
	err := q.QueryRow(ctx, `SELECT COALESCE(sum(remaining_cents),0) FROM store_credit_grants WHERE user_id=$1 AND expires_at>now()`, user).Scan(&amount)
	return amount, err
}

// The user lock serializes reservations across carts. Ledger entries and stock
// reservations are committed in the same order transaction.
func reserveOrderCredit(ctx context.Context, tx pgx.Tx, user, order, total int64, enabled bool) (int64, error) {
	if !enabled {
		return 0, nil
	}
	if _, err := tx.Exec(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, user); err != nil {
		return 0, err
	}
	available, err := creditAvailable(ctx, tx, user)
	if err != nil {
		return 0, err
	}
	amount := min(available, total)
	// Razorpay requires at least one rupee for a nonzero cash remainder.
	if total-amount > 0 && total-amount < 100 {
		amount = max(0, total-100)
	}
	left := amount
	rows, err := tx.Query(ctx, `SELECT id,remaining_cents FROM store_credit_grants WHERE user_id=$1 AND expires_at>now() AND remaining_cents>0 ORDER BY expires_at,id FOR UPDATE`, user)
	if err != nil {
		return 0, err
	}
	type allocation struct{ id, amount int64 }
	var entries []allocation
	for rows.Next() {
		var id, n int64
		if err = rows.Scan(&id, &n); err != nil {
			break
		}
		take := min(n, left)
		if take > 0 {
			entries = append(entries, allocation{id, take})
			left -= take
		}
	}
	if err = errorsJoinRows(err, rows); err != nil {
		return 0, err
	}
	for _, a := range entries {
		if _, err = tx.Exec(ctx, `UPDATE store_credit_grants SET remaining_cents=remaining_cents-$2 WHERE id=$1`, a.id, a.amount); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO store_credit_allocations(order_id,grant_id,amount_cents) VALUES($1,$2,$3)`, order, a.id, a.amount); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO store_credit_history(user_id,grant_id,order_id,amount_cents,kind,source_key) VALUES($1,$2,$3,$4,'reserved',$5)`, user, a.id, order, -a.amount, fmt.Sprintf("reserve:%d:%d", order, a.id)); err != nil {
			return 0, err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE orders SET credit_cents=$2 WHERE id=$1`, order, amount)
	return amount, err
}

func commitOrderCredit(ctx context.Context, tx pgx.Tx, order int64) error {
	var credit, reserved int64
	if err := tx.QueryRow(ctx, `SELECT credit_cents,COALESCE((SELECT sum(amount_cents) FROM store_credit_allocations WHERE order_id=o.id AND status IN ('reserved','committed')),0) FROM orders o WHERE id=$1`, order).Scan(&credit, &reserved); err != nil {
		return err
	}
	if credit != reserved {
		return ports.ErrPaymentState
	}
	_, err := tx.Exec(ctx, `UPDATE store_credit_allocations SET status='committed' WHERE order_id=$1 AND status='reserved'`, order)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE orders SET funding_status='funded' WHERE id=$1`, order)
	return err
}

// Restore only committed credit into a new grant tied to its source. A restored
// balance cannot revive unrelated expired credit.
func restoreOrderCredit(ctx context.Context, tx pgx.Tx, order, amount int64, key string) error {
	if amount == 0 {
		return nil
	}
	var user int64
	if err := tx.QueryRow(ctx, `SELECT user_id FROM orders WHERE id=$1`, order).Scan(&user); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, user); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM store_credit_history WHERE source_key LIKE $1 AND order_id=$2)`, key+":%", order).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT a.grant_id,a.amount_cents-a.restored_cents FROM store_credit_allocations a JOIN store_credit_grants g ON g.id=a.grant_id WHERE a.order_id=$1 AND a.status='committed' ORDER BY g.expires_at,g.id FOR UPDATE OF a,g`, order)
	if err != nil {
		return err
	}
	type allocation struct{ id, n int64 }
	var entries []allocation
	left := amount
	for rows.Next() {
		var a allocation
		if err = rows.Scan(&a.id, &a.n); err != nil {
			break
		}
		a.n = min(left, a.n)
		if a.n > 0 {
			entries = append(entries, a)
			left -= a.n
		}
	}
	if err = errorsJoinRows(err, rows); err != nil {
		return err
	}
	if left != 0 {
		return ports.ErrRefundState
	}
	for _, a := range entries {
		var restoredGrant int64
		restoreKey := fmt.Sprintf("%s:%d", key, a.id)
		if err = tx.QueryRow(ctx, `INSERT INTO store_credit_grants(user_id,amount_cents,remaining_cents,expires_at,source_grant_id,restoration_key) SELECT user_id,$2,$2,GREATEST(expires_at,now()+interval '30 days'),id,$3 FROM store_credit_grants WHERE id=$1 RETURNING id`, a.id, a.n, restoreKey).Scan(&restoredGrant); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE store_credit_allocations SET restored_cents=restored_cents+$3 WHERE order_id=$1 AND grant_id=$2`, order, a.id, a.n); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO store_credit_history(user_id,grant_id,order_id,amount_cents,kind,source_key) VALUES($1,$2,$3,$4,'restored',$5)`, user, restoredGrant, order, a.n, restoreKey); err != nil {
			return err
		}
	}
	return nil
}

func checkPolicyCheckout(ctx context.Context, tx pgx.Tx, user, cart int64, lock bool) error {
	query := `SELECT shipping_mode,COALESCE(pilot_user_id,0),COALESCE(pilot_variant_id,0),COALESCE(pilot_order_id,0),published_at IS NOT NULL FROM marketplace_policy_settings WHERE id`
	if lock {
		query += " FOR UPDATE"
	}
	var mode string
	var allowedUser, variant, order int64
	var published bool
	if err := tx.QueryRow(ctx, query).Scan(&mode, &allowedUser, &variant, &order, &published); err != nil {
		return err
	}
	if !published || mode != "pilot" && mode != "live" {
		return ports.ErrDeliveryUnavailable
	}
	var count, quantity int
	var actual int64
	var ready bool
	if err := tx.QueryRow(ctx, `SELECT count(*),COALESCE(sum(ci.quantity),0),COALESCE(min(ci.variant_id),0),
	      COALESCE(bool_and(COALESCE(v.stock>=ci.quantity AND ($2<>'live' OR v.selected),false)),false)
	    FROM cart_items ci LEFT JOIN marketplace_ready_variants v ON v.variant_id=ci.variant_id
	    WHERE ci.cart_id=$1`, cart, mode).Scan(&count, &quantity, &actual, &ready); err != nil {
		return err
	}
	if !ready || mode == "pilot" && (user != allowedUser || variant != actual || count != 1 || quantity != 1 || order != 0) {
		return ports.ErrDeliveryUnavailable
	}
	return nil
}
