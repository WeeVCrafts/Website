package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func sellerSupportOwner(ctx context.Context, tx pgx.Tx, userID int64) error {
	var allowed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sellers s JOIN users u ON u.id=s.owner_user_id JOIN user_roles ur ON ur.user_id=u.id AND ur.role_slug='seller_owner' WHERE s.owner_user_id=$1 AND u.status='active' AND s.status NOT IN ('suspended','closed'))`, userID).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return ports.ErrForbidden
	}
	return nil
}

func (r *CommerceRepository) CreateSellerSupportTicket(ctx context.Context, userID int64, orderNumber, subject, message string) (domaincommerce.SupportTicket, error) {
	var ticket domaincommerce.SupportTicket
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := sellerSupportOwner(ctx, tx, userID); err != nil {
			return err
		}
		var orderID *int64
		if orderNumber != "" {
			var id int64
			err := tx.QueryRow(ctx, `SELECT o.id FROM orders o WHERE o.order_number=$2 AND EXISTS(SELECT 1 FROM order_items oi JOIN product_variants v ON v.id=oi.variant_id JOIN products p ON p.id=v.product_id JOIN sellers s ON s.id=p.seller_id WHERE oi.order_id=o.id AND s.owner_user_id=$1)`, userID, orderNumber).Scan(&id)
			if errors.Is(err, pgx.ErrNoRows) {
				return ports.ErrOrderNotFound
			}
			if err != nil {
				return err
			}
			orderID = &id
		}
		ticket.OrderNumber = orderNumber
		if err := tx.QueryRow(ctx, `INSERT INTO support_tickets(ticket_number,user_id,order_id,subject,message) VALUES($1,$2,$3,$4,$5) RETURNING id,ticket_number,subject,message,status,priority,created_at,updated_at`, fmt.Sprintf("SUP-%d-%d", time.Now().UTC().UnixNano(), userID), userID, orderID, subject, message).Scan(&ticket.ID, &ticket.TicketNumber, &ticket.Subject, &ticket.Message, &ticket.Status, &ticket.Priority, &ticket.CreatedAt, &ticket.UpdatedAt); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'support.seller_ticket_created','support_ticket',$2,$3,jsonb_build_object('order_number',$4::text))`, fmt.Sprint(userID), fmt.Sprint(ticket.ID), ports.RequestID(ctx), orderNumber)
		return err
	})
	return ticket, err
}

func (r *CommerceRepository) ReplySellerSupportTicketWithID(ctx context.Context, userID, ticketID int64, message string) (int64, error) {
	var messageID int64
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := sellerSupportOwner(ctx, tx, userID); err != nil {
			return err
		}
		var ownerID int64
		var status string
		err := tx.QueryRow(ctx, `SELECT user_id,status FROM support_tickets WHERE id=$1 FOR UPDATE`, ticketID).Scan(&ownerID, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrSupportNotFound
		}
		if err != nil {
			return err
		}
		if ownerID != userID {
			return ports.ErrForbidden
		}
		if status == "closed" {
			return ports.ErrSupportState
		}
		if err := tx.QueryRow(ctx, `INSERT INTO support_ticket_messages(ticket_id,author_id,author_role,body) VALUES($1,$2,'seller_owner',$3) RETURNING id`, ticketID, userID, message).Scan(&messageID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE support_tickets SET status=CASE WHEN status IN ('resolved','waiting_customer') THEN 'in_progress' ELSE status END,updated_by=$1,updated_at=NOW() WHERE id=$2`, userID, ticketID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'support.seller_ticket_replied','support_ticket',$2,$3,jsonb_build_object('message_id',$4::bigint))`, fmt.Sprint(userID), fmt.Sprint(ticketID), ports.RequestID(ctx), messageID)
		return err
	})
	return messageID, err
}
