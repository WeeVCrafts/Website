package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

type orderFactReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func readOrderFinancial(ctx context.Context, q orderFactReader, order, seller int64) (f ports.OrderFinancialSummary, err error) {
	var payment, funding string
	var amount int64
	var review bool
	err = q.QueryRow(ctx, `SELECT o.total_cents,o.credit_cents,o.payment_review_required,o.funding_status,COALESCE(p.status,'pending'),COALESCE(p.amount_cents,0),COALESCE(p.refunded_amount_cents,0),
 COALESCE((SELECT sum(amount_cents+shipping_cents-credit_cents) FROM marketplace_refunds WHERE order_id=o.id AND status NOT IN ('processed','rejected') AND ($2=0 OR seller_id=$2)),0)+CASE WHEN $2=0 THEN COALESCE((SELECT sum(amount_cents) FROM payment_refunds WHERE order_id=o.id AND status IN ('pending','processing')),0) ELSE 0 END,
 COALESCE((SELECT sum(credit_cents) FROM marketplace_refunds WHERE order_id=o.id AND status='processed' AND ($2=0 OR seller_id=$2)),0)
 FROM orders o LEFT JOIN LATERAL(SELECT status,amount_cents,refunded_amount_cents FROM payment_attempts WHERE order_id=o.id ORDER BY id DESC LIMIT 1)p ON true WHERE o.id=$1`, order, seller).Scan(&f.OriginalTotalCents, &f.CreditAppliedCents, &review, &funding, &payment, &amount, &f.ProviderRefundedCents, &f.PendingRefundCents, &f.CreditRestoredCents)
	if err != nil {
		return f, err
	}
	f.DisplayStatus = payment
	if payment == "captured" || payment == "refunded" {
		f.CapturedCents = amount
	}
	if payment == "refunded" && f.ProviderRefundedCents == 0 {
		f.ProviderRefundedCents = amount
	}
	if amount == 0 && funding == "funded" {
		f.DisplayStatus = "funded"
	}
	returned := f.ProviderRefundedCents + f.CreditRestoredCents
	if returned > 0 {
		f.DisplayStatus = "partially_refunded"
		if returned >= f.OriginalTotalCents {
			f.DisplayStatus = "refunded"
		}
	}
	if review {
		f.DisplayStatus = "under_review"
	}
	f.NetProviderPaidCents = f.CapturedCents - f.ProviderRefundedCents
	return f, nil
}

func readOrderItems(ctx context.Context, q orderFactReader, order, seller int64) (list []ports.OrderItemProgress, err error) {
	rows, err := q.Query(ctx, `SELECT i.id,COALESCE(i.seller_id,0),i.product_name,i.sku,COALESCE(s.display_name,'Independent maker'),i.quantity,i.cancelled_quantity,i.refunded_quantity,i.line_total_cents,i.line_total_cents-COALESCE(i.allocated_discount_cents,0),i.unit_price_cents FROM order_items i LEFT JOIN sellers s ON s.id=i.seller_id WHERE order_id=$1 AND ($2=0 OR i.seller_id=$2) ORDER BY i.id`, order, seller)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var x ports.OrderItemProgress
		if err = rows.Scan(&x.ID, &x.SellerID, &x.Name, &x.SKU, &x.Seller, &x.Ordered, &x.Cancelled, &x.Refunded, &x.OriginalCents, &x.PaidCents, &x.UnitPriceCents); err != nil {
			return nil, err
		}
		list = append(list, x)
	}
	return list, rows.Err()
}
