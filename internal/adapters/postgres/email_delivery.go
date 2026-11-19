package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

type EmailDeliveryRepository struct {
	pool *pgxpool.Pool
}

// The portal passes an order resolved through the support case authorization
// path for Support. Only Super Admin calls this with an empty order filter.
func (r *EmailDeliveryRepository) List(ctx context.Context, orderNumber, before string) ([]domain.EmailDeliverySummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text,order_number,kind,COALESCE(provider_id::text,''),state,started_at,state_at,workflow_reference
		FROM order_email_deliveries
		WHERE started_at>now()-interval '30 days' AND ($1='' OR order_number=$1)
			AND ($2='' OR (started_at,id)<(SELECT started_at,id FROM order_email_deliveries WHERE id=NULLIF($2,'')::uuid))
		ORDER BY started_at DESC,id DESC LIMIT 51
	`, orderNumber, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.EmailDeliverySummary, error) {
		var item domain.EmailDeliverySummary
		err := row.Scan(&item.ID, &item.OrderNumber, &item.Kind, &item.ProviderID, &item.State, &item.StartedAt, &item.StateAt, &item.WorkflowReference)
		return item, err
	})
}

func NewEmailDeliveryRepository(pool *pgxpool.Pool) *EmailDeliveryRepository {
	return &EmailDeliveryRepository{pool: pool}
}

func (r *EmailDeliveryRepository) BeginEmailDelivery(ctx context.Context, message ports.EmailMessage) (ports.EmailDeliveryAttempt, error) {
	var attempt ports.EmailDeliveryAttempt
	err := r.pool.QueryRow(ctx, `
		INSERT INTO order_email_deliveries(send_key,order_number,kind,workflow_reference)
		VALUES ($1,$2,$3,$4) ON CONFLICT (send_key) DO UPDATE SET send_key=EXCLUDED.send_key
		WHERE order_email_deliveries.order_number=EXCLUDED.order_number AND order_email_deliveries.kind=EXCLUDED.kind AND order_email_deliveries.workflow_reference=EXCLUDED.workflow_reference
		RETURNING id::text,COALESCE(provider_id::text,''),state,started_at
	`, message.IdempotencyKey, message.OrderNumber, message.DeliveryKind, message.WorkflowReference).Scan(&attempt.ID, &attempt.ProviderID, &attempt.State, &attempt.StartedAt)
	return attempt, err
}

func (r *EmailDeliveryRepository) FinishEmailDelivery(ctx context.Context, id, providerID, state string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE order_email_deliveries SET provider_id=COALESCE(provider_id,NULLIF($2,'')::uuid),
			state=CASE WHEN state='submitting' THEN $3 ELSE state END,
			state_at=CASE WHEN state='submitting' THEN now() ELSE state_at END
		WHERE id=$1::uuid AND ($2='' OR provider_id IS NULL OR provider_id=NULLIF($2,'')::uuid)
	`, id, providerID, state)
	if err == nil && result.RowsAffected() != 1 {
		return ports.ErrInvalidState
	}
	return err
}

func (r *EmailDeliveryRepository) RecordEmailDeliveryEvent(ctx context.Context, e ports.EmailDeliveryEvent) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var providerID string
		err := tx.QueryRow(ctx, `
			SELECT COALESCE(provider_id::text,'') FROM order_email_deliveries
			WHERE id=$1::uuid AND started_at>now()-interval '30 days' FOR UPDATE
		`, e.DeliveryID).Scan(&providerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if providerID != "" && providerID != e.ProviderID {
			return nil
		}
		insert, err := tx.Exec(ctx, `
			INSERT INTO order_email_events(event_key,delivery_id,state,occurred_at)
			VALUES ($1,$2::uuid,$3,$4) ON CONFLICT DO NOTHING
		`, e.ID, e.DeliveryID, e.State, e.At)
		if err != nil || insert.RowsAffected() == 0 {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE order_email_deliveries SET provider_id=$2::uuid,
				state=CASE WHEN next.wins THEN $3 ELSE state END,
				state_at=CASE WHEN next.wins THEN $4 ELSE state_at END
			FROM (SELECT CASE
				WHEN state IN ('submitting','rejected','uncertain') THEN true
				WHEN $3='accepted' THEN false
				WHEN state='complained' THEN $3='complained' AND $4>state_at
				WHEN $3='complained' OR state='accepted' THEN true
				WHEN state='delayed' AND $3<>'delayed' THEN true
				WHEN $3='delayed' AND state<>'delayed' THEN false
				ELSE $4>=state_at END AS wins
			FROM order_email_deliveries WHERE id=$1::uuid) next
			WHERE id=$1::uuid
		`, e.DeliveryID, e.ProviderID, e.State, e.At)
		return err
	})
}

func (r *EmailDeliveryRepository) ExpireEmailDeliveries(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE order_email_deliveries SET state='uncertain',state_at=now()
		WHERE id IN (SELECT id FROM order_email_deliveries WHERE state='submitting'
			AND started_at<=now()-interval '23 hours' LIMIT 1000)
	`)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		DELETE FROM order_email_deliveries
		WHERE id IN (SELECT id FROM order_email_deliveries
		WHERE started_at<=now()-interval '30 days' ORDER BY started_at LIMIT 1000)
	`)
	return err
}
