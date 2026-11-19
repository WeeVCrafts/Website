package postgres

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wecratfs/commerce/internal/ports"
	"golang.org/x/sync/singleflight"
)

type ProductDeliveryRepository struct {
	pool     *pgxpool.Pool
	gateway  ports.DeliveryEstimateGateway
	profiles map[int64]DeliveryProfile
	mu       sync.Mutex
	cache    map[deliveryRoute]productDeliveryCacheEntry
	requests singleflight.Group
	slots    chan struct{}
}

type productDeliveryCacheEntry struct {
	result  ports.ProductDeliveryResult
	expires time.Time
}

func NewProductDeliveryRepository(pool *pgxpool.Pool, gateway ports.DeliveryEstimateGateway, profiles map[int64]DeliveryProfile) *ProductDeliveryRepository {
	return &ProductDeliveryRepository{pool: pool, gateway: gateway, profiles: maps.Clone(profiles),
		cache: make(map[deliveryRoute]productDeliveryCacheEntry), slots: make(chan struct{}, 4)}
}

func (r *ProductDeliveryRepository) ProductDelivery(ctx context.Context, slug string, variant int64, quantity int, pin string) (ports.ProductDeliveryResult, error) {
	var line deliveryLine
	var pickup string
	var availableStock int
	err := r.pool.QueryRow(ctx, `
		SELECT p.seller_id,COALESCE(sp.postal_code,''),COALESCE(i.available_quantity,0),
			parcel.shipping_weight_g,parcel.shipping_length_cm,parcel.shipping_width_cm,parcel.shipping_height_cm
		FROM products p JOIN sellers s ON s.id=p.seller_id AND s.status='active'
		JOIN product_variants v ON v.product_id=p.id AND v.status='active'
		JOIN marketplace_variant_parcels parcel ON parcel.variant_id=v.id
		LEFT JOIN marketplace_shipping_profiles sp ON sp.seller_id=p.seller_id AND sp.verified
		LEFT JOIN inventory_stock i ON i.variant_id=v.id
		WHERE p.slug=$1 AND p.status='approved' AND v.id=$2
	`, slug, variant).Scan(&line.seller, &pickup, &availableStock, &line.parcel.WeightGrams, &line.parcel.LengthCM, &line.parcel.WidthCM, &line.parcel.HeightCM)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.ProductDeliveryResult{}, ports.ErrNotFound
	}
	if err != nil {
		return ports.ProductDeliveryResult{}, err
	}
	unavailable := ports.ProductDeliveryResult{State: "unavailable"}
	if r.gateway == nil || pickup == "" || quantity < 1 || quantity > availableStock {
		return unavailable, nil
	}
	profile := r.profiles[line.seller]
	profile.PostalCode = pickup
	line.quantity = quantity
	decisions, err := estimateCheckoutParcels([]deliveryLine{line}, map[int64]DeliveryProfile{line.seller: profile})
	if err != nil {
		return unavailable, nil
	}
	return r.estimate(ctx, deliveryRoute{pickup, pin, decisions[0].estimate})
}

func (r *ProductDeliveryRepository) estimate(ctx context.Context, route deliveryRoute) (ports.ProductDeliveryResult, error) {
	if result, ok := r.cached(route); ok {
		return result, nil
	}
	key := fmt.Sprintf("%s/%s/%+v", route.origin, route.destination, route.parcel)
	call := r.requests.DoChan(key, func() (any, error) {
		if result, ok := r.cached(route); ok {
			return result, nil
		}
		select {
		case r.slots <- struct{}{}:
			defer func() { <-r.slots }()
		default:
			return nil, ports.ErrShippingUnavailable
		}
		checkCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		available, days, err := r.gateway.DeliveryEstimate(checkCtx, route.origin, route.destination, route.parcel)
		if err != nil {
			return nil, err
		}
		result := ports.ProductDeliveryResult{State: "unserviceable"}
		ttl := time.Minute
		if available {
			result = ports.ProductDeliveryResult{State: "available", TransitDays: days}
			ttl = 5 * time.Minute
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if len(r.cache) >= 1024 {
			for k, v := range r.cache {
				if !time.Now().Before(v.expires) {
					delete(r.cache, k)
				}
			}
		}
		if len(r.cache) < 1024 {
			r.cache[route] = productDeliveryCacheEntry{result, time.Now().Add(ttl)}
		}
		return result, nil
	})
	select {
	case <-ctx.Done():
		return ports.ProductDeliveryResult{}, ctx.Err()
	case loaded := <-call:
		if loaded.Err != nil {
			return ports.ProductDeliveryResult{}, loaded.Err
		}
		return loaded.Val.(ports.ProductDeliveryResult), nil
	}
}

func (r *ProductDeliveryRepository) cached(route deliveryRoute) (ports.ProductDeliveryResult, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.cache[route]
	return entry.result, ok && time.Now().Before(entry.expires)
}
