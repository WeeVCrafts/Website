package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

var reconciliationIST = time.FixedZone("IST", 5*60*60+30*60)

func (h *LivePortalHandler) dailyReconciliation(r *http.Request, root string) (*viewmodels.FinanceReconciliation, error) {
	reportDate := r.URL.Query().Get("date")
	if reportDate == "" {
		reportDate = time.Now().In(reconciliationIST).Format(time.DateOnly)
	}
	start, err := time.ParseInLocation(time.DateOnly, reportDate, reconciliationIST)
	if err != nil || start.Year() < 2000 || start.Year() > 2100 {
		return nil, errors.New("invalid reconciliation date")
	}
	report := &viewmodels.FinanceReconciliation{Date: reportDate}
	var captured, refunds, shipping, commission, reversals, payout int64
	err = h.queryRow(r, `
		WITH paid_orders AS (
			SELECT o.id,o.shipping_cents,MIN(h.created_at) confirmed_at
			FROM orders o JOIN order_status_history h ON h.order_id=o.id AND h.to_status='paid'
			WHERE o.currency='INR' GROUP BY o.id
		), paid_refunds AS (
			SELECT provider_refund_id,amount_cents,updated_at FROM payment_refunds WHERE status='processed' AND currency='INR'
			UNION ALL
			SELECT r.provider_refund_id,r.amount_cents,r.updated_at FROM marketplace_refunds r JOIN orders o ON o.id=r.order_id
				WHERE r.status='processed' AND o.currency='INR'
			UNION ALL
			SELECT provider_refund_id,amount_cents,updated_at FROM payment_review_cases WHERE status='refunded' AND currency='INR'
		), confirmed_refunds AS (
			SELECT DISTINCT ON (provider_refund_id) amount_cents,updated_at FROM paid_refunds
			WHERE COALESCE(provider_refund_id,'')<>'' ORDER BY provider_refund_id,updated_at,amount_cents
		)
		SELECT
			COALESCE((SELECT SUM(p.amount_cents) FROM paid_orders o JOIN payment_attempts p ON p.order_id=o.id
				WHERE p.provider='razorpay' AND p.status IN ('captured','refunded') AND COALESCE(p.provider_payment_id,'')<>''
					AND o.confirmed_at >= $1 AND o.confirmed_at < $2),0),
			COALESCE((SELECT SUM(amount_cents) FROM confirmed_refunds WHERE updated_at >= $1 AND updated_at < $2),0),
			COALESCE((SELECT SUM(shipping_cents) FROM paid_orders WHERE confirmed_at >= $1 AND confirmed_at < $2),0),
			COALESCE((SELECT SUM(commission_cents) FROM seller_ledger WHERE commission_cents>0 AND created_at >= $1 AND created_at < $2),0),
			COALESCE((SELECT SUM(-commission_cents) FROM seller_ledger WHERE commission_cents<0 AND created_at >= $1 AND created_at < $2),0),
			COALESCE((SELECT SUM(transfer_amount_cents) FROM seller_settlements WHERE status='paid'
				AND transfer_evidence_key IS NOT NULL AND paid_at >= $1 AND paid_at < $2),0)
	`, start, start.AddDate(0, 0, 1)).Scan(&captured, &refunds, &shipping, &commission, &reversals, &payout)
	if err != nil {
		return nil, err
	}
	report.Totals = []viewmodels.AdminStat{
		{Label: "Razorpay captured for confirmed orders", Value: storefrontMoney(captured)},
		{Label: "Provider-confirmed refunds", Value: storefrontMoney(refunds)},
		{Label: "Customer shipping charges", Value: storefrontMoney(shipping)},
		{Label: "Commissions recorded", Value: storefrontMoney(commission)},
		{Label: "Commission reversals", Value: storefrontMoney(reversals)},
		{Label: "Bank-confirmed payouts", Value: storefrontMoney(payout)},
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT o.order_number,'Payment review',c.amount_cents,c.status,c.created_at
		FROM payment_review_cases c JOIN orders o ON o.id=c.order_id
		WHERE c.status<>'refunded' AND c.currency='INR'
		UNION ALL
		SELECT o.order_number,'Refund follow-up',f.amount_cents,f.status,f.created_at
		FROM marketplace_refunds f JOIN orders o ON o.id=f.order_id
		WHERE f.status NOT IN ('processed','rejected') AND o.currency='INR'
		ORDER BY 5,1 LIMIT 51`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item viewmodels.FinanceFollowUp
		var amount int64
		var created time.Time
		if err := rows.Scan(&item.Order, &item.Kind, &amount, &item.Status, &created); err != nil {
			return nil, err
		}
		item.Amount = storefrontMoney(amount)
		item.URL = root + "/fulfillment?order=" + url.QueryEscape(item.Order)
		if item.Kind == "Payment review" {
			item.URL = root + "/payment-reviews"
		}
		report.FollowUps = append(report.FollowUps, item)
	}
	report.HasMore = len(report.FollowUps) > 50
	if report.HasMore {
		report.FollowUps = report.FollowUps[:50]
	}
	return report, rows.Err()
}
