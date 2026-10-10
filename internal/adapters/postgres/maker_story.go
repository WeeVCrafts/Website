package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *CommerceRepository) UpdateMakerStory(ctx context.Context, ownerID int64, input domaincommerce.MakerStoryInput) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var sellerID int64
		err := tx.QueryRow(ctx, `SELECT s.id FROM sellers s JOIN users u ON u.id=s.owner_user_id AND u.status='active' WHERE s.owner_user_id=$1 AND s.status<>'closed' AND EXISTS(SELECT 1 FROM user_roles WHERE user_id=$1 AND role_slug='seller_owner') FOR UPDATE OF s`, ownerID).Scan(&sellerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrForbidden
		}
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE sellers SET display_name=$2,public_location=$3,public_story=$4,public_technique=$5,public_portrait=$6,updated_at=NOW() WHERE id=$1`, sellerID, input.DisplayName, input.Location, input.Story, input.Technique, input.PortraitURL); err != nil {
			return err
		}
		metadata, _ := json.Marshal(map[string]string{"display_name": input.DisplayName, "reason": input.Reason})
		_, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'seller.public_story_updated','seller',$2,$3,$4)`, fmt.Sprint(ownerID), fmt.Sprint(sellerID), ports.RequestID(ctx), metadata)
		return err
	})
}
