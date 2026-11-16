package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wecratfs/commerce/internal/adapters/shiprocket"
	application "github.com/wecratfs/commerce/internal/application/commerce"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

type marketplaceFixture struct {
	pool          *pgxpool.Pool
	repo          *MarketplaceRepository
	commerce      *CommerceRepository
	f             commerceFixture
	order         domain.Order
	seller, admin int64
}

func newMarketplaceFixture(t *testing.T) marketplaceFixture {
	t.Helper()
	p := openIntegrationPool(t)
	return newMarketplaceFixtureWithPool(t, p)
}

func newMarketplaceFixtureWithPool(t *testing.T, p *pgxpool.Pool) marketplaceFixture {
	t.Helper()
	f := newCommerceFixture(t, p, 2, 10)
	ctx := context.Background()
	x := marketplaceFixture{pool: p, repo: NewMarketplaceRepository(p).WithInvoiceRenderer(func(ports.InvoiceDocument) ([]byte, error) { return []byte("%PDF-1.7 test document"), nil }), commerce: NewCommerceRepository(p), f: f}
	if err := p.QueryRow(ctx, `SELECT id FROM sellers WHERE owner_user_id=$1`, f.ownerID).Scan(&x.seller); err != nil {
		t.Fatal(err)
	}
	for _, uid := range f.userIDs {
		if _, err := p.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'customer') ON CONFLICT DO NOTHING`, uid); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := p.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'seller') ON CONFLICT DO NOTHING`, f.ownerID); err != nil {
		t.Fatal(err)
	}
	if err := p.QueryRow(ctx, `INSERT INTO users(email,display_name,status) VALUES($1,'Marketplace operator','active') RETURNING id`, fmt.Sprintf("marketplace-admin-%d@example.invalid", time.Now().UnixNano())).Scan(&x.admin); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'super_admin')`, x.admin); err != nil {
		t.Fatal(err)
	}
	if err := x.repo.SaveInvoiceProfile(ctx, x.admin, ports.SellerInvoiceProfile{SellerID: x.seller, LegalName: "Verified craft business", Address: "Verified test depot, Pune 411001", StateCode: "27", Registration: "unregistered", Prefix: "TEST"}, "Verified test merchant evidence"); err != nil {
		t.Fatal(err)
	}
	if err := x.repo.SaveTaxProfile(ctx, x.admin, f.variantID, "4420", 0, "Approved test classification"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `UPDATE cart_items SET quantity=3 WHERE cart_id=$1`, f.cartIDs[0]); err != nil {
		t.Fatal(err)
	}
	var err error
	x.order, err = x.commerce.CreateOrder(ctx, f.userIDs[0], f.cartIDs[0], fmt.Sprintf("marketplace-checkout-%d", time.Now().UnixNano()), domain.AddressInput{RecipientName: "Test buyer", Phone: "9000000000", Line1: "Test lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
	if err != nil {
		t.Fatal(err)
	}
	if err = x.repo.ConfigureShippingProfiles(ctx, map[int64]string{x.seller: "Fixture Depot"}, map[int64]string{x.seller: "411001"}); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(ctx, `INSERT INTO order_delivery_decisions(order_id,seller_id,decision,estimate) VALUES($1,$2,'shiprocket','{}')`, x.order.ID, x.seller); err != nil {
		t.Fatal(err)
	}
	return x
}

func TestMarketplaceLostBookingResponseDoesNotCreateAnotherShipment(t *testing.T) {
	x := newMarketplaceFixture(t)
	x.capture(t)
	ctx := context.Background()
	c := x.command("accept")
	if err := x.repo.Act(ctx, x.f.ownerID, c); err != nil {
		t.Fatal(err)
	}
	c.Action, c.Revision = "pack", 1
	if err := x.repo.Act(ctx, x.f.ownerID, c); err != nil {
		t.Fatal(err)
	}
	v, err := x.repo.View(ctx, x.f.ownerID, x.order.OrderNumber)
	if err != nil {
		t.Fatal(err)
	}
	c = x.command("package")
	c.Key = fmt.Sprintf("recover-package-%016d", x.order.ID)
	c.Parcel = ports.Parcel{WeightGrams: 500, LengthCM: 15, WidthCM: 12, HeightCM: 8}
	c.Items = map[int64]int{v.Lines[0].ID: 3}
	if err = x.repo.Act(ctx, x.f.ownerID, c); err != nil {
		t.Fatal(err)
	}
	c = x.command("ready")
	c.Revision = 2
	if err = x.repo.Act(ctx, x.f.ownerID, c); err != nil {
		t.Fatal(err)
	}
	var job int64
	if err = x.pool.QueryRow(ctx, `SELECT id FROM marketplace_jobs WHERE kind='book_package' AND resource_id=(SELECT id FROM marketplace_packages WHERE order_id=$1)`, x.order.ID).Scan(&job); err != nil {
		t.Fatal(err)
	}
	provider := &parcelProvider{providerOrderID: time.Now().UnixNano() / 1000 * 2}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { provider.serve(t, w, r) }))
	defer server.Close()
	processor := &application.MarketplaceProcessor{Store: x.repo, Shipping: shiprocket.New(server.URL, "test@example.invalid", "test-only"), BookingsEnabled: true}
	if err = processor.Process(ctx, job); err == nil {
		t.Fatal("uncertain response reported complete")
	}
	if _, err = x.pool.Exec(ctx, `UPDATE marketplace_jobs SET available_at=now() WHERE id=$1`, job); err != nil {
		t.Fatal(err)
	}
	if err = processor.Process(ctx, job); err != nil {
		t.Fatal(err)
	}
	if provider.created != 1 || provider.pickedUp != 1 || provider.labelled != 1 {
		t.Fatalf("side effects create=%d pickup=%d label=%d", provider.created, provider.pickedUp, provider.labelled)
	}
}

func TestMarketplaceDeliveryHoldAndExceptionReturn(t *testing.T) {
	x := newMarketplaceFixture(t)
	x.capture(t)
	ctx := context.Background()
	if _, err := x.pool.Exec(ctx, `UPDATE orders SET status='delivered',delivered_at=now()-interval '8 days' WHERE id=$1`, x.order.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(ctx, `UPDATE seller_fulfillments SET status='delivered',lifecycle_status='delivered',delivered_at=now()-interval '6 days' WHERE order_id=$1`, x.order.ID); err != nil {
		t.Fatal(err)
	}
	eligible := func() bool {
		var ok bool
		if err := x.pool.QueryRow(ctx, `SELECT marketplace_payout_eligible($1,$2)`, x.order.ID, x.seller).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if eligible() {
		t.Fatal("paid before seven day hold")
	}
	if _, err := x.pool.Exec(ctx, `UPDATE seller_fulfillments SET delivered_at=now()-interval '8 days' WHERE order_id=$1`, x.order.ID); err != nil {
		t.Fatal(err)
	}
	if !eligible() {
		t.Fatal("verified delivery did not mature")
	}
	checkSellerExit := func(blocked bool) {
		t.Helper()
		tx, err := x.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		err = guardSellerRoleExit(ctx, tx, x.f.ownerID)
		if blocked && !errors.Is(err, ports.ErrSellerPendingWork) || !blocked && err != nil {
			t.Fatalf("seller exit blocked=%v: %v", blocked, err)
		}
	}
	checkSellerExit(false)
	var supportCase int64
	if err := x.pool.QueryRow(ctx, `INSERT INTO marketplace_cases(order_id,kind,reason) VALUES($1,'delivery_dispute','Investigate proof of delivery') RETURNING id`, x.order.ID).Scan(&supportCase); err != nil {
		t.Fatal(err)
	}
	checkSellerExit(true)
	if _, err := x.pool.Exec(ctx, `UPDATE marketplace_cases SET status='resolved',resolution='Evidence checked',resolved_at=now() WHERE id=$1`, supportCase); err != nil {
		t.Fatal(err)
	}
	v, err := x.repo.View(ctx, x.f.userIDs[0], x.order.OrderNumber)
	if err != nil {
		t.Fatal(err)
	}
	c := x.command("request_return")
	c.ItemID = v.Lines[0].ID
	c.Quantity = 1
	c.Key = fmt.Sprintf("exception-return-%016d", x.order.ID)
	c.Reason = "Delivered item is damaged; investigate exception"
	if err = x.repo.Act(ctx, x.f.userIDs[0], c); err != nil {
		t.Fatal(err)
	}
	if eligible() {
		t.Fatal("open return did not hold payout")
	}
	checkSellerExit(true)
	var ret int64
	if err = x.pool.QueryRow(ctx, `SELECT id FROM marketplace_returns WHERE request_key=$1`, c.Key).Scan(&ret); err != nil {
		t.Fatal(err)
	}
	c.Action = "return_status"
	c.CaseID = ret
	c.Status = "completed"
	if err = x.repo.Act(ctx, x.admin, c); err == nil {
		t.Fatal("uninspected return completed")
	}
	for _, stage := range []string{"approved", "received", "inspected", "completed"} {
		c.Status = stage
		if err = x.repo.Act(ctx, x.admin, c); err != nil {
			t.Fatal(stage, err)
		}
	}
	if err = x.repo.Act(ctx, x.admin, c); err != nil {
		t.Fatal("completed return retry", err)
	}
	var count int
	if err = x.pool.QueryRow(ctx, `SELECT count(*) FROM marketplace_refunds WHERE order_id=$1`, x.order.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("return refund count=%d %v", count, err)
	}
	checkSellerExit(true)
}

func TestMarketplaceProductionCheckoutRequiresVerifiedInvoiceData(t *testing.T) {
	p := openIntegrationPool(t)
	f := newCommerceFixture(t, p, 1, 2)
	repo := NewCommerceRepository(p)
	repo.RequireInvoiceProfiles(true)
	_, err := repo.CreateOrder(context.Background(), f.userIDs[0], f.cartIDs[0], fmt.Sprintf("missing-invoice-%d", f.userIDs[0]), domain.AddressInput{RecipientName: "Test", Phone: "9000000000", Line1: "Test Lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
	if !errors.Is(err, ports.ErrInvalidState) {
		t.Fatalf("unverified invoice accepted: %v", err)
	}
	var orders, reserved int
	if err = p.QueryRow(context.Background(), `SELECT count(*) FROM orders WHERE user_id=$1`, f.userIDs[0]).Scan(&orders); err != nil {
		t.Fatal(err)
	}
	if err = p.QueryRow(context.Background(), `SELECT reserved_quantity FROM inventory_stock WHERE variant_id=$1`, f.variantID).Scan(&reserved); err != nil {
		t.Fatal(err)
	}
	if orders != 0 || reserved != 0 {
		t.Fatalf("failed checkout left order=%d reservation=%d", orders, reserved)
	}
}

func TestMarketplaceDatabaseEnforcesDeliveryHold(t *testing.T) {
	x := newMarketplaceFixture(t)
	x.capture(t)
	ctx := context.Background()
	var batch int64
	if err := x.pool.QueryRow(ctx, `INSERT INTO seller_settlements(seller_id,period_start,period_end,gross_cents,commission_cents,net_cents,status,reference) SELECT seller_id,current_date,current_date,gross_cents,commission_cents,net_cents,'pending',$2 FROM seller_ledger WHERE order_id=$1 AND entry_type='sale' RETURNING id`, x.order.ID, fmt.Sprintf("hold-fixture-%d", x.order.ID)).Scan(&batch); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(ctx, `UPDATE seller_ledger SET settlement_id=$1 WHERE order_id=$2`, batch, x.order.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(ctx, `UPDATE seller_settlements SET status='approved' WHERE id=$1`, batch); err == nil {
		t.Fatal("database permitted approval before delivery")
	}
	if _, err := x.pool.Exec(ctx, `UPDATE seller_fulfillments SET status='delivered',lifecycle_status='delivered',delivered_at=now()-interval '8 days' WHERE order_id=$1`, x.order.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(ctx, `UPDATE seller_settlements SET status='approved' WHERE id=$1`, batch); err != nil {
		t.Fatal("matured delivery blocked", err)
	}
}
func (x marketplaceFixture) capture(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	payments := NewPaymentRepository(x.pool)
	provider := fmt.Sprintf("mp-order-%d", x.order.ID)
	if _, err := payments.AttachProviderOrder(ctx, x.f.userIDs[0], x.order.OrderNumber, ports.PaymentOrder{ProviderOrderID: provider, AmountCents: x.order.TotalCents, Currency: x.order.Currency}); err != nil {
		t.Fatal(err)
	}
	if _, err := payments.ProcessWebhook(ctx, ports.PaymentWebhook{EventID: provider + "-event", EventType: "payment.captured", ProviderOrderID: provider, ProviderPaymentID: provider + "-payment", AmountCents: x.order.TotalCents, Currency: x.order.Currency}, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
}
func (x marketplaceFixture) command(action string) ports.MarketplaceCommand {
	return ports.MarketplaceCommand{Action: action, Number: x.order.OrderNumber, SellerID: x.seller}
}
func TestMarketplacePaidAcceptanceAndMultiplePackages(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := context.Background()
	c := x.command("accept")
	if err := x.repo.Act(ctx, x.f.ownerID, c); !errors.Is(err, ports.ErrInvalidState) {
		t.Fatalf("unpaid acceptance: %v", err)
	}
	x.capture(t)
	if err := x.repo.Act(ctx, x.f.ownerID, c); err != nil {
		t.Fatal(err)
	}
	c.Action, c.Revision = "pack", 1
	if err := x.repo.Act(ctx, x.f.ownerID, c); err != nil {
		t.Fatal(err)
	}
	v, err := x.repo.View(ctx, x.f.userIDs[0], x.order.OrderNumber)
	if err != nil {
		t.Fatal(err)
	}
	item := v.Lines[0].ID
	for i, qty := range []int{2, 1} {
		c = x.command("package")
		c.Key = fmt.Sprintf("package-%d-%016d", x.order.ID, i)
		c.Parcel = ports.Parcel{WeightGrams: 500, LengthCM: 15, WidthCM: 12, HeightCM: 8}
		c.Items = map[int64]int{item: qty}
		if err = x.repo.Act(ctx, x.f.ownerID, c); err != nil {
			t.Fatal(err)
		}
	}
	c.Key = "overallocate-quantity-1234"
	if err = x.repo.Act(ctx, x.f.ownerID, c); err == nil {
		t.Fatal("overallocated package")
	}
	if _, err = x.repo.View(ctx, x.f.userIDs[1], x.order.OrderNumber); !errors.Is(err, ports.ErrOrderNotFound) {
		t.Fatalf("cross-customer order: %v", err)
	}
	c = x.command("ready")
	c.Revision = 2
	if err = x.repo.Act(ctx, x.f.ownerID, c); err != nil {
		t.Fatal(err)
	}
	v, err = x.repo.View(ctx, x.f.ownerID, x.order.OrderNumber)
	if err != nil || len(v.Packages) != 2 {
		t.Fatalf("packages %+v %v", v.Packages, err)
	}
	for i, p := range v.Packages {
		s, e := x.repo.ShippingOrder(ctx, p.ID)
		if e != nil {
			t.Fatal(e)
		}
		s.ProviderOrderID = 100000 + p.ID
		s.ProviderShipmentID = 200000 + p.ID
		s.ProviderStatus = "DELIVERED"
		s.AWB = fmt.Sprint(p.ID)
		s.Courier = "Verified carrier"
		if err = x.repo.SavePackage(ctx, p.ID, s.Shipment, ""); err != nil {
			t.Fatal(err)
		}
		got, e := x.repo.View(ctx, x.f.userIDs[0], x.order.OrderNumber)
		if e != nil {
			t.Fatal(e)
		}
		want := "partially_delivered"
		if i == 1 {
			want = "completed"
		}
		if got.Status != want {
			t.Fatalf("aggregate=%s want=%s", got.Status, want)
		}
		s.ProviderStatus = "NEW"
		if err = x.repo.SavePackage(ctx, p.ID, s.Shipment, ""); err != nil {
			t.Fatal(err)
		}
	}
	var status string
	if err = x.pool.QueryRow(ctx, `SELECT status FROM marketplace_packages WHERE id=$1`, v.Packages[0].ID).Scan(&status); err != nil || status != "delivered" {
		t.Fatalf("delivery regressed: %s %v", status, err)
	}
	if err = x.repo.IssueInvoices(ctx, x.order.ID); err != nil {
		t.Fatal(err)
	}
	if err = x.repo.IssueInvoices(ctx, x.order.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = x.pool.QueryRow(ctx, `SELECT count(*) FROM marketplace_invoices WHERE order_id=$1`, x.order.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("invoice duplication %d %v", count, err)
	}
}
func TestMarketplaceCancellationPartialRefundAndPolicy(t *testing.T) {
	x := newMarketplaceFixture(t)
	x.capture(t)
	ctx := context.Background()
	v, err := x.repo.View(ctx, x.f.userIDs[0], x.order.OrderNumber)
	if err != nil {
		t.Fatal(err)
	}
	item := v.Lines[0].ID
	c := x.command("cancel")
	c.ItemID = item
	c.Quantity = 1
	c.Key = fmt.Sprintf("cancel-request-%016d", x.order.ID)
	c.Reason = "Customer cancels one item before dispatch"
	for i := 0; i < 2; i++ {
		if err = x.repo.Act(ctx, x.f.userIDs[0], c); err != nil {
			t.Fatal(err)
		}
	}
	var cancellation int64
	if err = x.pool.QueryRow(ctx, `SELECT id FROM marketplace_cancellations WHERE request_key=$1`, c.Key).Scan(&cancellation); err != nil {
		t.Fatal(err)
	}
	if err = x.repo.CompleteCancellation(ctx, cancellation); err != nil {
		t.Fatal(err)
	}
	if err = x.repo.CompleteCancellation(ctx, cancellation); err != nil {
		t.Fatal(err)
	}
	v, err = x.repo.View(ctx, x.f.userIDs[0], x.order.OrderNumber)
	if err != nil || len(v.Refunds) != 1 || v.Refunds[0].AmountCents != 1000 {
		t.Fatalf("refund %+v %v", v.Refunds, err)
	}
	c = x.command("approve_refund")
	c.RefundID = v.Refunds[0].ID
	c.Reason = "Verified cancellation and remaining paid amount"
	if err = x.repo.Act(ctx, x.f.ownerID, c); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("seller refund approval=%v", err)
	}
	if err = x.repo.Act(ctx, x.admin, c); err != nil {
		t.Fatal(err)
	}
	record, submit, err := x.repo.RefundRecord(ctx, c.RefundID)
	if err != nil || !submit {
		t.Fatalf("refund submission %v %v", submit, err)
	}
	_, submit, err = x.repo.RefundRecord(ctx, c.RefundID)
	if err != nil || submit {
		t.Fatalf("duplicate provider POST %v %v", submit, err)
	}
	failed := ports.PaymentRefund{ProviderRefundID: fmt.Sprintf("failed-refund-%d", c.RefundID), ProviderPaymentID: record.ProviderPaymentID, AmountCents: record.AmountCents, Currency: record.Currency, Status: "failed"}
	if err = x.repo.SavePartialRefund(ctx, c.RefundID, failed); err != nil {
		t.Fatal(err)
	}
	c.Action = "retry_failed_refund"
	if err = x.repo.Act(ctx, x.admin, c); err != nil {
		t.Fatal(err)
	}
	retry, submit, err := x.repo.RefundRecord(ctx, c.RefundID)
	if err != nil || !submit || retry.Receipt == record.Receipt {
		t.Fatalf("explicit failed refund retry: %+v %v %v", retry, submit, err)
	}
	if matched, err := x.repo.RefundCallback(ctx, ports.PaymentWebhook{ProviderRefundID: failed.ProviderRefundID, ProviderPaymentID: record.ProviderPaymentID, AmountCents: record.AmountCents, Currency: record.Currency, RefundReceipt: record.Receipt, Status: "failed"}); err != nil || matched {
		t.Fatalf("old callback contaminated new attempt: %v %v", matched, err)
	}
	result := ports.PaymentRefund{ProviderRefundID: fmt.Sprintf("refund-%d", c.RefundID), ProviderPaymentID: record.ProviderPaymentID, AmountCents: record.AmountCents, Currency: record.Currency, Status: "processed"}
	for i := 0; i < 2; i++ {
		if err = x.repo.SavePartialRefund(ctx, c.RefundID, result); err != nil {
			t.Fatal(err)
		}
	}
	if err = x.repo.IssueInvoices(ctx, x.order.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = x.repo.IssueCreditNote(ctx, c.RefundID); err != nil {
			t.Fatal(err)
		}
	}
	var notes int
	if err = x.pool.QueryRow(ctx, `SELECT count(*) FROM marketplace_credit_notes WHERE refund_id=$1 AND octet_length(pdf_bytes)>4`, c.RefundID).Scan(&notes); err != nil || notes != 1 {
		t.Fatalf("credit note duplication count=%d %v", notes, err)
	}
	var entries, stock int
	if err = x.pool.QueryRow(ctx, `SELECT count(*) FROM seller_ledger WHERE marketplace_refund_id=$1`, c.RefundID).Scan(&entries); err != nil || entries != 1 {
		t.Fatalf("refund ledger %d %v", entries, err)
	}
	if err = x.pool.QueryRow(ctx, `SELECT available_quantity FROM inventory_stock WHERE variant_id=$1`, x.f.variantID).Scan(&stock); err != nil || stock != 8 {
		t.Fatalf("stock=%d %v", stock, err)
	}
	var allowed bool
	if err = x.pool.QueryRow(ctx, `SELECT (policy_snapshot->>'standard_returns_allowed')::boolean FROM orders WHERE id=$1`, x.order.ID).Scan(&allowed); err != nil || allowed {
		t.Fatalf("new return policy %v %v", allowed, err)
	}
	if _, err = x.pool.Exec(ctx, `UPDATE orders SET status='delivered',delivered_at=now()-interval '8 days' WHERE id=$1`, x.order.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = x.commerce.CreateReturnRequest(ctx, x.f.userIDs[0], x.order.OrderNumber, "Change of mind is not permitted"); !errors.Is(err, ports.ErrReturnNotFound) {
		t.Fatalf("standard return allowed: %v", err)
	}
}
