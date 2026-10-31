package handlers

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/wecratfs/commerce/internal/ports"
)

// The caller has already loaded the order through the customer's ownership
// check. Resolve only public product media belonging to that order's items.
func (h *LiveStorefrontHandler) receiptMedia(ctx context.Context, orderNumber string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		reader, ok := h.profileStorage.(ports.ObjectStorageContentReader)
		if !ok || !strings.HasPrefix(path, "/media/products/") {
			return nil, errors.New("receipt image unavailable")
		}
		id, err := strconv.ParseInt(strings.TrimPrefix(path, "/media/products/"), 10, 64)
		if err != nil || id <= 0 {
			return nil, errors.New("invalid receipt media")
		}
		var key string
		var size int64
		err = h.pool.QueryRow(ctx, `SELECT pm.object_key,pm.byte_size FROM product_media pm WHERE pm.id=$1 AND pm.status='ready' AND pm.visibility='public' AND pm.content_type IN ('image/jpeg','image/png','image/webp') AND EXISTS (
		 SELECT 1 FROM order_items oi JOIN orders o ON o.id=oi.order_id JOIN product_variants pv ON pv.id=oi.variant_id WHERE o.order_number=$2 AND pv.product_id=pm.product_id
		)`, id, orderNumber).Scan(&key, &size)
		if err != nil || size > 8<<20 {
			return nil, errors.New("receipt media unavailable")
		}
		body, _, err := reader.OpenObject(ctx, key)
		if err != nil {
			return nil, err
		}
		defer body.Close()
		data, err := io.ReadAll(io.LimitReader(body, (8<<20)+1))
		if len(data) > 8<<20 {
			return nil, errors.New("receipt image too large")
		}
		return data, err
	}
}
