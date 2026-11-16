package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wecratfs/commerce/internal/ports"
)

type SMSRepository struct{ pool *pgxpool.Pool }

func NewSMSRepository(pool *pgxpool.Pool) *SMSRepository { return &SMSRepository{pool: pool} }

func (r *SMSRepository) ClaimSMS(ctx context.Context) (job ports.SMSJob, claimed bool, err error) {
	err = WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		// A lost response or a worker crash must never cause another paid SMS.
		if _, e := tx.Exec(ctx, `UPDATE transactional_sms SET status='uncertain',updated_at=now() WHERE status='submitting' AND submitted_at<now()-interval '5 minutes'`); e != nil {
			return e
		}
		if _, e := tx.Exec(ctx, `UPDATE transactional_sms m SET status='skipped',outcome_reason='expired_or_account_unavailable',updated_at=now() WHERE status='queued' AND (expires_at<=now() OR NOT EXISTS(SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id=u.id LEFT JOIN notification_preferences p ON p.user_id=u.id WHERE u.id=m.user_id AND u.status='active' AND ur.role_slug=m.role_slug AND (SELECT count(*) FROM user_roles WHERE user_id=u.id)=1 AND length(trim(u.phone))>0 AND CASE WHEN m.category='order' THEN COALESCE(p.order_updates,true) ELSE COALESCE(p.maker_messages,true) END))`); e != nil {
			return e
		}
		e := tx.QueryRow(ctx, `SELECT m.id,m.user_id,m.template_key,u.phone,m.reference,m.update_label FROM transactional_sms m JOIN users u ON u.id=m.user_id WHERE m.status='queued' AND m.expires_at>now() ORDER BY m.id FOR UPDATE OF m SKIP LOCKED LIMIT 1`).Scan(&job.ID, &job.UserID, &job.TemplateKey, &job.Phone, &job.Reference, &job.Update)
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `UPDATE transactional_sms SET status='submitting',submitted_at=now(),updated_at=now() WHERE id=$1`, job.ID)
		claimed = e == nil
		return e
	})
	return
}

func (r *SMSRepository) FinishSMS(ctx context.Context, id int64, result ports.SMSResult) error {
	if result.Outcome != "accepted" && result.Outcome != "uncertain" && result.Outcome != "rejected" && result.Outcome != "skipped" {
		return ports.ErrInvalidState
	}
	if result.Outcome == "accepted" && result.ProviderID == "" {
		return ports.ErrInvalidState
	}
	tag, err := r.pool.Exec(ctx, `UPDATE transactional_sms SET status=$2,provider_request_id=NULLIF($3,''),outcome_reason=$2,updated_at=now() WHERE id=$1 AND status='submitting'`, id, result.Outcome, result.ProviderID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ports.ErrInvalidState
	}
	return nil
}
