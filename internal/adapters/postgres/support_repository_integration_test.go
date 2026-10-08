package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	storageadapter "github.com/wecratfs/commerce/internal/adapters/storage"
	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"

	"github.com/wecratfs/commerce/internal/ports"
)

func TestSupportAgentReplyPersistsWithoutChangingStatusOrInternalStatusNote(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var agentID, customerID, otherCustomerID, ticketID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status) VALUES($1,'Support reply fixture','active') RETURNING id`, "support-agent-"+suffix+"@example.invalid").Scan(&agentID); err != nil {
		t.Fatalf("create support agent fixture: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status) VALUES($1,'Support customer fixture','active') RETURNING id`, "support-customer-"+suffix+"@example.invalid").Scan(&customerID); err != nil {
		t.Fatalf("create support customer fixture: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status) VALUES($1,'Other support customer fixture','active') RETURNING id`, "support-other-customer-"+suffix+"@example.invalid").Scan(&otherCustomerID); err != nil {
		t.Fatalf("create unrelated support customer fixture: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO support_tickets(ticket_number,user_id,subject,message,status,agent_note) VALUES($1,$2,'Reply persistence fixture','Original customer message.','in_progress','Keep this internal note.') RETURNING id`, "SUP-REPLY-"+suffix, customerID).Scan(&ticketID); err != nil {
		t.Fatalf("create support ticket fixture: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE actor_id IN ($1,$2) OR resource_id=$3`, fmt.Sprint(agentID), fmt.Sprint(customerID), fmt.Sprint(ticketID))
		_, _ = pool.Exec(context.Background(), `DELETE FROM support_tickets WHERE id=$1`, ticketID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id IN ($1,$2,$3)`, agentID, customerID, otherCustomerID)
	})

	if err := NewCommerceRepository(pool).ReplySupportTicket(ctx, agentID, ticketID, "Your replacement is on the way."); err != nil {
		t.Fatalf("persist support reply: %v", err)
	}
	var gotReply, status, note string
	if err := pool.QueryRow(ctx, `SELECT m.body,t.status,t.agent_note FROM support_ticket_messages m JOIN support_tickets t ON t.id=m.ticket_id WHERE m.ticket_id=$1 ORDER BY m.id DESC LIMIT 1`, ticketID).Scan(&gotReply, &status, &note); err != nil {
		t.Fatalf("read saved support reply: %v", err)
	}
	if gotReply != "Your replacement is on the way." || status != "in_progress" || note != "Keep this internal note." {
		t.Fatalf("reply changed unrelated case workflow: reply=%q status=%q note=%q", gotReply, status, note)
	}

	repository := NewCommerceRepository(pool)
	if os.Getenv("S3_INTEGRATION") == "1" {
		storage, err := storageadapter.NewS3(os.Getenv("S3_ADDR"), "wecratfs", "wecratfs-local-only", os.Getenv("S3_BUCKET"), false)
		if err != nil {
			t.Fatal(err)
		}
		key := fmt.Sprintf("support/%d/%032x", agentID, time.Now().UnixNano())
		payload := []byte{137, 80, 78, 71, 13, 10, 26, 10}
		if err := storage.PutObject(ctx, key, "image/png", int64(len(payload)), bytes.NewReader(payload)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = storage.DeleteObject(context.Background(), key) })
		attachment, err := repository.CreateSupportAttachment(ctx, ticketID, 0, agentID, "proof.png", "image/png", int64(len(payload)), key)
		if err != nil {
			t.Fatal(err)
		}
		service := applicationcommerce.NewSupportAttachmentService(storage, repository, nil)
		body, _, err := service.Open(ctx, customerID, attachment.ID)
		if err != nil {
			t.Fatalf("ticket owner cannot read agent attachment: %v", err)
		}
		got, err := io.ReadAll(body)
		_ = body.Close()
		if err != nil || !bytes.Equal(got, payload) {
			t.Fatalf("attachment content mismatch: %v", err)
		}
		if body, _, err := service.Open(ctx, otherCustomerID, attachment.ID); !errors.Is(err, ports.ErrForbidden) {
			if body != nil {
				_ = body.Close()
			}
			t.Fatalf("unrelated customer attachment read returned %v", err)
		}
	}
	if err := repository.ReplyCustomerSupportTicket(ctx, customerID, ticketID, "I have added the requested details."); err != nil {
		t.Fatalf("persist customer follow-up: %v", err)
	}
	if err := repository.ReplyCustomerSupportTicket(ctx, otherCustomerID, ticketID, "This is not my ticket."); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("unrelated customer reply returned %v", err)
	}
	var customerReply, replyRole, updatedStatus string
	if err := pool.QueryRow(ctx, `SELECT m.body,m.author_role,t.status FROM support_ticket_messages m JOIN support_tickets t ON t.id=m.ticket_id WHERE m.ticket_id=$1 ORDER BY m.id DESC LIMIT 1`, ticketID).Scan(&customerReply, &replyRole, &updatedStatus); err != nil {
		t.Fatalf("read customer follow-up: %v", err)
	}
	if customerReply != "I have added the requested details." || replyRole != "customer" || updatedStatus != "in_progress" {
		t.Fatalf("customer follow-up was not persisted correctly: reply=%q role=%q status=%q", customerReply, replyRole, updatedStatus)
	}
	if _, err := pool.Exec(ctx, `UPDATE support_tickets SET status='closed' WHERE id=$1`, ticketID); err != nil {
		t.Fatalf("close support fixture: %v", err)
	}
	if err := repository.ReplyCustomerSupportTicket(ctx, customerID, ticketID, "Please reopen this."); !errors.Is(err, ports.ErrSupportState) {
		t.Fatalf("closed ticket reply returned %v", err)
	}
}
