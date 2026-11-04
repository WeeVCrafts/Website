package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

// checkoutDiscount is shared by preview and transactional order placement.
// Placement locks coupon counters before validating their remaining capacity.
func checkoutDiscount(ctx context.Context, q catalogProductQuerier, userID, cartID, shipping int64, code string, lock bool) (couponID, promotionID, discount int64, err error) {
	if code == "" {
		return
	}
	var kind string
	var value int64
	query := `SELECT c.id,p.id,p.discount_type,p.discount_value FROM coupons c JOIN promotions p ON p.id=c.promotion_id
	 WHERE UPPER(c.code)=UPPER($1) AND c.active AND p.status='active'
	 AND (p.starts_at IS NULL OR p.starts_at<=NOW()) AND (p.ends_at IS NULL OR p.ends_at>NOW())
	 AND (c.usage_limit IS NULL OR c.usage_count<c.usage_limit)
	 AND (p.usage_limit IS NULL OR p.usage_count<p.usage_limit)
	 AND NOT EXISTS(SELECT 1 FROM promotion_usages pu WHERE pu.coupon_id=c.id AND pu.user_id=$2)`
	if lock {
		query += " FOR UPDATE OF c,p"
	}
	err = q.QueryRow(ctx, query, code, userID).Scan(&couponID, &promotionID, &kind, &value)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ports.ErrCouponInvalid
	}
	if err != nil {
		return
	}
	var eligible int64
	err = q.QueryRow(ctx, `SELECT COALESCE(SUM(ci.quantity*pv.price_cents),0) FROM cart_items ci JOIN product_variants pv ON pv.id=ci.variant_id JOIN promotions p ON p.id=$2
	 WHERE ci.cart_id=$1 AND (p.targeting_mode='all'
	 OR EXISTS(SELECT 1 FROM promotion_products WHERE promotion_id=$2 AND product_id=pv.product_id))`, cartID, promotionID).Scan(&eligible)
	if err != nil {
		return
	}
	if eligible <= 0 || value < 0 {
		err = ports.ErrCouponInvalid
		return
	}
	switch kind {
	case "percentage":
		if value <= 0 || value > 100 {
			err = ports.ErrCouponInvalid
			return
		}
		discount = eligible * value / 100
	case "fixed":
		discount = min(value, eligible)
	case "free_shipping":
		discount = shipping
	default:
		err = ports.ErrCouponInvalid
	}
	return
}

func (r *CommerceRepository) QuoteOrder(ctx context.Context, userID, cartID int64, coupon string) (domaincommerce.CheckoutQuote, error) {
	var quote domaincommerce.CheckoutQuote
	quote.CartID = cartID
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		revision, e := readPlacementPolicy(ctx, tx)
		if e != nil {
			return e
		}
		quote.PolicyRevision = revision.ID
		err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(ci.quantity*pv.price_cents),0),COALESCE(SUM(ci.quantity),0),
	 COALESCE(BOOL_AND(COALESCE(p.status='approved' AND s.status='active' AND pv.status='active',false) AND COALESCE(i.available_quantity,0)>=ci.quantity),false)
	 FROM carts c JOIN cart_items ci ON ci.cart_id=c.id JOIN product_variants pv ON pv.id=ci.variant_id
	 JOIN products p ON p.id=pv.product_id LEFT JOIN sellers s ON s.id=p.seller_id LEFT JOIN inventory_stock i ON i.variant_id=pv.id
	 WHERE c.id=$1 AND c.user_id=$2 AND c.status='active' AND c.expires_at>NOW()`, cartID, userID).Scan(&quote.SubtotalCents, &quote.ItemCount, &quote.Available)
		if err != nil {
			return err
		}
		if quote.ItemCount == 0 {
			return ports.ErrCartNotFound
		}
		if !quote.Available {
			return ports.ErrInsufficientStock
		}
		if strings.TrimSpace(coupon) != "" && !revision.Config.Settings.Features.Coupons {
			return ports.ErrCouponInvalid
		}
		quote.ShippingCents = revision.Config.Settings.Policy.Shipping(quote.SubtotalCents)
		_, _, quote.DiscountCents, err = checkoutDiscount(ctx, tx, userID, cartID, quote.ShippingCents, strings.TrimSpace(coupon), false)
		quote.TotalCents = quote.SubtotalCents + quote.ShippingCents - quote.DiscountCents
		return err
	})
	return quote, err
}
