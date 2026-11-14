package postgres

import (
	"context"
	"encoding/json"
	"maps"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

type DeliveryProfile struct {
	PostalCode              string
	PackingGrams, PackingCM int
}

type checkoutDelivery struct {
	checker     ports.CheckoutDeliveryChecker
	environment string
	profiles    map[int64]DeliveryProfile
	mu          sync.Mutex
	cache       map[deliveryRoute]deliveryCacheEntry
}

type deliveryRoute struct {
	origin, destination string
	parcel              ports.Parcel
}

type deliveryCacheEntry struct {
	available bool
	expires   time.Time
}

type deliveryLine struct {
	variant, seller int64
	quantity        int
	parcel          ports.Parcel
}
type deliveryDecision struct {
	seller   int64
	method   string
	estimate ports.Parcel
}

func (r *CommerceRepository) ConfigureCheckoutDelivery(checker ports.CheckoutDeliveryChecker, profiles map[int64]DeliveryProfile, environment string) {
	r.delivery = &checkoutDelivery{checker: checker, environment: environment, profiles: maps.Clone(profiles), cache: make(map[deliveryRoute]deliveryCacheEntry)}
}

type deliveryQuery interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func checkoutDeliveryLines(ctx context.Context, q deliveryQuery, user, cart int64, lock bool) ([]deliveryLine, error) {
	query := `SELECT v.id,COALESCE(p.seller_id,0),ci.quantity,v.shipping_weight_g,v.shipping_length_cm,v.shipping_width_cm,v.shipping_height_cm
	FROM carts c JOIN cart_items ci ON ci.cart_id=c.id JOIN product_variants v ON v.id=ci.variant_id JOIN products p ON p.id=v.product_id
	WHERE c.id=$1 AND c.user_id=$2 AND c.status='active' AND c.expires_at>now() ORDER BY v.id`
	if lock {
		query += ` FOR SHARE OF v`
	}
	rows, err := q.Query(ctx, query, cart, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lines []deliveryLine
	for rows.Next() {
		var l deliveryLine
		if err := rows.Scan(&l.variant, &l.seller, &l.quantity, &l.parcel.WeightGrams, &l.parcel.LengthCM, &l.parcel.WidthCM, &l.parcel.HeightCM); err != nil {
			return nil, err
		}
		lines = append(lines, l)
	}
	return lines, rows.Err()
}

func (r *CommerceRepository) CheckDelivery(ctx context.Context, user, cart int64, pin string) error {
	if r.delivery == nil || pin == "" {
		return nil
	}
	lines, err := checkoutDeliveryLines(ctx, r.pool, user, cart, false)
	if err != nil {
		return err
	}
	_, err = r.delivery.check(ctx, lines, pin)
	return err
}

func (d *checkoutDelivery) check(ctx context.Context, lines []deliveryLine, pin string) ([]deliveryDecision, error) {
	var decisions []deliveryDecision
	indexes := make(map[int64]int)
	for _, line := range lines {
		i, ok := indexes[line.seller]
		if !ok {
			i = len(decisions)
			indexes[line.seller] = i
			decisions = append(decisions, deliveryDecision{seller: line.seller, method: "manual"})
		}
		if _, mapped := d.profiles[line.seller]; !mapped {
			continue
		}
		if !line.parcel.Valid() || line.quantity < 1 || line.quantity > 50000 {
			return nil, ports.ErrShippingParcelMissing
		}
		estimate := &decisions[i].estimate
		length, width := max(line.parcel.LengthCM, line.parcel.WidthCM), min(line.parcel.LengthCM, line.parcel.WidthCM)
		estimate.WeightGrams += line.parcel.WeightGrams * line.quantity
		estimate.LengthCM = max(estimate.LengthCM, length)
		estimate.WidthCM = max(estimate.WidthCM, width)
		estimate.HeightCM += line.parcel.HeightCM * line.quantity
		if !estimate.Valid() {
			return nil, ports.ErrShippingParcelMissing
		}
	}
	for i := range decisions {
		decision := &decisions[i]
		profile, mapped := d.profiles[decision.seller]
		if !mapped {
			continue
		}
		estimate := &decision.estimate
		estimate.WeightGrams = max(100, estimate.WeightGrams+profile.PackingGrams)
		estimate.LengthCM += profile.PackingCM
		estimate.WidthCM += profile.PackingCM
		estimate.HeightCM += profile.PackingCM
		if !estimate.Valid() {
			return nil, ports.ErrShippingParcelMissing
		}
	}
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for i := range decisions {
		decision := &decisions[i]
		profile, mapped := d.profiles[decision.seller]
		if !mapped {
			continue
		}
		available, err := d.available(checkCtx, deliveryRoute{profile.PostalCode, pin, decision.estimate})
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err != nil {
			decision.method = "manual_fallback"
			continue
		}
		if !available {
			return nil, ports.ErrDeliveryUnavailable
		}
		decision.method = "shiprocket"
	}
	return decisions, nil
}

func (d *checkoutDelivery) available(ctx context.Context, route deliveryRoute) (bool, error) {
	now := time.Now()
	d.mu.Lock()
	entry, ok := d.cache[route]
	d.mu.Unlock()
	if ok && now.Before(entry.expires) {
		return entry.available, nil
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	available, err := d.checker.DeliveryAvailable(ctx, route.origin, route.destination, route.parcel)
	if err != nil {
		return false, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.cache) >= 4096 {
		for key, value := range d.cache {
			if !now.Before(value.expires) {
				delete(d.cache, key)
			}
		}
	}
	if len(d.cache) < 4096 {
		d.cache[route] = deliveryCacheEntry{available, time.Now().Add(5 * time.Minute)}
	}
	return available, nil
}

func (d *checkoutDelivery) saveDecisions(ctx context.Context, tx pgx.Tx, order int64, decisions []deliveryDecision) error {
	for _, decision := range decisions {
		estimate, err := json.Marshal(decision.estimate)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO order_delivery_decisions(order_id,seller_id,decision,estimate) VALUES($1,$2,$3,$4)`, order, decision.seller, decision.method, estimate); err != nil {
			return err
		}
		if decision.method == "manual_fallback" {
			if err = openDeliveryException(ctx, tx, order, decision.seller, d.environment, "checkout_fallback"); err != nil {
				return err
			}
		}
	}
	return nil
}

func openDeliveryException(ctx context.Context, tx pgx.Tx, order, seller int64, environment, kind string) error {
	_, err := tx.Exec(ctx, `INSERT INTO delivery_exceptions(order_id,seller_id,environment,kind) VALUES($1,$2,$3,$4)
 ON CONFLICT(order_id,seller_id,environment,kind) DO UPDATE SET opened_at=now(),assignee_id=NULL,resolved_at=NULL,resolved_by=NULL,resolution=''
 WHERE delivery_exceptions.resolved_at IS NOT NULL AND excluded.kind IN ('pending_operation','document_failed')`, order, seller, environment, kind)
	return err
}
