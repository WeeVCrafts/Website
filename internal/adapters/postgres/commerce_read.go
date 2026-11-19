package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"net/url"
	"strconv"
	"strings"
)

type CommerceReadRepository struct {
	pool   *pgxpool.Pool
	readTx pgx.Tx
}

func NewCommerceReadRepository(p *pgxpool.Pool) *CommerceReadRepository {
	return &CommerceReadRepository{pool: p}
}

func (r *CommerceReadRepository) OrderForActor(ctx context.Context, actor int64, number string) (v ports.CommerceOrderSummary, err error) {
	tx, owned, err := r.beginRead(ctx)
	if err != nil {
		return v, err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	a, err := marketplaceIdentity(ctx, tx, actor)
	if err != nil {
		return v, err
	}
	v.Role = a.role
	err = tx.QueryRow(ctx, `SELECT id,order_number,lifecycle_status,status,currency,workflow_version,created_at,total_cents,credit_cents FROM orders o WHERE order_number=$1 AND ($2 OR ($3='customer' AND user_id=$4) OR ($3='seller' AND EXISTS(SELECT 1 FROM order_items WHERE order_id=o.id AND seller_id=$5)))`, number, a.company(), a.role, actor, a.seller).Scan(&v.ID, &v.Number, &v.Status, &v.LegacyStatus, &v.Currency, &v.WorkflowVersion, &v.CreatedAt, &v.Financial.OriginalTotalCents, &v.Financial.CreditAppliedCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ports.ErrOrderNotFound
	}
	if err != nil {
		return v, err
	}
	sellerScope := int64(0)
	if a.role == "seller" {
		sellerScope = a.seller
	}
	v.Financial, err = readOrderFinancial(ctx, tx, v.ID, sellerScope)
	if err != nil {
		return v, err
	}
	v.Items, err = readOrderItems(ctx, tx, v.ID, sellerScope)
	if err != nil {
		return v, err
	}
	if v.WorkflowVersion != 2 {
		v.Status = v.LegacyStatus
		legacyRows, e := tx.Query(ctx, `SELECT s.display_name,COALESCE(sf.status,'pending'),COALESCE(sf.carrier,''),COALESCE(sf.tracking_number,'') FROM (SELECT DISTINCT seller_id FROM order_items WHERE order_id=$1) owned JOIN sellers s ON s.id=owned.seller_id LEFT JOIN seller_fulfillments sf ON sf.seller_id=s.id AND sf.order_id=$1 WHERE ($2=0 OR s.id=$2) ORDER BY s.id`, v.ID, sellerScope)
		if e != nil {
			return v, e
		}
		for legacyRows.Next() {
			var f commerce.OrderFulfillment
			if e = legacyRows.Scan(&f.SellerName, &f.Status, &f.Carrier, &f.TrackingNumber); e != nil {
				break
			}
			v.LegacyShipments = append(v.LegacyShipments, f)
		}
		if e = errorsJoinRows(e, legacyRows); e != nil {
			return v, e
		}
	}
	var rows pgx.Rows
	if a.role == "seller" {
		v.Financial.SellerScoped = true
		v.Financial.OriginalTotalCents = 0
		for _, x := range v.Items {
			v.Financial.OriginalTotalCents += x.PaidCents
		}
		// The customer-wide provider charge must not be exposed as seller revenue.
		v.Financial.CapturedCents = 0
		v.Financial.ProviderRefundedCents = 0
		v.Financial.NetProviderPaidCents = 0
		v.Financial.CreditAppliedCents = 0
	}
	if a.role != "seller" {
		rows, err = tx.Query(ctx, `SELECT ticket_number,status,id FROM support_tickets WHERE order_id=$1 AND ($2 OR user_id=$3) ORDER BY updated_at DESC LIMIT 50`, v.ID, a.company(), actor)
		if err != nil {
			return v, err
		}
		for rows.Next() {
			var x ports.OrderSupportReference
			var id int64
			if err = rows.Scan(&x.Reference, &x.Status, &id); err != nil {
				break
			}
			x.Kind = "Customer conversation"
			x.URL = "/account/support?ticket_id=" + fmt.Sprint(id)
			if a.company() {
				x.URL = companyReadRoot(a.role) + "/support?ticket_id=" + fmt.Sprint(id)
			}
			v.Support = append(v.Support, x)
		}
		if err = errorsJoinRows(err, rows); err != nil {
			return v, err
		}
	}
	if a.role == "seller" || a.company() {
		rows, err = tx.Query(ctx, `SELECT id,status FROM seller_confirmations WHERE order_id=$1 AND ($2 OR seller_id=$3) ORDER BY updated_at DESC LIMIT 50`, v.ID, a.company(), a.seller)
		if err != nil {
			return v, err
		}
		for rows.Next() {
			var x ports.OrderSupportReference
			var id int64
			if err = rows.Scan(&id, &x.Status); err != nil {
				break
			}
			x.Kind = "Private seller coordination"
			x.Reference = fmt.Sprintf("CONF-%d", id)
			x.URL = "/seller-admin/support"
			if a.company() {
				x.URL = companyReadRoot(a.role) + "/support"
			}
			v.Support = append(v.Support, x)
		}
		if err = errorsJoinRows(err, rows); err != nil {
			return v, err
		}
	}
	if a.company() {
		rows, err = tx.Query(ctx, `SELECT c.kind,c.id,c.status,COALESCE(u.display_name,''),c.next_action FROM marketplace_cases c LEFT JOIN users u ON u.id=c.assignee_id WHERE c.order_id=$1 ORDER BY c.created_at DESC LIMIT 50`, v.ID)
		if err != nil {
			return v, err
		}
		for rows.Next() {
			var x ports.OrderSupportReference
			var id int64
			if err = rows.Scan(&x.Kind, &id, &x.Status, &x.Owner, &x.NextAction); err != nil {
				break
			}
			x.Reference = fmt.Sprintf("CASE-%d", id)
			x.URL = companyReadRoot(a.role) + "/orders/" + url.PathEscape(v.Number) + "?tab=support&case_id=" + fmt.Sprint(id)
			v.Support = append(v.Support, x)
		}
		if err = errorsJoinRows(err, rows); err != nil {
			return v, err
		}
	}
	if owned {
		err = tx.Commit(ctx)
	}
	return v, err
}

func companyReadRoot(role string) string {
	if role == "super_admin" {
		return "/super-admin"
	}
	return "/admin"
}

const requestBucketSQL = `r.bucket`

func (r *CommerceReadRepository) RequestsForActor(ctx context.Context, actor int64, q ports.RequestQuery) (p ports.RequestPage, err error) {
	p.Page = q.Page
	if p.Page < 1 {
		p.Page = 1
	}
	p.Counts = map[string]int{"": 0, "needs_review": 0, "in_progress": 0, "completed": 0, "rejected": 0}
	if _, ok := p.Counts[q.Bucket]; !ok {
		q.Bucket = ""
	}
	tx, owned, err := r.beginRead(ctx)
	if err != nil {
		return p, err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	a, err := marketplaceIdentity(ctx, tx, actor)
	if err != nil {
		return p, err
	}
	where := ` FROM commerce_request_summary r JOIN orders o ON o.id=r.order_id WHERE ($1 OR ($2='customer' AND o.user_id=$3) OR ($2='seller' AND (r.seller_id=$4 OR (r.seller_id IS NULL AND EXISTS(SELECT 1 FROM order_items oi WHERE oi.order_id=o.id AND oi.seller_id=$4))))) AND ($5='' OR o.order_number=$5) AND ($6='' OR strpos(lower(o.order_number||' '||r.kind||' '||r.item),lower($6))>0)`
	args := []any{a.company(), a.role, actor, a.seller, q.OrderNumber, strings.TrimSpace(q.Search)}
	rows, err := tx.Query(ctx, `SELECT `+requestBucketSQL+`,count(*)`+where+` GROUP BY 1`, args...)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var bucket string
		var n int
		if err = rows.Scan(&bucket, &n); err != nil {
			break
		}
		p.Counts[bucket] = n
		p.Counts[""] += n
	}
	if err = errorsJoinRows(err, rows); err != nil {
		return p, err
	}
	if kind, raw, ok := strings.Cut(q.Focus, ":"); ok {
		id, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr == nil && id > 0 {
			var x ports.CommerceRequest
			detailArgs := append(append([]any{}, args[:6]...), kind, id)
			detailErr := tx.QueryRow(ctx, `SELECT r.kind,r.id,r.order_id,COALESCE(r.seller_id,0),o.order_number,r.item,r.seller,r.status,`+requestBucketSQL+`,r.reason,COALESCE(r.amount_cents,0),r.created_at`+where+` AND r.kind=$7 AND r.id=$8`, detailArgs...).Scan(&x.Kind, &x.ID, &x.OrderID, &x.SellerID, &x.Number, &x.Item, &x.Seller, &x.Status, &x.Bucket, &x.Reason, &x.AmountCents, &x.CreatedAt)
			if detailErr != nil && !errors.Is(detailErr, pgx.ErrNoRows) {
				return p, detailErr
			}
			if detailErr == nil {
				x.AmountKnown = true
				if a.role == "seller" && x.SellerID == 0 {
					x.AmountKnown = false
					x.AmountCents = 0
					x.Reason = "Historical order request; contact company support for your item outcome."
				}
				p.Selected = &x
			}
		}
	}
	p.Total = p.Counts[q.Bucket]
	p.Pages = (p.Total + 19) / 20
	if p.Pages == 0 {
		p.Page = 1
	}
	if p.Pages > 0 && p.Page > p.Pages {
		p.Page = p.Pages
	}
	args = append(args, q.Bucket, (p.Page-1)*20)
	rows, err = tx.Query(ctx, `SELECT r.kind,r.id,r.order_id,COALESCE(r.seller_id,0),o.order_number,r.item,r.seller,r.status,`+requestBucketSQL+`,r.reason,COALESCE(r.amount_cents,0),r.created_at`+where+` AND ($7='' OR (`+requestBucketSQL+`)=$7) ORDER BY r.created_at DESC,r.kind,r.id DESC LIMIT 20 OFFSET $8`, args...)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var x ports.CommerceRequest
		if err = rows.Scan(&x.Kind, &x.ID, &x.OrderID, &x.SellerID, &x.Number, &x.Item, &x.Seller, &x.Status, &x.Bucket, &x.Reason, &x.AmountCents, &x.CreatedAt); err != nil {
			break
		}
		x.AmountKnown = true
		if a.role == "seller" && x.SellerID == 0 {
			x.AmountKnown = false
			x.AmountCents = 0
			x.Reason = "Historical order request; contact company support for your item outcome."
		}
		p.Rows = append(p.Rows, x)
	}
	if err = errorsJoinRows(err, rows); err != nil {
		return p, err
	}
	if owned {
		err = tx.Commit(ctx)
	}
	return p, err
}
