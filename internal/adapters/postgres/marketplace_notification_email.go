package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/url"
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
		SELECT u.email,u.display_name,n.title,n.body,n.link_url,n.created_at
		FROM notifications n JOIN users u ON u.id=n.user_id AND u.status='active'
		WHERE n.id=$1 AND marketplace_package_notice_current(n)
		AND ((n.kind='order' AND n.title IN ('New paid order','Package packed','Package out for delivery','Package delivered','Cancellation requested','Items cancelled','Cancellation needs review','Refund pending','Refund processed'))
		 OR (n.kind='account' AND n.title IN ('Seller details submitted','Seller details approved','Seller details changes requested')))
	`, id).Scan(&message.To, &message.DisplayName, &message.Subject, &message.Body, &path, &message.DeliveryQueuedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if strings.HasPrefix(path, "/super-admin/fulfillment/reviews/") || strings.HasPrefix(path, "/seller-admin/setup?submission_id=") {
		message.WorkflowReference = path
		message.DeliveryKind = message.Subject
		message.ActionURL = strings.TrimRight(publicURL, "/") + path
		message.IdempotencyKey = fmt.Sprintf("marketplace-notification-%d", id)
		return sender.Send(ctx, message)
	}
	destination, err := url.ParseRequestURI(path)
	if err != nil || destination.Host != "" || destination.Fragment != "" {
		return ports.ErrInvalidState
	}
	if destination.RawQuery != "" && destination.RawQuery != "tab=returns" && destination.RawQuery != "tab=packages" {
		return ports.ErrInvalidState
	}
	orderPath := strings.TrimPrefix(destination.Path, "/seller-admin")
	orderPath, ok := strings.CutPrefix(orderPath, "/orders/")
	if !ok || strings.ContainsAny(orderPath, "?#\\") {
		return ports.ErrInvalidState
	}
	orderNumber, _, _ := strings.Cut(orderPath, "/")
	if orderNumber == "" {
		return ports.ErrInvalidState
	}
	message.OrderNumber = orderNumber
	message.DeliveryKind = message.Subject
	message.ActionURL = strings.TrimRight(publicURL, "/") + path
	message.IdempotencyKey = fmt.Sprintf("marketplace-notification-%d", id)
	return sender.Send(ctx, message)
}
