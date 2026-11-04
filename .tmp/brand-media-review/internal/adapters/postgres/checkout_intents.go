package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *CommerceRepository) CreateCheckoutIntent(ctx context.Context, userID, variantID, quantity int64, key string) (domaincommerce.CheckoutIntent, error) {
	var result domaincommerce.CheckoutIntent
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return result, err
	}
	token := hex.EncodeToString(b)
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		// Serialize identical requests, including retries after a lost response.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key+":"+strconv.FormatInt(userID, 10)); err != nil {
			return err
		}
		var allowed bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.id=$1 AND u.status='active' AND ur.role_slug='customer')`, userID).Scan(&allowed); err != nil {
			return err
		}
		if !allowed {
			return ports.ErrForbidden
		}
		var oldVariant, oldQuantity int64
		err := tx.QueryRow(ctx, `SELECT c.checkout_token,c.id,c.expires_at,ci.variant_id,ci.quantity FROM carts c JOIN cart_items ci ON ci.cart_id=c.id WHERE c.user_id=$1 AND c.purpose='buy_now' AND c.intent_request_key=$2`, userID, key).Scan(&result.ID, &result.CartID, &result.ExpiresAt, &oldVariant, &oldQuantity)
		if err == nil {
			if oldVariant != variantID || oldQuantity != quantity {
				return ports.ErrIdempotencyConflict
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var available int64
		err = tx.QueryRow(ctx, `SELECT i.available_quantity FROM product_variants v JOIN products p ON p.id=v.product_id JOIN sellers s ON s.id=p.seller_id JOIN inventory_stock i ON i.variant_id=v.id WHERE v.id=$1 AND v.status='active' AND p.status='approved' AND s.status='active'`, variantID).Scan(&available)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrInvalidState
		}
		if err != nil {
			return err
		}
		if available < quantity {
			return ports.ErrInsufficientStock
		}
		err = tx.QueryRow(ctx, `INSERT INTO carts(user_id,purpose,checkout_token,intent_request_key,expires_at) VALUES($1,'buy_now',$2,$3,NOW()+INTERVAL '24 hours') RETURNING id,expires_at`, userID, token, key).Scan(&result.CartID, &result.ExpiresAt)
		if err != nil {
			return err
		}
		result.ID = token
		_, err = tx.Exec(ctx, `INSERT INTO cart_items(cart_id,variant_id,quantity) VALUES($1,$2,$3)`, result.CartID, variantID, quantity)
		return err
	})
	return result, err
}

func (r *CommerceRepository) GetCheckoutIntent(ctx context.Context, userID int64, token string) (domaincommerce.CheckoutIntent, error) {
	var intent domaincommerce.CheckoutIntent
	err := r.pool.QueryRow(ctx, `SELECT c.checkout_token,c.id,c.expires_at,COALESCE(o.order_number,'') FROM carts c LEFT JOIN orders o ON o.id=c.checkout_order_id AND o.user_id=c.user_id WHERE c.user_id=$1 AND c.checkout_token=$2 AND c.purpose='buy_now' AND ((c.status='active' AND c.expires_at>NOW()) OR c.status='checked_out')`, userID, token).Scan(&intent.ID, &intent.CartID, &intent.ExpiresAt, &intent.OrderNumber)
	if errors.Is(err, pgx.ErrNoRows) {
		return intent, ports.ErrCartNotFound
	}
	return intent, err
}
