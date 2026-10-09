package postgres

import (
	"context"
	"fmt"

	"github.com/wecratfs/commerce/internal/ports"
)

func (r *CommerceRepository) PrepareOrderConfirmation(ctx context.Context, eventID int64, number string) (ports.EmailRecipient, bool, error) {
	var recipient ports.EmailRecipient
	var enabled bool
	err := r.pool.QueryRow(ctx, `SELECT u.id, u.email, u.display_name, COALESCE(p.order_updates,true)
		FROM orders o JOIN users u ON u.id=o.user_id
		LEFT JOIN notification_preferences p ON p.user_id=u.id
		WHERE o.order_number=$1`, number).Scan(&recipient.UserID, &recipient.Email, &recipient.DisplayName, &enabled)
	if err != nil {
		return recipient, false, err
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO notifications(user_id,kind,title,body,source_event_id)
		VALUES($1,'order','Order received',$2,$3) ON CONFLICT (source_event_id) WHERE source_event_id IS NOT NULL DO NOTHING`, recipient.UserID, fmt.Sprintf("Order %s is saved. Open your orders to check payment and delivery status.", number), eventID)
	return recipient, enabled, err
}
