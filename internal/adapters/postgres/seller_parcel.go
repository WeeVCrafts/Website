package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/adapters/postgres/generated"
	"github.com/wecratfs/commerce/internal/ports"
)

const sellerParcelOptionFields = `SELECT v.id,p.name,v.sku,parcel.source,
	COALESCE(v.parcel_profile_id,0),v.use_seller_parcel_default,parcel.has_override
	FROM product_variants v JOIN products p ON p.id=v.product_id
	JOIN marketplace_variant_parcels parcel ON parcel.variant_id=v.id
	WHERE p.seller_id=$1 AND p.status<>'archived' AND v.status='active'`

func sellerParcelOwner(ctx context.Context, q generated.DBTX, actor int64) (int64, error) {
	a, err := marketplaceIdentity(ctx, q, actor)
	if err != nil {
		return 0, err
	}
	if a.role != "seller" || a.seller <= 0 {
		return 0, ports.ErrForbidden
	}
	return a.seller, nil
}

func readSellerParcelOptions(ctx context.Context, q generated.DBTX, suffix string, args ...any) ([]ports.SellerParcelOption, error) {
	rows, err := q.Query(ctx, sellerParcelOptionFields+suffix, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var options []ports.SellerParcelOption
	for rows.Next() {
		var option ports.SellerParcelOption
		if err := rows.Scan(&option.ID, &option.Product, &option.SKU, &option.Source, &option.ProfileID, &option.FollowsDefault, &option.Measured); err != nil {
			return nil, err
		}
		options = append(options, option)
	}
	return options, rows.Err()
}

func (r *MarketplaceRepository) SellerParcelSetup(ctx context.Context, actor, before int64) (ports.SellerParcelSetup, error) {
	var data ports.SellerParcelSetup
	seller, err := sellerParcelOwner(ctx, r.pool, actor)
	if err != nil {
		return data, err
	}
	rows, err := r.pool.Query(ctx, `SELECT pp.id,pp.name,pp.weight_grams,pp.length_cm,pp.width_cm,pp.height_cm,
		COALESCE(d.profile_id=pp.id,false) FROM seller_parcel_profiles pp
		LEFT JOIN seller_parcel_defaults d ON d.seller_id=pp.seller_id
		WHERE pp.seller_id=$1 ORDER BY pp.name,pp.id`, seller)
	if err != nil {
		return data, err
	}
	defer rows.Close()
	for rows.Next() {
		var profile ports.SellerParcelProfile
		var isDefault bool
		if err := rows.Scan(&profile.ID, &profile.Name, &profile.WeightGrams, &profile.LengthCM, &profile.WidthCM, &profile.HeightCM, &isDefault); err != nil {
			return data, err
		}
		if isDefault {
			data.DefaultID = profile.ID
		}
		data.Profiles = append(data.Profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return data, err
	}
	data.Options, err = readSellerParcelOptions(ctx, r.pool, " AND ($2=0 OR v.id<$2) ORDER BY v.id DESC LIMIT 26", seller, before)
	if len(data.Options) > 25 {
		data.Next = data.Options[24].ID
		data.Options = data.Options[:25]
	}
	return data, err
}

func (r *MarketplaceRepository) CreateSellerParcel(ctx context.Context, actor int64, name string, p ports.Parcel) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 80 || p.WeightGrams < 100 || p.WeightGrams > 50000 ||
		p.LengthCM < 1 || p.LengthCM > 200 || p.WidthCM < 1 || p.WidthCM > 200 || p.HeightCM < 1 || p.HeightCM > 200 {
		return ports.ErrShippingInput
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		seller, err := sellerParcelOwner(ctx, tx, actor)
		if err != nil {
			return err
		}
		var id int64
		err = tx.QueryRow(ctx, `INSERT INTO seller_parcel_profiles(seller_id,name,weight_grams,length_cm,width_cm,height_cm)
			VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(seller_id,name) DO NOTHING RETURNING id`,
			seller, name, p.WeightGrams, p.LengthCM, p.WidthCM, p.HeightCM).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrFulfillmentConflict
		}
		if err != nil {
			return err
		}
		return auditSellerParcel(ctx, tx, actor, seller, "created", map[string]any{"profile_id": id})
	})
}

func sellerParcelPreview(ctx context.Context, q generated.DBTX, seller int64, change ports.SellerParcelChange) (ports.SellerParcelPreview, error) {
	preview := ports.SellerParcelPreview{Change: change}
	if change.Action != "assign" && change.Action != "default" ||
		change.Action == "default" && (change.ProfileID <= 0 || change.FollowDefault || len(change.OptionIDs) != 0) ||
		change.Action == "assign" && (len(change.OptionIDs) == 0 || len(change.OptionIDs) > 25 || change.FollowDefault && change.ProfileID != 0) {
		return preview, ports.ErrShippingInput
	}
	var currentDefault int64
	err := q.QueryRow(ctx, `SELECT COALESCE((SELECT profile_id FROM seller_parcel_defaults WHERE seller_id=$1),0)`, seller).Scan(&currentDefault)
	if err != nil {
		return preview, err
	}
	profileID := change.ProfileID
	if change.FollowDefault {
		profileID = currentDefault
		if profileID == 0 {
			return preview, ports.ErrShippingInput
		}
	}
	if profileID > 0 {
		p := &preview.Profile
		err = q.QueryRow(ctx, `SELECT id,name,weight_grams,length_cm,width_cm,height_cm FROM seller_parcel_profiles
			WHERE seller_id=$1 AND id=$2`, seller, profileID).Scan(&p.ID, &p.Name, &p.WeightGrams, &p.LengthCM, &p.WidthCM, &p.HeightCM)
		if errors.Is(err, pgx.ErrNoRows) {
			return preview, ports.ErrNotFound
		}
		if err != nil {
			return preview, err
		}
	}
	if change.Action == "default" {
		preview.Options, err = readSellerParcelOptions(ctx, q, " AND v.use_seller_parcel_default AND NOT parcel.has_override ORDER BY v.id", seller)
	} else {
		preview.Options, err = readSellerParcelOptions(ctx, q, " AND v.id=ANY($2) AND NOT parcel.has_override ORDER BY v.id", seller, change.OptionIDs)
		if err == nil && len(preview.Options) != len(change.OptionIDs) {
			return preview, ports.ErrFulfillmentConflict
		}
	}
	if err != nil {
		return preview, err
	}
	state, err := json.Marshal(struct {
		Seller, Default int64
		Preview         ports.SellerParcelPreview
	}{seller, currentDefault, preview})
	if err != nil {
		return preview, err
	}
	preview.Token = fmt.Sprintf("%x", sha256.Sum256(state))
	return preview, nil
}

func (r *MarketplaceRepository) PreviewSellerParcelChange(ctx context.Context, actor int64, change ports.SellerParcelChange) (ports.SellerParcelPreview, error) {
	seller, err := sellerParcelOwner(ctx, r.pool, actor)
	if err != nil {
		return ports.SellerParcelPreview{}, err
	}
	return sellerParcelPreview(ctx, r.pool, seller, change)
}

func (r *MarketplaceRepository) SaveSellerParcelChange(ctx context.Context, actor int64, change ports.SellerParcelChange, token string) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		seller, err := sellerParcelOwner(ctx, tx, actor)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT id FROM sellers WHERE id=$1 AND status='active' FOR NO KEY UPDATE`, seller).Scan(&seller); err != nil {
			return err
		}
		// Serialize with checkout parcel reads and variant edits, then recheck the preview.
		_, err = tx.Exec(ctx, `SELECT v.id FROM product_variants v JOIN products p ON p.id=v.product_id
			WHERE p.seller_id=$1 AND (v.id=ANY($2) OR ($3 AND v.use_seller_parcel_default))
			ORDER BY v.id FOR UPDATE OF v`, seller, change.OptionIDs, change.Action == "default")
		if err != nil {
			return err
		}
		preview, err := sellerParcelPreview(ctx, tx, seller, change)
		if err != nil {
			return err
		}
		if token != preview.Token {
			return ports.ErrFulfillmentConflict
		}
		if change.Action == "default" {
			_, err = tx.Exec(ctx, `INSERT INTO seller_parcel_defaults(seller_id,profile_id) VALUES($1,$2)
				ON CONFLICT(seller_id) DO UPDATE SET profile_id=EXCLUDED.profile_id`, seller, change.ProfileID)
		} else {
			_, err = tx.Exec(ctx, `UPDATE product_variants SET parcel_profile_id=NULLIF($1,0),
				use_seller_parcel_default=$2,updated_at=now() WHERE id=ANY($3)`,
				change.ProfileID, change.FollowDefault, change.OptionIDs)
		}
		if err != nil {
			return err
		}
		return auditSellerParcel(ctx, tx, actor, seller, change.Action, change)
	})
}

func auditSellerParcel(ctx context.Context, tx pgx.Tx, actor, seller int64, action string, data any) error {
	metadata, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata)
		VALUES($1,$2,'seller',$3,$4,$5)`, fmt.Sprint(actor), "fulfillment.parcel_profile_"+action,
		fmt.Sprint(seller), ports.RequestID(ctx), metadata)
	return err
}
