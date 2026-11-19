package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"net/url"
)

func (r *CommerceRepository) loadOrderWorkspaceMetadata(ctx context.Context, actor int64, d *domain.OrderDetail) error {
	rows, err := r.pool.Query(ctx, `SELECT p.id,p.seller_id,p.reference,p.status,p.courier,p.awb,COALESCE((SELECT jsonb_agg(jsonb_build_object('name',i.product_name,'sku',i.sku,'quantity',pi.quantity) ORDER BY i.id) FROM marketplace_package_items pi JOIN order_items i ON i.id=pi.order_item_id WHERE pi.package_id=p.id),'[]') FROM marketplace_packages p WHERE p.order_id=$1 ORDER BY p.id LIMIT 101`, d.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var x domain.OrderPackageSummary
		var items []byte
		if err = rows.Scan(&x.ID, &x.SellerID, &x.Reference, &x.Status, &x.Courier, &x.AWB, &items); err != nil {
			break
		}
		if err = json.Unmarshal(items, &x.Items); err != nil {
			break
		}
		d.Packages = append(d.Packages, x)
	}
	if err = errorsJoinRows(err, rows); err != nil {
		return err
	}
	if len(d.Packages) > 100 {
		d.HasMorePackages = true
		d.Packages = d.Packages[:100]
	}
	rows, err = r.pool.Query(ctx, `SELECT id,seller_id,number,'invoice' FROM marketplace_invoices WHERE order_id=$1 UNION ALL SELECT id,seller_id,number,'credit_note' FROM marketplace_credit_notes WHERE order_id=$1 ORDER BY 1 LIMIT 101`, d.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var x domain.OrderDocumentReference
		if err = rows.Scan(&x.ID, &x.SellerID, &x.Number, &x.Kind); err != nil {
			break
		}
		x.URL = fmt.Sprintf("/orders/%s/fulfillment?invoice=%d", url.PathEscape(d.OrderNumber), x.ID)
		if x.Kind == "credit_note" {
			x.URL += "&kind=credit"
		}
		d.Documents = append(d.Documents, x)
	}
	if err = errorsJoinRows(err, rows); err != nil {
		return err
	}
	if len(d.Documents) > 100 {
		d.HasMoreDocuments = true
		d.Documents = d.Documents[:100]
	}
	requests, err := NewCommerceReadRepository(r.pool).RequestsForActor(ctx, actor, ports.RequestQuery{OrderNumber: d.OrderNumber})
	if err != nil {
		return err
	}
	for _, x := range requests.Rows {
		d.Requests = append(d.Requests, domain.OrderRequestSummary{ID: x.ID, Kind: x.Kind, Status: x.Status, AmountCents: x.AmountCents})
	}
	d.HasMoreRequests = requests.Total > len(requests.Rows)
	return nil
}
