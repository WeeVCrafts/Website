package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *MarketplaceRepository) IssueInvoices(ctx context.Context, order int64) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var number, currency string
		var address domain.AddressInput
		var placed time.Time
		if err := tx.QueryRow(ctx, `SELECT order_number,currency,address_snapshot,created_at FROM orders WHERE id=$1 FOR UPDATE`, order).Scan(&number, &currency, &address, &placed); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT DISTINCT seller_id FROM order_items WHERE order_id=$1 AND seller_id IS NOT NULL ORDER BY seller_id`, order)
		if err != nil {
			return err
		}
		var sellers []int64
		for rows.Next() {
			var id int64
			if err = rows.Scan(&id); err != nil {
				break
			}
			sellers = append(sellers, id)
		}
		err = errorsJoinRows(err, rows)
		if err != nil {
			return err
		}
		for _, seller := range sellers {
			var exists bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_invoices WHERE order_id=$1 AND seller_id=$2)`, order, seller).Scan(&exists); err != nil {
				return err
			}
			if exists {
				continue
			}
			d := ports.InvoiceDocument{Currency: currency, Date: time.Now().In(time.FixedZone("IST", 19800)).Format("2006-01-02"), CustomerName: address.RecipientName, CustomerAddress: strings.Join([]string{address.Line1, address.Line2, address.City, address.State, address.PostalCode, address.CountryCode}, ", ")}
			d.CustomerStateCode = domain.IndianStateCode(address.State)
			if d.CustomerStateCode == "" {
				return ports.ErrInvalidState
			}
			var next int64
			if err = tx.QueryRow(ctx, `SELECT seller_id,legal_name,address,state_code,gstin,registration,prefix,next_number FROM seller_invoice_profiles WHERE seller_id=$1 FOR UPDATE`, seller).Scan(&d.Seller.SellerID, &d.Seller.LegalName, &d.Seller.Address, &d.Seller.StateCode, &d.Seller.GSTIN, &d.Seller.Registration, &d.Seller.Prefix, &next); err != nil {
				return fmt.Errorf("verified seller invoice profile required: %w", err)
			}
			d.Number = fmt.Sprintf("%s/%s/%06d", d.Seller.Prefix, time.Now().In(time.FixedZone("IST", 19800)).Format("06"), next)
			if len(d.Number) > 16 {
				return ports.ErrInvalidState
			}
			d.Kind = "Invoice"
			if d.Seller.Registration == "registered" {
				d.Kind = "Tax invoice"
			}
			lines, err := tx.Query(ctx, `SELECT id,product_name,sku,quantity,line_total_cents-COALESCE(allocated_discount_cents,0),tax_snapshot FROM order_items WHERE order_id=$1 AND seller_id=$2 ORDER BY id`, order, seller)
			if err != nil {
				return err
			}
			for lines.Next() {
				var line ports.InvoiceLine
				var snapshot []byte
				if err = lines.Scan(&line.ItemID, &line.Name, &line.SKU, &line.Quantity, &line.PaidCents, &snapshot); err != nil {
					break
				}
				var tax struct {
					HSN  string `json:"hsn"`
					Rate int64  `json:"rate_basis_points"`
				}
				if json.Unmarshal(snapshot, &tax) != nil || tax.HSN == "" {
					err = ports.ErrInvalidState
					break
				}
				line.HSN, line.RateBasisPoints = tax.HSN, tax.Rate
				if d.Seller.Registration == "unregistered" && tax.Rate != 0 {
					err = ports.ErrInvalidState
					break
				}
				line.TaxableCents = line.PaidCents * 10000 / (10000 + tax.Rate)
				line.TaxCents = line.PaidCents - line.TaxableCents
				d.TotalCents += line.PaidCents
				d.TaxCents += line.TaxCents
				d.TaxableCents += line.TaxableCents
				d.Lines = append(d.Lines, line)
			}
			err = errorsJoinRows(err, lines)
			if err != nil {
				return err
			}
			if d.Seller.StateCode == d.CustomerStateCode {
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
			if _, err = tx.Exec(ctx, `INSERT INTO marketplace_invoices(order_id,seller_id,number,snapshot,pdf_bytes,pdf_sha256) VALUES($1,$2,$3,$4,$5,$6)`, order, seller, d.Number, snapshot, pdf, fmt.Sprintf("%x", hash)); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE seller_invoice_profiles SET next_number=next_number+1 WHERE seller_id=$1`, seller); err != nil {
				return err
			}
		}
		return nil
	})
}
func errorsJoinRows(err error, rows pgx.Rows) error {
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	return err
}

func (r *MarketplaceRepository) SaveInvoiceProfile(ctx context.Context, actor int64, p ports.SellerInvoiceProfile, reason string) error {
	if len(strings.TrimSpace(reason)) < 4 || len(p.Prefix) > 6 || !domain.ValidIndianStateCode(p.StateCode) {
		return ports.ErrInvalidState
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		a, err := marketplaceIdentity(ctx, tx, actor)
		if err != nil {
			return err
		}
		if a.role != "super_admin" {
			return ports.ErrForbidden
		}
		_, err = tx.Exec(ctx, `INSERT INTO seller_invoice_profiles(seller_id,legal_name,address,state_code,gstin,registration,prefix,verified_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(seller_id) DO UPDATE SET legal_name=EXCLUDED.legal_name,address=EXCLUDED.address,state_code=EXCLUDED.state_code,gstin=EXCLUDED.gstin,registration=EXCLUDED.registration,verified_by=EXCLUDED.verified_by,verified_at=now()`, p.SellerID, p.LegalName, p.Address, p.StateCode, p.GSTIN, p.Registration, p.Prefix, actor)
		if err != nil {
			return err
		}
		return marketplaceAudit(ctx, tx, actor, 0, "invoice_profile_verified", reason)
	})
}
func (r *MarketplaceRepository) SaveTaxProfile(ctx context.Context, actor, variant int64, hsn string, rate int, reason string) error {
	if len(strings.TrimSpace(reason)) < 4 || variant <= 0 || rate < 0 || rate > 10000 {
		return ports.ErrInvalidState
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		a, err := marketplaceIdentity(ctx, tx, actor)
		if err != nil {
			return err
		}
		if a.role != "super_admin" {
			return ports.ErrForbidden
		}
		_, err = tx.Exec(ctx, `INSERT INTO variant_tax_profiles(variant_id,hsn,rate_basis_points,verified_by) VALUES($1,$2,$3,$4) ON CONFLICT(variant_id) DO UPDATE SET hsn=EXCLUDED.hsn,rate_basis_points=EXCLUDED.rate_basis_points,verified_by=EXCLUDED.verified_by,verified_at=now()`, variant, hsn, rate, actor)
		if err != nil {
			return err
		}
		return marketplaceAudit(ctx, tx, actor, 0, "product_tax_verified", reason)
	})
}
