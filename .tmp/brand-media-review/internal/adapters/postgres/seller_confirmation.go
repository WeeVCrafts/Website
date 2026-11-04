package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

const confirmationCompany = `EXISTS(SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.id=$1 AND u.status='active' AND ur.role_slug IN ('admin','super_admin'))`
const confirmationSeller = `s.owner_user_id=$1 AND s.status='active' AND EXISTS(SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.id=$1 AND u.status='active' AND ur.role_slug='seller')`

func (r *CommerceRepository) CaseSellers(ctx context.Context, actor, ticket int64) ([]domain.CaseSeller, error) {
	if err := managementAuthorize(ctx, r.pool, actor, "support"); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT s.id,s.display_name,COALESCE(f.carrier,''),COALESCE(f.tracking_number,''),COALESCE(f.status,'pending') FROM support_tickets t JOIN order_items i ON i.order_id=t.order_id JOIN product_variants v ON v.id=i.variant_id JOIN products p ON p.id=v.product_id JOIN sellers s ON s.id=p.seller_id LEFT JOIN seller_fulfillments f ON f.order_id=t.order_id AND f.seller_id=s.id WHERE t.id=$1 ORDER BY s.id`, ticket)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.CaseSeller{}
	for rows.Next() {
		var item domain.CaseSeller
		if err = rows.Scan(&item.ID, &item.Name, &item.Carrier, &item.TrackingNumber, &item.FulfillmentStatus); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// The queue is bounded. Private details are independently scoped to the same seller.
func (r *CommerceRepository) ListSellerConfirmations(ctx context.Context, actor, ticket int64, seller bool, status string, before int64) ([]domain.SellerConfirmation, error) {
	predicate := confirmationCompany
	if seller {
		predicate = confirmationSeller
	} else if err := managementAuthorize(ctx, r.pool, actor, "support"); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `SELECT c.id,c.ticket_id,c.seller_id,t.ticket_number,o.order_number,s.display_name,c.question,c.status,COALESCE(c.close_reason,''),COALESCE(f.carrier,''),COALESCE(f.tracking_number,''),COALESCE(f.status,'pending'),c.created_at FROM seller_confirmations c JOIN support_tickets t ON t.id=c.ticket_id JOIN orders o ON o.id=c.order_id JOIN sellers s ON s.id=c.seller_id LEFT JOIN seller_fulfillments f ON f.order_id=c.order_id AND f.seller_id=c.seller_id WHERE `+predicate+` AND ($2::bigint=0 OR c.ticket_id=$2) AND ($3='' OR c.status=$3) AND ($4::bigint=0 OR c.id<$4) ORDER BY c.id DESC LIMIT 51`, actor, ticket, status, before)
	if err != nil {
		return nil, err
	}
	result := []domain.SellerConfirmation{}
	for rows.Next() {
		var c domain.SellerConfirmation
		if err = rows.Scan(&c.ID, &c.TicketID, &c.SellerID, &c.CaseNumber, &c.OrderNumber, &c.SellerName, &c.Question, &c.Status, &c.CloseReason, &c.Carrier, &c.TrackingNumber, &c.FulfillmentStatus, &c.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		result = append(result, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range result {
		history, err := r.pool.Query(ctx, `SELECT COALESCE(a.metadata->>'to_status',''),COALESCE(a.metadata->>'carrier',''),COALESCE(a.metadata->>'tracking_number',''),COALESCE(a.metadata->>'note',''),a.created_at FROM audit_logs a JOIN seller_confirmations c ON a.resource_id=c.seller_id::text||':'||c.order_id::text WHERE c.id=$1 AND a.action='seller.order_fulfillment_updated' ORDER BY a.created_at DESC,a.id DESC LIMIT 20`, result[i].ID)
		if err != nil {
			return nil, err
		}
		for history.Next() {
			var event domain.SellerFulfillmentEvent
			if err = history.Scan(&event.Status, &event.Carrier, &event.TrackingNumber, &event.Note, &event.CreatedAt); err != nil {
				history.Close()
				return nil, err
			}
			result[i].History = append(result[i].History, event)
		}
		err = history.Err()
		history.Close()
		if err != nil {
			return nil, err
		}

		items, err := r.pool.Query(ctx, `SELECT oi.product_name||' × '||oi.quantity::text FROM seller_confirmations c JOIN order_items oi ON oi.order_id=c.order_id JOIN product_variants v ON v.id=oi.variant_id JOIN products p ON p.id=v.product_id WHERE c.id=$1 AND p.seller_id=c.seller_id ORDER BY oi.id`, result[i].ID)
		if err != nil {
			return nil, err
		}
		for items.Next() {
			var name string
			if err = items.Scan(&name); err != nil {
				items.Close()
				return nil, err
			}
			result[i].Items = append(result[i].Items, name)
		}
		err = items.Err()
		items.Close()
		if err != nil {
			return nil, err
		}
		messages, err := r.pool.Query(ctx, `SELECT u.display_name,m.body,m.created_at FROM seller_confirmation_messages m JOIN users u ON u.id=m.author_id WHERE m.confirmation_id=$1 ORDER BY m.id LIMIT 50`, result[i].ID)
		if err != nil {
			return nil, err
		}
		for messages.Next() {
			var m domain.SellerConfirmationMessage
			if err = messages.Scan(&m.Author, &m.Body, &m.CreatedAt); err != nil {
				messages.Close()
				return nil, err
			}
			result[i].Messages = append(result[i].Messages, m)
		}
		err = messages.Err()
		messages.Close()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func confirmationAudit(ctx context.Context, tx pgx.Tx, actor, id int64, action string) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,$2,'seller_confirmation',$3,$4,'{}')`, fmt.Sprint(actor), action, fmt.Sprint(id), ports.RequestID(ctx))
	return err
}

func (r *CommerceRepository) CreateSellerConfirmation(ctx context.Context, actor, ticket, seller int64, question, key string) (int64, error) {
	var id int64
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := managementAuthorize(ctx, tx, actor, "support"); err != nil {
			return err
		}
		var oldTicket, oldSeller int64
		var oldQuestion string
		err := tx.QueryRow(ctx, `SELECT id,ticket_id,seller_id,question FROM seller_confirmations WHERE created_by=$1 AND request_key=$2`, actor, key).Scan(&id, &oldTicket, &oldSeller, &oldQuestion)
		if err == nil {
			if oldTicket != ticket || oldSeller != seller || oldQuestion != question {
				return ports.ErrIdempotencyConflict
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var order int64
		if err = tx.QueryRow(ctx, `SELECT t.order_id FROM support_tickets t WHERE t.id=$1 AND t.status NOT IN ('resolved','closed') AND EXISTS(SELECT 1 FROM order_items i JOIN product_variants v ON v.id=i.variant_id JOIN products p ON p.id=v.product_id WHERE i.order_id=t.order_id AND p.seller_id=$2) FOR UPDATE`, ticket, seller).Scan(&order); errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrSupportNotFound
		} else if err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `INSERT INTO seller_confirmations(ticket_id,order_id,seller_id,created_by,request_key,question) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(created_by,request_key) DO NOTHING RETURNING id`, ticket, order, seller, actor, key, question).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			if err = tx.QueryRow(ctx, `SELECT id,ticket_id,seller_id,question FROM seller_confirmations WHERE created_by=$1 AND request_key=$2`, actor, key).Scan(&id, &oldTicket, &oldSeller, &oldQuestion); err != nil {
				return err
			}
			if oldTicket != ticket || oldSeller != seller || oldQuestion != question {
				return ports.ErrIdempotencyConflict
			}
			return nil
		}
		if err != nil {
			return err
		}
		return confirmationAudit(ctx, tx, actor, id, "support.seller_confirmation_created")
	})
	return id, err
}

func (r *CommerceRepository) ReplySellerConfirmation(ctx context.Context, actor, id int64, body, key string) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var status string
		err := tx.QueryRow(ctx, `SELECT c.status FROM seller_confirmations c JOIN sellers s ON s.id=c.seller_id WHERE c.id=$2 AND `+confirmationSeller+` FOR UPDATE OF c`, actor, id).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrForbidden
		}
		if err != nil {
			return err
		}
		var previous string
		err = tx.QueryRow(ctx, `SELECT body FROM seller_confirmation_messages WHERE confirmation_id=$1 AND author_id=$2 AND request_key=$3`, id, actor, key).Scan(&previous)
		if err == nil {
			if previous != body {
				return ports.ErrIdempotencyConflict
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if status == "closed" {
			return ports.ErrSupportState
		}
		if _, err = tx.Exec(ctx, `INSERT INTO seller_confirmation_messages(confirmation_id,author_id,request_key,body) VALUES($1,$2,$3,$4)`, id, actor, key, body); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE seller_confirmations SET status='answered',updated_at=NOW() WHERE id=$1`, id); err != nil {
			return err
		}
		return confirmationAudit(ctx, tx, actor, id, "support.seller_confirmation_answered")
	})
}

func (r *CommerceRepository) CloseSellerConfirmation(ctx context.Context, actor, id int64, reason string) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := managementAuthorize(ctx, tx, actor, "support"); err != nil {
			return err
		}
		var status string
		err := tx.QueryRow(ctx, `SELECT status FROM seller_confirmations WHERE id=$1 FOR UPDATE`, id).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrSupportNotFound
		}
		if err != nil {
			return err
		}
		if status == "closed" {
			return nil
		}
		if status != "answered" {
			return ports.ErrSupportState
		}
		if _, err = tx.Exec(ctx, `UPDATE seller_confirmations SET status='closed',close_reason=$2,updated_at=NOW() WHERE id=$1`, id, reason); err != nil {
			return err
		}
		return confirmationAudit(ctx, tx, actor, id, "support.seller_confirmation_closed")
	})
}
