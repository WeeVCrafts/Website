package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/domain/storefront"
)

func readPlacementPolicy(ctx context.Context, tx pgx.Tx) (storefront.Revision, error) {
	var r storefront.Revision
	var data []byte
	e := tx.QueryRow(ctx, `SELECT r.id,r.config FROM storefront_head h JOIN storefront_releases r ON r.id=h.release_id WHERE h.singleton FOR SHARE OF h`).Scan(&r.ID, &data)
	if errors.Is(e, pgx.ErrNoRows) {
		r.Config = storefront.Defaults()
		return r, nil
	}
	if e == nil {
		e = json.Unmarshal(data, &r.Config)
	}
	return r, e
}
