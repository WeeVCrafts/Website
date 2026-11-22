package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type CheckoutHealth struct {
	CheckedAt, ValidUntil time.Time
	Blockers              []string
	Enabled, Paused       bool
}

// InitializeAutomaticCheckout removes the legacy pilot fallback before a live
// instance can serve orders. A restart preserves the last recorded result.
func (r *MarketplaceRepository) InitializeAutomaticCheckout(ctx context.Context, settingsKey string) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT id FROM marketplace_policy_settings WHERE id FOR UPDATE`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO marketplace_checkout_health(id,checked_at,valid_until,blockers,settings_key,pickup_postcodes)
			VALUES(true,now(),now(),ARRAY['unchecked'],$1,'{}') ON CONFLICT (id) DO UPDATE
			SET checked_at=EXCLUDED.checked_at,valid_until=EXCLUDED.valid_until,blockers=EXCLUDED.blockers,
				settings_key=EXCLUDED.settings_key,pickup_postcodes=EXCLUDED.pickup_postcodes
			WHERE marketplace_checkout_health.settings_key<>EXCLUDED.settings_key`, settingsKey)
		return err
	})
}

func (r *MarketplaceRepository) CheckoutHealth(ctx context.Context) (CheckoutHealth, error) {
	var h CheckoutHealth
	err := r.pool.QueryRow(ctx, `SELECT h.id IS NOT NULL, m.checkout_paused,
		COALESCE(h.checked_at,'epoch'),COALESCE(h.valid_until,'epoch'),COALESCE(h.blockers,'{}')
		FROM marketplace_policy_settings m LEFT JOIN marketplace_checkout_health h ON h.id WHERE m.id`).
		Scan(&h.Enabled, &h.Paused, &h.CheckedAt, &h.ValidUntil, &h.Blockers)
	return h, err
}

func (r *MarketplaceRepository) SaveCheckoutHealth(ctx context.Context, settingsKey string, startedAt time.Time, blockers []string, pickups map[string]string) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		// Checkout and the pause operation acquire this same lock.
		if _, err := tx.Exec(ctx, `SELECT id FROM marketplace_policy_settings WHERE id FOR UPDATE`); err != nil {
			return err
		}
		result, err := tx.Exec(ctx, `UPDATE marketplace_checkout_health SET checked_at=$2,valid_until=$2::timestamptz+interval '5 minutes',
			blockers=COALESCE($3::text[],'{}'),pickup_postcodes=COALESCE($4::jsonb,'{}')
			WHERE id AND settings_key=$1 AND checked_at<=$2`, settingsKey, startedAt, blockers, pickups)
		if err != nil || result.RowsAffected() == 0 {
			return err
		}
		if len(blockers) == 0 {
			// The existing shipping workers use this setting for operational pause.
			if _, err := tx.Exec(ctx, `UPDATE marketplace_policy_settings SET shipping_mode='live',live_catalog_scope='ready',
				live_catalog_seller_id=NULL,updated_at=now() WHERE id AND NOT checkout_paused AND published_at IS NOT NULL
				AND (shipping_mode<>'live' OR live_catalog_scope<>'ready')`); err != nil {
				return err
			}
		}
		return nil
	})
}

// PublishCheckoutGuidance runs outside the order-policy lock. The persistent
// open issue row makes repeated runs and worker retries safe.
func (r *MarketplaceRepository) PublishCheckoutGuidance(ctx context.Context) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		// The readiness views span several tables. Avoid spending the job's
		// bounded time compiling the occasional maintenance query.
		if _, err := tx.Exec(ctx, `SET LOCAL jit=off`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `WITH seller_checks AS (
			SELECT s.owner_user_id,
				CASE WHEN NOT o.invoice_ready THEN 'seller_invoice'
				WHEN o.stock<=0 THEN 'seller_stock'
				WHEN NOT (o.shipping_weight_g BETWEEN 100 AND 50000 AND o.shipping_length_cm BETWEEN 1 AND 200
					AND o.shipping_width_cm BETWEEN 1 AND 200 AND o.shipping_height_cm BETWEEN 1 AND 200) THEN 'seller_parcel'
				WHEN NOT o.tax_ready THEN 'seller_tax' END AS code,
				o.stock>0 AND (NOT o.pickup_ready OR (h.valid_until>statement_timestamp()
					AND NOT (h.blockers && ARRAY['shiprocket_config','shiprocket_pickups'])
					AND NOT COALESCE(h.pickup_postcodes->>sp.pickup_location=sp.postal_code,false))) AS pickup_issue
			FROM marketplace_variant_readiness o JOIN sellers s ON s.id=o.seller_id
			JOIN marketplace_checkout_health h ON h.id
			LEFT JOIN marketplace_shipping_profiles sp ON sp.seller_id=o.seller_id
		), staff AS (
			SELECT u.id,ur.role_slug FROM users u JOIN user_roles ur ON ur.user_id=u.id
			WHERE u.status='active' AND ur.role_slug IN ('super_admin','admin','shipping_support')
		), current_issues AS (
			SELECT owner_user_id AS owner_id,code FROM seller_checks WHERE code IS NOT NULL
			UNION SELECT staff.id,code FROM staff CROSS JOIN (
				SELECT unnest(blockers) AS code FROM marketplace_checkout_health WHERE id
				UNION SELECT 'policy' FROM marketplace_policy_settings WHERE id AND published_at IS NULL
			) platform WHERE staff.role_slug='super_admin'
			UNION SELECT staff.id,'pickup' FROM staff WHERE EXISTS(SELECT 1 FROM seller_checks WHERE pickup_issue)
		), closed AS (
			DELETE FROM marketplace_checkout_issue old WHERE NOT EXISTS(
				SELECT 1 FROM current_issues c WHERE c.owner_id=old.owner_id AND c.code=old.code)
		), opened AS (
			INSERT INTO marketplace_checkout_issue(owner_id,code) SELECT owner_id,code FROM current_issues
			ON CONFLICT DO NOTHING RETURNING id,owner_id,code
		)
		INSERT INTO notifications(user_id,kind,title,body,link_url,source_key)
		SELECT owner_id,'account','Checkout setup needs attention',
			CASE code WHEN 'seller_invoice' THEN 'Review your seller business and invoice details.'
			WHEN 'seller_stock' THEN 'Check stock for your approved products.'
			WHEN 'seller_parcel' THEN 'Add valid packed measurements for your approved products.'
			WHEN 'seller_tax' THEN 'Review the tax details in Fulfillment Setup.'
			WHEN 'pickup' THEN 'At least one seller pickup needs verification. Coordinate with Super Admin.'
			ELSE 'Review the checkout integration checklist.' END,
			CASE WHEN code='seller_stock' THEN '/seller-admin/inventory'
			WHEN code LIKE 'seller_%' THEN '/seller-admin/setup'
			WHEN EXISTS(SELECT 1 FROM staff s WHERE s.id=owner_id AND s.role_slug='super_admin') THEN '/super-admin/fulfillment?step=readiness'
			ELSE '/admin/support' END,
			'checkout-issue:'||id FROM opened`)
		if err != nil {
			return fmt.Errorf("publish seller and staff checkout guidance: %w", err)
		}
		_, err = tx.Exec(ctx, `WITH eligible AS (
			SELECT w.cart_id,c.user_id,w.requested_at FROM marketplace_checkout_wait w JOIN carts c ON c.id=w.cart_id
			WHERE c.status='active' AND c.expires_at>now()
			AND EXISTS(SELECT 1 FROM cart_items WHERE cart_id=c.id)
			AND NOT EXISTS(SELECT 1 FROM cart_items ci WHERE ci.cart_id=c.id
				AND NOT marketplace_option_can_checkout(ci.variant_id,c.user_id,ci.quantity))
			ORDER BY w.requested_at LIMIT 100
		), cleared AS (
			DELETE FROM marketplace_checkout_wait w USING eligible e WHERE w.cart_id=e.cart_id
			RETURNING e.user_id,e.cart_id,e.requested_at
		), expired AS (
			DELETE FROM marketplace_checkout_wait w USING carts c WHERE c.id=w.cart_id
			AND (c.status<>'active' OR c.expires_at<=now())
		)
		INSERT INTO notifications(user_id,kind,title,body,link_url,source_key)
		SELECT user_id,'account','Your cart is available for checkout',
			'Review your cart and confirm delivery to your PIN code.','/cart',
			'checkout-ready:'||cart_id||':'||extract(epoch FROM requested_at) FROM cleared`)
		if err != nil {
			return fmt.Errorf("publish customer checkout guidance: %w", err)
		}
		return nil
	})
}

func (r *CommerceRepository) rememberCheckoutWait(ctx context.Context, user, cart int64) {
	// Record only a checkout attempt for the customer's own active cart. A PIN
	// that cannot be served is not an availability change for the product.
	_, _ = r.pool.Exec(ctx, `INSERT INTO marketplace_checkout_wait(cart_id)
		SELECT c.id FROM carts c WHERE c.id=$1 AND c.user_id=$2 AND c.status='active' AND c.expires_at>now()
		AND EXISTS(SELECT 1 FROM marketplace_checkout_health WHERE id)
		AND EXISTS(SELECT 1 FROM cart_items ci WHERE ci.cart_id=c.id
			AND NOT marketplace_option_can_checkout(ci.variant_id,c.user_id,ci.quantity))
		ON CONFLICT DO NOTHING`, cart, user)
}
