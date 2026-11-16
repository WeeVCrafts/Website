package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
	"time"
)

func (r *MarketplaceRepository) IssueCreditNote(ctx context.Context, refund int64) error {
	return r.issueCreditDocument(ctx, refund, false)
}
func (r *MarketplaceRepository) IssueClaimCreditNote(ctx context.Context, claim int64) error {
	return r.issueCreditDocument(ctx, claim, true)
}
func (r *MarketplaceRepository) issueCreditDocument(ctx context.Context, refund int64, claim bool) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var order, seller, item, amount int64
		var quantity int
		var original []byte
		query := `SELECT mr.order_id,mr.seller_id,mr.order_item_id,mr.amount_cents,mr.quantity,i.snapshot FROM marketplace_refunds mr JOIN marketplace_invoices i ON i.order_id=mr.order_id AND i.seller_id=mr.seller_id WHERE mr.id=$1 AND mr.status='processed'`
		column := "refund_id"
		if claim {
			query = `SELECT c.order_id,c.seller_id,c.order_item_id,c.amount_cents,c.quantity,i.snapshot FROM marketplace_claims c JOIN marketplace_invoices i ON i.order_id=c.order_id AND i.seller_id=c.seller_id WHERE c.id=$1 AND c.status='completed' AND c.resolution='credit'`
			column = "claim_id"
		}
		if err := tx.QueryRow(ctx, query, refund).Scan(&order, &seller, &item, &amount, &quantity, &original); err != nil {
			return err
		}

		var next int64
		if err := tx.QueryRow(ctx, `SELECT next_credit_number FROM seller_invoice_profiles WHERE seller_id=$1 FOR UPDATE`, seller).Scan(&next); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_credit_notes WHERE `+column+`=$1)`, refund).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return nil
		}
		var d ports.InvoiceDocument
		if json.Unmarshal(original, &d) != nil {
			return ports.ErrInvalidState
		}
		var line ports.InvoiceLine
		for _, candidate := range d.Lines {
			if candidate.ItemID == item {
				line = candidate
				break
			}
		}
		if line.ItemID == 0 {
			return ports.ErrInvalidState
		}
		d.RelatedNumber = d.Number
		d.Number = fmt.Sprintf("CN/%s/%06d", time.Now().In(time.FixedZone("IST", 19800)).Format("06"), next)
		d.Kind = "Credit note"
		d.Date = time.Now().In(time.FixedZone("IST", 19800)).Format("2006-01-02")
		line.Quantity = quantity
		var prior int64
		if err := tx.QueryRow(ctx, `SELECT COALESCE(sum((snapshot->>'TotalCents')::bigint),0) FROM marketplace_credit_notes WHERE order_id=$1 AND seller_id=$2 AND (snapshot->'Lines'->0->>'ItemID')::bigint=$3`, order, seller, item).Scan(&prior); err != nil {
			return err
		}

		originalPaid, originalTax := line.PaidCents, line.TaxCents
		if originalPaid <= 0 || prior+amount > originalPaid {
			return ports.ErrRefundState
		}
		line.PaidCents = amount
		line.TaxCents = originalTax*(prior+amount)/originalPaid - originalTax*prior/originalPaid
		line.TaxableCents = amount - line.TaxCents
		d.Lines = []ports.InvoiceLine{line}
		d.TotalCents = amount
		d.TaxableCents = line.TaxableCents
		d.TaxCents = line.TaxCents
		d.CGSTCents = 0
		d.SGSTCents = 0
		d.IGSTCents = 0
		if d.CustomerStateCode == d.Seller.StateCode {
			d.CGSTCents = d.TaxCents / 2
			d.SGSTCents = d.TaxCents - d.CGSTCents
		} else {
			d.IGSTCents = d.TaxCents
		}
		snapshot, err := json.Marshal(d)
		if err != nil {
			return err
		}
		if r.renderInvoice == nil {
			return ports.ErrInvalidState
		}
		pdf, err := r.renderInvoice(d)
		if err != nil {
			return err
		}
		if len(pdf) < 4 || len(pdf) > 3<<20 || string(pdf[:4]) != "%PDF" {
			return ports.ErrInvalidState
		}
		hash := sha256.Sum256(pdf)
		if _, err = tx.Exec(ctx, `INSERT INTO marketplace_credit_notes(`+column+`,order_id,seller_id,number,snapshot,pdf_bytes,pdf_sha256) VALUES($1,$2,$3,$4,$5,$6,$7)`, refund, order, seller, d.Number, snapshot, pdf, fmt.Sprintf("%x", hash)); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE seller_invoice_profiles SET next_credit_number=next_credit_number+1 WHERE seller_id=$1`, seller)
		return err
	})
}
