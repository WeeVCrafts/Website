package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *MarketplaceRepository) readSource() orderFactReader {
	if r.readTx != nil {
		return r.readTx
	}
	return r.pool
}
func (r *CommerceReadRepository) beginRead(ctx context.Context) (pgx.Tx, bool, error) {
	if r.readTx != nil {
		return r.readTx, false, nil
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	return tx, true, err
}

// A single database snapshot supplies every section of an order workspace.
// The transaction is confined to copies of the repositories, never shared with
// other requests or command handlers.
func (r *MarketplaceRepository) OrderWorkspace(ctx context.Context, actor int64, number string, query ports.RequestQuery) (summary ports.CommerceOrderSummary, view ports.MarketplaceView, requests ports.RequestPage, err error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	reader := &CommerceReadRepository{pool: r.pool, readTx: tx}
	summary, err = reader.OrderForActor(ctx, actor, number)
	if err != nil {
		return
	}
	copy := *r
	copy.readTx = tx
	view, err = copy.View(ctx, actor, number)
	if err != nil {
		return
	}
	query.OrderNumber = number
	requests, err = reader.RequestsForActor(ctx, actor, query)
	if err != nil {
		return
	}
	if summary.Role == "admin" || summary.Role == "super_admin" {
		rows, e := tx.Query(ctx, `SELECT c.id,c.order_id,COALESCE(c.seller_id,0),COALESCE(c.package_id,0),o.order_number,c.kind,c.status,c.reason,c.created_at,COALESCE(u.display_name,''),c.next_action FROM marketplace_cases c JOIN orders o ON o.id=c.order_id LEFT JOIN users u ON u.id=c.assignee_id WHERE c.order_id=$1 ORDER BY c.id DESC LIMIT 50`, summary.ID)
		if e != nil {
			err = e
			return
		}
		for rows.Next() {
			var c ports.MarketplaceCase
			if e = rows.Scan(&c.ID, &c.OrderID, &c.SellerID, &c.PackageID, &c.Number, &c.Kind, &c.Status, &c.Reason, &c.CreatedAt, &c.Assignee, &c.NextAction); e != nil {
				break
			}
			view.Cases = append(view.Cases, c)
		}
		if err = errorsJoinRows(e, rows); err != nil {
			return
		}
	}
	err = tx.Commit(ctx)
	return
}
