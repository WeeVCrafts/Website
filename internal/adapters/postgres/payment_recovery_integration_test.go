package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestPaymentRecoveryAttemptsAndLateCaptureReview(t *testing.T) {
	for _, resolution := range []string{"normal", "approve", "refund", "refund-retry", "stock-unavailable", "additional", "rollback", "race"} {
		t.Run(resolution, func(t *testing.T) {
			pool := openIntegrationPool(t)
			fixture := newCommerceFixture(t, pool, 1, 1)
			ctx := context.Background()
			commerce := NewCommerceRepository(pool)
			repo := NewPaymentRepository(pool)
			order, err := commerce.CreateOrder(ctx, fixture.userIDs[0], fixture.cartIDs[0], "payment-recovery-"+fmt.Sprint(fixture.userIDs[0]), domaincommerce.AddressInput{RecipientName: "QA Customer", Phone: "9876543210", Line1: "1 QA Lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
			if err != nil {
				t.Fatal(err)
			}
			providerOrder := "order_recovery_" + fmt.Sprint(order.ID)
			if _, err = repo.AttachProviderOrder(ctx, fixture.userIDs[0], order.OrderNumber, ports.PaymentOrder{ProviderOrderID: providerOrder, AmountCents: order.TotalCents, Currency: order.Currency}); err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'super_admin')`, fixture.ownerID); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_, _ = pool.Exec(ctx, `DELETE FROM payment_review_cases WHERE order_id=$1`, order.ID)
				_, _ = pool.Exec(ctx, `DELETE FROM notifications WHERE user_id=$1`, fixture.ownerID)
				_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE resource_type='payment_review' OR (resource_type='order' AND resource_id=$1)`, fmt.Sprint(order.ID))
				_, _ = pool.Exec(ctx, `DELETE FROM payment_webhook_events WHERE event_id LIKE $1`, providerOrder+"%")
			})
			event := func(kind, payment, suffix string) error {
				_, err := repo.ProcessWebhook(ctx, ports.PaymentWebhook{EventID: providerOrder + suffix, EventType: kind, ProviderOrderID: providerOrder, ProviderPaymentID: payment, AmountCents: order.TotalCents, Currency: order.Currency}, []byte(`{}`))
				return err
			}
			failedID := "pay_failed_" + fmt.Sprint(order.ID)
			capturedID := "pay_ok_" + fmt.Sprint(order.ID)
			if err = event("payment.failed", failedID, "failed"); err != nil {
				t.Fatal(err)
			}
			var status, reservation string
			var held bool
			if err = pool.QueryRow(ctx, `SELECT o.status,r.status,o.payment_review_required FROM orders o JOIN inventory_reservations r ON r.order_id=o.id WHERE o.id=$1`, order.ID).Scan(&status, &reservation, &held); err != nil {
				t.Fatal(err)
			}
			if status != "pending_payment" || reservation != "active" || held {
				t.Fatalf("single failed attempt closed order: %s %s %v", status, reservation, held)
			}
			if resolution == "rollback" {
				if _, err = pool.Exec(ctx, `UPDATE inventory_stock SET reserved_quantity=0 WHERE variant_id=$1`, fixture.variantID); err != nil {
					t.Fatal(err)
				}
				if err = event("payment.captured", capturedID, "rollback"); !errors.Is(err, ports.ErrInsufficientStock) {
					t.Fatalf("expected stock rollback, got %v", err)
				}
				var state, payment string
				var count int
				if err = pool.QueryRow(ctx, `SELECT o.status,pa.status,(SELECT count(*) FROM provider_payment_attempts p WHERE p.payment_attempt_id=pa.id) FROM orders o JOIN payment_attempts pa ON pa.order_id=o.id WHERE o.id=$1`, order.ID).Scan(&state, &payment, &count); err != nil {
					t.Fatal(err)
				}
				if state != "pending_payment" || payment != "pending" || count != 1 {
					t.Fatalf("partial capture persisted: %s %s %d", state, payment, count)
				}
				return
			}
			if resolution == "race" {
				if _, err = pool.Exec(ctx, `UPDATE inventory_reservations SET expires_at=NOW()-INTERVAL '1 minute' WHERE order_id=$1`, order.ID); err != nil {
					t.Fatal(err)
				}
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					defer wg.Done()
					if err := event("payment.captured", capturedID, "race"); err != nil {
						t.Error(err)
					}
				}()
				go func() {
					defer wg.Done()
					if _, err := commerce.ReleaseExpiredReservations(ctx, 100); err != nil {
						t.Error(err)
					}
				}()
				wg.Wait()
				var state string
				var review bool
				var available, reserved, sales int
				if err = pool.QueryRow(ctx, `SELECT o.status,o.payment_review_required,i.available_quantity,i.reserved_quantity,(SELECT count(*) FROM seller_ledger WHERE order_id=o.id) FROM orders o CROSS JOIN inventory_stock i WHERE o.id=$1 AND i.variant_id=$2`, order.ID, fixture.variantID).Scan(&state, &review, &available, &reserved, &sales); err != nil {
					t.Fatal(err)
				}
				if state == "paid" || !review || available != 1 || reserved != 0 || sales != 0 {
					t.Fatalf("expiry race breached hold: %s %v %d %d %d", state, review, available, reserved, sales)
				}
				return
			}
			if resolution != "normal" && resolution != "additional" {
				if _, err = pool.Exec(ctx, `UPDATE inventory_reservations SET expires_at=NOW()-INTERVAL '1 minute' WHERE order_id=$1`, order.ID); err != nil {
					t.Fatal(err)
				}
				if _, err = commerce.ReleaseExpiredReservations(ctx, 100); err != nil {
					t.Fatal(err)
				}
			}
			if err = event("payment.captured", capturedID, "captured"); err != nil {
				t.Fatal(err)
			}
			// Delivery retries, reordering and another failed provider attempt are harmless.
			if err = event("payment.captured", capturedID, "duplicate"); err != nil {
				t.Fatal(err)
			}
			if err = event("payment.failed", capturedID, "reordered"); err != nil {
				t.Fatal(err)
			}
			if resolution == "additional" {
				capturedID = "pay_extra_" + fmt.Sprint(order.ID)
				if err = event("payment.captured", capturedID, "extra"); err != nil {
					t.Fatal(err)
				}
			}
			var count int
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM provider_payment_attempts WHERE payment_attempt_id IN(SELECT id FROM payment_attempts WHERE order_id=$1)`, order.ID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			expected := 2
			if resolution == "additional" {
				expected = 3
			}
			if count != expected {
				t.Fatalf("attempt count %d", count)
			}
			if err = pool.QueryRow(ctx, `SELECT status,payment_review_required FROM orders WHERE id=$1`, order.ID).Scan(&status, &held); err != nil {
				t.Fatal(err)
			}
			if resolution == "normal" {
				if status != "paid" || held {
					t.Fatalf("capture did not fulfil atomically: %s held=%v", status, held)
				}
				var sales, events int
				if err = pool.QueryRow(ctx, `SELECT count(*) FROM seller_ledger WHERE order_id=$1 AND entry_type='sale'`, order.ID).Scan(&sales); err != nil {
					t.Fatal(err)
				}
				if err = pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id=$1 AND event_type='OrderPaid'`, order.OrderNumber).Scan(&events); err != nil {
					t.Fatal(err)
				}
				if sales != 1 || events != 1 {
					t.Fatalf("duplicate/missing settlement effects: sales=%d events=%d", sales, events)
				}
				return
			}
			if !held {
				t.Fatal("late or additional capture must hold fulfilment and payout")
			}
			reviews, err := repo.ListPaymentReviews(ctx, fixture.ownerID, 100)
			if err != nil {
				t.Fatal(err)
			}
			var id int64
			for _, review := range reviews {
				if review.OrderID == order.ID && review.ProviderPaymentID == capturedID {
					id = review.ID
				}
			}
			if id == 0 {
				t.Fatal("durable review missing")
			}
			if err = repo.ApprovePaymentReview(ctx, fixture.userIDs[0], id, "customer must not approve"); !errors.Is(err, ports.ErrForbidden) {
				t.Fatalf("customer approval: %v", err)
			}
			if _, err = repo.BeginReviewRefund(ctx, fixture.userIDs[0], id, "customer must not authorize finance refunds"); !errors.Is(err, ports.ErrForbidden) {
				t.Fatalf("customer finance refund: %v", err)
			}
			if resolution == "approve" || resolution == "stock-unavailable" {
				if resolution == "stock-unavailable" {
					_, err = pool.Exec(ctx, `UPDATE inventory_stock SET available_quantity=0 WHERE variant_id=$1`, fixture.variantID)
					if err != nil {
						t.Fatal(err)
					}
				}
				err = repo.ApprovePaymentReview(ctx, fixture.ownerID, id, "Stock revalidated by finance")
				if resolution == "stock-unavailable" {
					if !errors.Is(err, ports.ErrInsufficientStock) {
						t.Fatalf("stock approval: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if err = repo.ApprovePaymentReview(ctx, fixture.ownerID, id, "Duplicate approval"); err != nil {
					t.Fatal(err)
				}
			} else {
				record, err := repo.BeginReviewRefund(ctx, fixture.ownerID, id, "Finance authorized capture refund")
				if err != nil {
					t.Fatal(err)
				}
				var claims int
				var mu sync.Mutex
				var wg sync.WaitGroup
				for i := 0; i < 4; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						ok, err := repo.ClaimReviewRefundSubmission(ctx, id)
						mu.Lock()
						defer mu.Unlock()
						if err != nil {
							t.Error(err)
						}
						if ok {
							claims++
						}
					}()
				}
				wg.Wait()
				if claims != 1 {
					t.Fatalf("refund submit claims %d", claims)
				}
				refund := ports.PaymentRefund{ProviderRefundID: "rfnd_review_" + fmt.Sprint(id), ProviderPaymentID: record.ProviderPaymentID, AmountCents: record.AmountCents, Currency: record.Currency, Status: "processed"}
				if resolution == "refund-retry" {
					refund.Status = "failed"
					if err = repo.CompleteReviewRefund(ctx, id, refund); err != nil {
						t.Fatal(err)
					}
					previousReceipt := record.Receipt
					if err = repo.AuthorizeReviewRefundRetry(ctx, fixture.ownerID, id, "Explicit new refund authorization", refund.ProviderRefundID); err != nil {
						t.Fatal(err)
					}
					if err = repo.AuthorizeReviewRefundRetry(ctx, fixture.ownerID, id, "Replayed old authorization", refund.ProviderRefundID); !errors.Is(err, ports.ErrRefundState) {
						t.Fatalf("old authorization was accepted: %v", err)
					}
					record, err = repo.BeginReviewRefund(ctx, fixture.ownerID, id, "New refund explicitly authorized after provider failure")
					if err != nil {
						t.Fatal(err)
					}
					if record.Receipt == previousReceipt || record.SubmissionState != "queued" || record.ProviderRefundID != "" {
						t.Fatalf("new refund attempt identity not reset: %+v", record)
					}
					if ok, err := repo.ClaimReviewRefundSubmission(ctx, id); err != nil || !ok {
						t.Fatalf("new submission claim: %v %v", ok, err)
					}
					if ok, err := repo.ClaimReviewRefundSubmission(ctx, id); err != nil || ok {
						t.Fatalf("duplicate new submission claim: %v %v", ok, err)
					}
					refund.ProviderRefundID += "_second"
					refund.Status = "processed"
				}
				if err = repo.CompleteReviewRefund(ctx, id, refund); err != nil {
					t.Fatal(err)
				}
				if err = repo.CompleteReviewRefund(ctx, id, refund); err != nil {
					t.Fatal(err)
				}
			}
			if err = pool.QueryRow(ctx, `SELECT status,payment_review_required FROM orders WHERE id=$1`, order.ID).Scan(&status, &held); err != nil {
				t.Fatal(err)
			}
			expectedStatus := "refunded"
			if resolution == "approve" || resolution == "additional" {
				expectedStatus = "paid"
			}
			if held || status != expectedStatus {
				t.Fatalf("resolution status=%s held=%v", status, held)
			}
		})
	}
}
