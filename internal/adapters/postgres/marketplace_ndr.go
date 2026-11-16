package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
	"time"
)

func queueNDR(ctx context.Context, tx pgx.Tx, order int64, c ports.MarketplaceCommand) error {
	action := "return"
	if c.Action == "ndr_reattempt" {
		action = "re-attempt"
		date, err := time.Parse("2006-01-02", c.PreferredDate)
		today := time.Now().In(time.FixedZone("IST", 19800)).Format("2006-01-02")
		if err != nil || c.PreferredDate <= today || date.Weekday() == time.Sunday {
			return ports.ErrShippingInput
		}
	}
	var id int64
	err := tx.QueryRow(ctx, `INSERT INTO marketplace_ndr_actions(package_id,action,preferred_date,comments,request_key) SELECT id,$2,NULLIF($3,'')::date,$4,$5 FROM marketplace_packages WHERE id=$1 AND order_id=$6 AND status='ndr' AND awb<>'' AND NOT EXISTS(SELECT 1 FROM marketplace_ndr_actions WHERE package_id=$1 AND status IN ('queued','submitting')) RETURNING id`, c.PackageID, action, c.PreferredDate, c.Reason, c.Key, order).Scan(&id)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `SELECT marketplace_enqueue('ndr',$1,$2)`, id, "ndr:"+c.Key)
	return err
}
func (r *MarketplaceRepository) NDRAction(ctx context.Context, id int64) (ports.Shipment, string, string, string, bool, error) {
	var parcel int64
	var action, date, comments, status string
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if e := tx.QueryRow(ctx, `SELECT package_id,action,COALESCE(preferred_date::text,''),comments,status FROM marketplace_ndr_actions WHERE id=$1 FOR UPDATE`, id).Scan(&parcel, &action, &date, &comments, &status); e != nil {
			return e
		}
		if status == "queued" {
			_, e := tx.Exec(ctx, `UPDATE marketplace_ndr_actions SET status='submitting' WHERE id=$1`, id)
			return e
		}
		return nil
	})
	if err != nil {
		return ports.Shipment{}, "", "", "", false, err
	}
	order, err := r.ShippingOrder(ctx, parcel)
	return order.Shipment, action, date, comments, status == "queued", err
}
func (r *MarketplaceRepository) ConfirmNDRAction(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE marketplace_ndr_actions SET status='accepted' WHERE id=$1`, id)
	return err
}
