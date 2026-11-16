package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *MarketplaceRepository) SendNotificationEmail(ctx context.Context, id int64, sender ports.EmailSender, publicURL string) error {
	if sender == nil {
		return nil
	}
	var message ports.EmailMessage
	var path string
	err := r.pool.QueryRow(ctx, `
		SELECT u.email,u.display_name,n.title,n.body,n.link_url
		FROM notifications n JOIN users u ON u.id=n.user_id AND u.status='active'
		WHERE n.id=$1 AND n.kind='order'
			AND n.title IN ('New paid order','Package out for delivery','Package delivered','Refund processed')
	`, id).Scan(&message.To, &message.DisplayName, &message.Subject, &message.Body, &path)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !strings.HasPrefix(path, "/orders/") && !strings.HasPrefix(path, "/seller-admin/orders/") {
		return ports.ErrInvalidState
	}
	message.ActionURL = strings.TrimRight(publicURL, "/") + path
	message.IdempotencyKey = fmt.Sprintf("marketplace-notification-%d", id)
	return sender.Send(ctx, message)
}
