package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

type marketplaceEmailRecorder struct {
	messages []ports.EmailMessage
}

func (r *marketplaceEmailRecorder) Send(_ context.Context, message ports.EmailMessage) error {
	r.messages = append(r.messages, message)
	return nil
}

func TestMarketplaceCancellationPickupEvidenceAndBuyerUpdates(t *testing.T) {
	x := newMarketplaceFixture(t)
	x.capture(t)
	ctx := t.Context()
	if err := x.repo.Act(ctx, x.f.ownerID, x.command("accept")); err != nil {
		t.Fatal(err)
	}
	pack := x.command("pack")
	pack.Revision = 1
	if err := x.repo.Act(ctx, x.f.ownerID, pack); err != nil {
		t.Fatal(err)
	}
	order, err := x.repo.View(ctx, x.f.ownerID, x.order.OrderNumber)
	if err != nil {
		t.Fatal(err)
	}
	item := order.Lines[0].ID
	for i, quantity := range []int{1, 2} {
		cmd := x.command("package")
		cmd.Key = fmt.Sprintf("audit-package-%d-%d", x.order.ID, i)
		cmd.Parcel = ports.Parcel{WeightGrams: 100, LengthCM: 10, WidthCM: 10, HeightCM: 10}
		cmd.Items = map[int64]int{item: quantity}
		if err = x.repo.Act(ctx, x.f.ownerID, cmd); err != nil {
			t.Fatal(err)
		}
	}
	order, err = x.repo.View(ctx, x.f.ownerID, x.order.OrderNumber)
	if err != nil || len(order.Packages) != 2 {
		t.Fatalf("load packages: %v, %v", order.Packages, err)
	}
	for i, pkg := range order.Packages {
		s, err := x.repo.ShippingOrder(ctx, pkg.ID)
		if err != nil {
			t.Fatal(err)
		}
		s.ProviderOrderID = 800000 + pkg.ID
		s.ProviderShipmentID = 900000 + pkg.ID
		s.ProviderStatus = "PICKUP SCHEDULED"
		s.PickupReference = "shared-pickup"
		s.LabelURL = "https://shiprocket.in/test-label.pdf"
		s.AWB = fmt.Sprintf("audit-awb-%d", i)
		if err = x.repo.SavePackage(ctx, pkg.ID, s.Shipment, ""); err != nil {
			t.Fatal(err)
		}
	}
	first, second := order.Packages[0].ID, order.Packages[1].ID
	var packedNotice int64
	if err = x.pool.QueryRow(ctx, `SELECT id FROM notifications WHERE marketplace_package_id=$1 AND title='Package packed'`, first).Scan(&packedNotice); err != nil {
		t.Fatal(err)
	}
	email := &marketplaceEmailRecorder{}
	if err = x.repo.SendNotificationEmail(ctx, packedNotice, email, "https://shop.example"); err != nil ||
		len(email.messages) != 1 || strings.ContainsAny(email.messages[0].Body, "₹") || !strings.HasSuffix(email.messages[0].ActionURL, "?tab=packages") {
		t.Fatalf("packed email: %v, %v", email.messages, err)
	}
	cancel := x.command("cancel")
	cancel.ItemID, cancel.Quantity = item, 1
	cancel.Key = fmt.Sprintf("audit-cancellation-%d", x.order.ID)
	cancel.Reason = "Customer requested a partial cancellation"
	if err = x.repo.Act(ctx, x.f.userIDs[0], cancel); err != nil {
		t.Fatal(err)
	}
	var cancellationID, jobID int64
	if err = x.pool.QueryRow(ctx, `SELECT c.id,j.id FROM marketplace_cancellations c JOIN marketplace_jobs j ON j.resource_id=c.id AND j.kind='cancel_items' WHERE request_key=$1`, cancel.Key).Scan(&cancellationID, &jobID); err != nil {
		t.Fatal(err)
	}
	job, claimed, err := x.repo.ClaimJob(ctx, jobID)
	if err != nil || !claimed {
		t.Fatalf("claim cancellation: %t, %v", claimed, err)
	}
	if err = x.repo.FinishJob(ctx, job, ports.ErrShippingPending); err != nil {
		t.Fatal(err)
	}
	order, err = x.repo.View(ctx, x.f.ownerID, x.order.OrderNumber)
	if err != nil || !order.Packages[0].CancellationRequested || !order.Packages[0].CancellationNeedsReview {
		t.Fatalf("pending cancellation and review were not projected: %v, %v", order.Packages, err)
	}
	if err = x.repo.SendNotificationEmail(ctx, packedNotice, email, "https://shop.example"); err != nil || len(email.messages) != 1 {
		t.Fatalf("packed email sent after cancellation request: %v, %v", email.messages, err)
	}
	markCancelled := func(id int64) {
		t.Helper()
		s, err := x.repo.ShippingOrder(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		s.ProviderStatus = "CANCELED"
		if err = x.repo.SavePackage(ctx, id, s.Shipment, ""); err != nil {
			t.Fatal(err)
		}
	}
	markCancelled(first)
	if err = x.repo.CompleteCancellation(ctx, cancellationID); !errors.Is(err, ports.ErrInvalidState) {
		t.Fatalf("completed with a live provider package: %v", err)
	}
	markCancelled(second)
	var pickupCase int64
	if err = x.pool.QueryRow(ctx, `SELECT id FROM marketplace_cases WHERE package_id=$1 AND kind='pickup_clearance'`, first).Scan(&pickupCase); err != nil {
		t.Fatal(err)
	}
	clear := x.command("resolve_case")
	clear.CaseID, clear.Reason = pickupCase, "Carrier confirmed this AWB was removed from pickup."
	if err = x.repo.Act(ctx, x.f.ownerID, clear); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("seller could confirm clearance: %v", err)
	}
	if err = x.repo.Act(ctx, x.admin, clear); err != nil {
		t.Fatal(err)
	}
	order, err = x.repo.View(ctx, x.f.ownerID, x.order.OrderNumber)
	if err != nil || !order.Packages[0].PickupCleared || order.Packages[1].PickupCleared {
		t.Fatalf("clearance leaked across the shared pickup: %v, %v", order.Packages, err)
	}
	for i := 0; i < 2; i++ {
		if err = x.repo.CompleteCancellation(ctx, cancellationID); err != nil {
			t.Fatal(err)
		}
	}
	var requested, cancelled, pending, refunds int
	if err = x.pool.QueryRow(ctx, `SELECT
		count(*) FILTER (WHERE title='Cancellation requested'),
		count(*) FILTER (WHERE title='Items cancelled'),
		count(*) FILTER (WHERE title='Refund pending' AND body LIKE '%₹10.00%'),
		(SELECT count(*) FROM marketplace_refunds WHERE order_id=$2)
		FROM notifications WHERE user_id=$1 AND link_url=$3`,
		x.f.userIDs[0], x.order.ID, "/orders/"+x.order.OrderNumber+"?tab=returns").Scan(&requested, &cancelled, &pending, &refunds); err != nil {
		t.Fatal(err)
	}
	if requested != 1 || cancelled != 1 || pending != 1 || refunds != 1 {
		t.Fatalf("unexpected event counts: requested=%d, cancelled=%d, pending=%d, refunds=%d", requested, cancelled, pending, refunds)
	}
}
