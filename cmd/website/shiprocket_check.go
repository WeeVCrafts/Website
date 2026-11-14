package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wecratfs/commerce/internal/adapters/shiprocket"
	"github.com/wecratfs/commerce/internal/config"
)

func configuredShiprocket(cfg config.ShippingConfig) *shiprocket.Gateway {
	base := shiprocket.SandboxURL
	if cfg.Mode == "live" {
		base = shiprocket.LiveURL
	}
	return shiprocket.New(base, cfg.Email, cfg.Password)
}

func checkShiprocket(cfg config.Config) error {
	if err := cfg.ValidateShipping(); err != nil {
		return err
	}
	if cfg.Shipping.Mode == "" || cfg.Shipping.Mode == "disabled" {
		return errors.New("Shiprocket is disabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	locations, err := configuredShiprocket(cfg.Shipping).PickupPostcodes(ctx)
	if err != nil {
		return err
	}
	var missing []int64
	for seller, location := range cfg.Shipping.PickupLocations {
		expected := cfg.Shipping.ParcelProfiles[seller].PostalCode
		if pin, found := locations[location]; !found || (expected != "" && pin != expected) {
			missing = append(missing, seller)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return fmt.Errorf("pickup location or PIN unverified for seller IDs %v", missing)
	}
	if !cfg.Shipping.BookingsEnabled && cfg.Shipping.Mode != "live" {
		return nil
	}
	poolConfig, err := cfg.DatabasePoolConfig()
	if err != nil {
		return errors.New("could not load database settings for shipment readiness")
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return errors.New("could not connect for shipment readiness")
	}
	defer pool.Close()
	ids := make([]int64, 0, len(cfg.Shipping.PickupLocations))
	for id := range cfg.Shipping.PickupLocations {
		ids = append(ids, id)
	}
	rows, err := pool.Query(ctx, `SELECT configured.id FROM unnest($1::bigint[]) configured(id)
  LEFT JOIN sellers s ON s.id=configured.id
  WHERE s.id IS NULL OR s.status<>'active' OR EXISTS(SELECT 1 FROM products p JOIN product_variants v ON v.product_id=p.id
   WHERE p.seller_id=s.id AND p.status='approved' AND v.status='active' AND v.shipping_weight_g=0)
  ORDER BY configured.id`, ids)
	if err != nil {
		return errors.New("could not read seller readiness from PostgreSQL")
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if rows.Scan(&id) != nil {
			return errors.New("could not read seller readiness from PostgreSQL")
		}
		missing = append(missing, id)
	}
	if rows.Err() != nil {
		return errors.New("could not read seller readiness from PostgreSQL")
	}
	if len(missing) > 0 {
		return fmt.Errorf("inactive seller or missing active variant dimensions for seller IDs %v", missing)
	}
	return nil
}
