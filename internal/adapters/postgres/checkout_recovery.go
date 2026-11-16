package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

// Each cart receives at most one reminder. A failed delivery can retry within
// the provider's idempotency window using the same persisted key.
func (r *MarketplaceRepository) RecoverCheckout(ctx context.Context, sender ports.EmailSender, publicURL string) error {
	if sender == nil {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var cart, user int64
	var message ports.EmailMessage
	err = tx.QueryRow(ctx, `
		SELECT c.id,u.id,u.email,u.display_name
		FROM checkout_recovery cr JOIN carts c ON c.id=cr.cart_id
		JOIN users u ON u.id=c.user_id AND u.status='active'
		JOIN marketing_preferences mp ON mp.user_id=u.id AND mp.email_marketing
		WHERE cr.order_id IS NULL AND cr.notice_sent_at IS NULL
			AND cr.last_quoted_at<=now()-interval '1 hour'
			AND cr.first_quoted_at>now()-interval '24 hours'
			AND (cr.notice_started_at IS NULL OR cr.notice_started_at>now()-interval '12 hours')
			AND c.status='active' AND c.expires_at>now()
			AND (SELECT array_agg(role_slug ORDER BY role_slug) FROM user_roles WHERE user_id=u.id)=ARRAY['customer']::text[]
			AND NOT EXISTS(SELECT 1 FROM orders o WHERE o.user_id=u.id AND o.created_at>=cr.first_quoted_at)
			AND EXISTS(SELECT 1 FROM cart_items WHERE cart_id=c.id)
			AND NOT EXISTS(SELECT 1 FROM cart_items ci LEFT JOIN marketplace_ready_variants v ON v.variant_id=ci.variant_id
				WHERE ci.cart_id=c.id AND NOT COALESCE(v.selected AND v.stock>=ci.quantity,false))
			AND EXISTS(SELECT 1 FROM marketplace_policy_settings WHERE id AND published_at IS NOT NULL AND shipping_mode='live')
		ORDER BY cr.last_quoted_at,c.id LIMIT 1 FOR UPDATE OF cr,c,mp SKIP LOCKED
	`).Scan(&cart, &user, &message.To, &message.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	// Serialize with checkout and consent changes until the provider accepts
	// the message. Any subsequent retry uses the same cart idempotency key.
	if _, err = tx.Exec(ctx, `UPDATE checkout_recovery SET notice_started_at=COALESCE(notice_started_at,now()) WHERE cart_id=$1`, cart); err != nil {
		return err
	}
	message.Subject = "Your WeevCrafts cart is available"
	message.Body = "You left items in your cart. Review current prices and delivery options when you are ready. Manage email offers in Account > Notifications."
	message.ActionURL = strings.TrimRight(publicURL, "/") + "/cart"
	message.IdempotencyKey = fmt.Sprintf("checkout-recovery-%d", cart)
	if err = sender.Send(ctx, message); err != nil {
		// Persist the first-attempt boundary even if sending is uncertain.
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return commitErr
		}
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE checkout_recovery SET notice_sent_at=now() WHERE cart_id=$1`, cart); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
