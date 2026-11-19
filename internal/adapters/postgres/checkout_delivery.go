package postgres

import (
	"context"
	"encoding/json"
	"maps"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
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
	days      int
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
	days     int
	estimate ports.Parcel
}

func (r *CommerceRepository) ConfigureCheckoutDelivery(checker ports.CheckoutDeliveryChecker, profiles map[int64]DeliveryProfile, environment string) {
	r.delivery = &checkoutDelivery{checker: checker, environment: environment, profiles: maps.Clone(profiles), cache: make(map[deliveryRoute]deliveryCacheEntry)}
}

type deliveryQuery interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func checkoutDeliveryLines(ctx context.Context, q deliveryQuery, user, cart int64, lock bool) ([]deliveryLine, error) {
	if lock {
		// Read effective profile measurements in a fresh statement after taking
		// the variant locks, so a concurrent default change is visible.
		locked, err := q.Query(ctx, `SELECT v.id FROM carts c JOIN cart_items ci ON ci.cart_id=c.id
			JOIN product_variants v ON v.id=ci.variant_id
			WHERE c.id=$1 AND c.user_id=$2 AND c.status='active' AND c.expires_at>now()
			ORDER BY v.id FOR SHARE OF v`, cart, user)
		if err != nil {
			return nil, err
		}
		for locked.Next() {
		}
		if err := locked.Err(); err != nil {
			return nil, err
		}
	}
	query := `SELECT v.id,COALESCE(p.seller_id,0),ci.quantity,parcel.shipping_weight_g,parcel.shipping_length_cm,parcel.shipping_width_cm,parcel.shipping_height_cm
	FROM carts c JOIN cart_items ci ON ci.cart_id=c.id JOIN product_variants v ON v.id=ci.variant_id JOIN products p ON p.id=v.product_id
	JOIN marketplace_variant_parcels parcel ON parcel.variant_id=v.id
	WHERE c.id=$1 AND c.user_id=$2 AND c.status='active' AND c.expires_at>now() ORDER BY v.id`
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
	_, err := r.PreviewDelivery(ctx, user, cart, pin)
	return err
}

func (r *CommerceRepository) PreviewDelivery(ctx context.Context, user, cart int64, pin string) (domain.CheckoutDelivery, error) {
	var result domain.CheckoutDelivery
	if r.delivery == nil || pin == "" {
		return result, nil
	}
	lines, err := checkoutDeliveryLines(ctx, r.pool, user, cart, false)
	if err != nil {
		return result, err
	}
	profiles, err := r.deliveryProfiles(ctx)
	if err != nil {
		return result, err
	}
	decisions, err := r.delivery.check(ctx, lines, pin, profiles)
	if err != nil || len(decisions) == 0 {
		return result, err
	}
	result.State = "confirmed"
	allEstimated := true
	for _, decision := range decisions {
		if decision.method != "shiprocket" {
			result.State = "assisted"
		}
		allEstimated = allEstimated && decision.days > 0
		result.TransitDays = max(result.TransitDays, decision.days)
	}
	if !allEstimated || result.State != "confirmed" {
		result.TransitDays = 0
	}
	return result, nil
}

func (r *CommerceRepository) deliveryProfiles(ctx context.Context) (map[int64]DeliveryProfile, error) {
	profiles := maps.Clone(r.delivery.profiles)
	if profiles == nil {
		profiles = map[int64]DeliveryProfile{}
	}
	if r.pool == nil || r.delivery.environment != "live" {
		return profiles, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT seller_id,postal_code FROM marketplace_shipping_profiles WHERE verified`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var seller int64
		var pin string
		if err = rows.Scan(&seller, &pin); err != nil {
			return nil, err
		}
		profile := profiles[seller]
		profile.PostalCode = pin
		profiles[seller] = profile
	}
	return profiles, rows.Err()
}

func estimateCheckoutParcels(lines []deliveryLine, profiles map[int64]DeliveryProfile) ([]deliveryDecision, error) {
	var decisions []deliveryDecision
	indexes := make(map[int64]int)
	for _, line := range lines {
		i, ok := indexes[line.seller]
		if !ok {
			i = len(decisions)
			indexes[line.seller] = i
			decisions = append(decisions, deliveryDecision{seller: line.seller, method: "manual"})
		}
		if _, mapped := profiles[line.seller]; !mapped {
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
		profile, mapped := profiles[decision.seller]
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
	return decisions, nil
}

func (d *checkoutDelivery) check(ctx context.Context, lines []deliveryLine, pin string, overrides ...map[int64]DeliveryProfile) ([]deliveryDecision, error) {
	profiles := d.profiles
	if len(overrides) > 0 {
		profiles = overrides[0]
	}
	decisions, err := estimateCheckoutParcels(lines, profiles)
	if err != nil {
		return nil, err
	}
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for i := range decisions {
		decision := &decisions[i]
		profile, mapped := profiles[decision.seller]
		if !mapped {
			continue
		}
		available, days, err := d.available(checkCtx, deliveryRoute{profile.PostalCode, pin, decision.estimate})
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
		decision.days = days
	}
	return decisions, nil
}

func (d *checkoutDelivery) available(ctx context.Context, route deliveryRoute) (bool, int, error) {
	now := time.Now()
	d.mu.Lock()
	entry, ok := d.cache[route]
	d.mu.Unlock()
	if ok && now.Before(entry.expires) {
		return entry.available, entry.days, nil
	}
	if err := ctx.Err(); err != nil {
		return false, 0, err
	}
	var available bool
	var days int
	var err error
	if gateway, ok := d.checker.(ports.DeliveryEstimateGateway); ok {
		available, days, err = gateway.DeliveryEstimate(ctx, route.origin, route.destination, route.parcel)
	} else {
		available, err = d.checker.DeliveryAvailable(ctx, route.origin, route.destination, route.parcel)
	}
	if err != nil {
		return false, 0, err
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
		d.cache[route] = deliveryCacheEntry{available: available, days: days, expires: time.Now().Add(5 * time.Minute)}
	}
	return available, days, nil
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
