package postgres

import (
	"context"
	"errors"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

func TestCommerceReadScopesAndPartialRefunds(t *testing.T) {
	x := newMarketplaceFixture(t)
	x.capture(t)
	ctx := context.Background()
	if _, err := x.pool.Exec(ctx, `UPDATE order_items SET cancelled_quantity=1,refunded_quantity=1 WHERE order_id=$1`, x.order.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(ctx, `UPDATE payment_attempts SET refunded_amount_cents=125 WHERE order_id=$1`, x.order.ID); err != nil {
		t.Fatal(err)
	}
	r := NewCommerceReadRepository(x.pool)
	for _, actor := range []int64{x.f.userIDs[0], x.f.ownerID, x.admin} {
		v, err := r.OrderForActor(ctx, actor, x.order.OrderNumber)
		if err != nil {
			t.Fatal(err)
		}
		if len(v.Items) != 1 || v.Items[0].Cancelled != 1 || v.Items[0].Refunded != 1 || v.Financial.DisplayStatus != "partially_refunded" {
			t.Fatalf("inconsistent order projection: %+v", v)
		}
		if v.Role != "seller" && v.Financial.NetProviderPaidCents != v.Financial.CapturedCents-125 {
			t.Fatal("net provider amount must use confirmed refund")
		}
	}
	if _, err := r.OrderForActor(ctx, x.f.userIDs[1], x.order.OrderNumber); !errors.Is(err, ports.ErrOrderNotFound) {
		t.Fatalf("unrelated customer: %v", err)
	}
}

func TestCommerceRequestSummaryIncludesModernRecords(t *testing.T) {
	x := newMarketplaceFixture(t)
	x.capture(t)
	ctx := context.Background()
	if _, err := x.pool.Exec(ctx, `INSERT INTO marketplace_returns(order_id,seller_id,order_item_id,quantity,status,reason,request_key) SELECT order_id,seller_id,id,1,'requested','Review damaged item','read-return-'||id FROM order_items WHERE order_id=$1`, x.order.ID); err != nil {
		t.Fatal(err)
	}
	r := NewCommerceReadRepository(x.pool)
	for _, actor := range []int64{x.f.userIDs[0], x.f.ownerID, x.admin} {
		p, err := r.RequestsForActor(ctx, actor, ports.RequestQuery{OrderNumber: x.order.OrderNumber})
		if err != nil {
			t.Fatal(err)
		}
		if p.Total != 1 || len(p.Rows) != 1 || p.Rows[0].Kind != "return" || p.Counts["needs_review"] != 1 {
			t.Fatalf("missing request: %+v", p)
		}
	}
	p, err := r.RequestsForActor(ctx, x.f.userIDs[1], ports.RequestQuery{OrderNumber: x.order.OrderNumber})
	if err != nil || p.Total != 0 {
		t.Fatalf("unrelated customer request scope: %+v %v", p, err)
	}
}

func TestCommerceHistoricalOrderKeepsPaymentAndDeliveryFacts(t *testing.T) {
	x := newMarketplaceFixture(t)
	x.capture(t)
	ctx := context.Background()
	if _, err := x.pool.Exec(ctx, `UPDATE orders SET workflow_version=1,status='processing',lifecycle_status='awaiting_payment' WHERE id=$1`, x.order.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(ctx, `UPDATE seller_fulfillments SET status='shipped',carrier='Historical carrier',tracking_number='HISTORICAL-TRACK' WHERE order_id=$1`, x.order.ID); err != nil {
		t.Fatal(err)
	}
	summary, view, _, err := x.repo.OrderWorkspace(ctx, x.f.userIDs[0], x.order.OrderNumber, ports.RequestQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != "processing" || summary.Financial.DisplayStatus != "captured" || len(summary.LegacyShipments) != 1 || summary.LegacyShipments[0].TrackingNumber != "HISTORICAL-TRACK" || view.WorkflowVersion != 1 {
		t.Fatalf("historical facts lost: %+v", summary)
	}
	detail, err := x.commerce.GetOrder(ctx, x.f.userIDs[0], x.order.OrderNumber)
	if err != nil {
		t.Fatal(err)
	}
	if detail.ProcessingStatus != "" || len(detail.Fulfillments) != 1 || detail.Fulfillments[0].Status != "shipped" {
		t.Fatalf("historical compatibility changed: %+v", detail)
	}
}
