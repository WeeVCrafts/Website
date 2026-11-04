package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/ports"
)

// CategoryImage describes server-normalized objects prepared before publication.
// Historical releases retain their objects; only failed new uploads are removed.
type CategoryImage struct {
	ID, ObjectKey string
	ByteSize      int64
	Variants      map[string]string
}

// PublishCategory is the narrowly scoped replacement for category authoring in
// the retired Super Admin editor. It never publishes another person's draft.
func (r *StorefrontRepository) PublishCategory(ctx context.Context, actor, expected int64, action string, input storefront.Category, media *CategoryImage, reason string) error {
	input.Slug, input.Name, reason = strings.TrimSpace(input.Slug), strings.TrimSpace(input.Name), strings.TrimSpace(reason)
	if (action != "create" && action != "update") || expected <= 0 || len(reason) < 4 || len(reason) > 1000 || input.Slug == "" || len(input.Slug) > 120 {
		return ports.ErrManagementValidation
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := storefrontAuthorize(ctx, tx, actor, true); err != nil {
			return err
		}
		var head int64
		if err := tx.QueryRow(ctx, `SELECT release_id FROM storefront_head WHERE singleton FOR UPDATE`).Scan(&head); err != nil {
			return err
		}
		if head != expected {
			return storefront.ErrConflict
		}
		var data []byte
		if err := tx.QueryRow(ctx, `SELECT config FROM storefront_releases WHERE id=$1`, head).Scan(&data); err != nil {
			return err
		}
		var config storefront.Config
		if err := json.Unmarshal(data, &config); err != nil {
			return err
		}
		config.ApplyCompatibleDefaults()
		index := -1
		for i, c := range config.Categories {
			if c.Slug == input.Slug {
				index = i
				break
			}
		}
		if action == "create" && index >= 0 {
			return ports.ErrManagementConflict
		}
		if action == "update" && index < 0 {
			return ports.ErrNotFound
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM categories WHERE slug=$1)`, input.Slug).Scan(&exists); err != nil {
			return err
		}
		if action == "create" && exists {
			return ports.ErrManagementConflict
		}
		if action == "update" && !exists {
			return ports.ErrNotFound
		}
		category := input
		if index >= 0 {
			category = config.Categories[index]
			category.Name, category.Order, category.Visible = input.Name, input.Order, input.Visible
		} else {
			category.Description = ""
			category.Image = ""
			category.ObjectKey = ""
		}
		if media != nil {
			if len(media.ID) != 29 || !strings.HasPrefix(media.ID, "item-") || media.ObjectKey != "storefront/"+media.ID+".jpg" || media.ByteSize <= 0 || media.ByteSize > 8<<20 {
				return ports.ErrManagementValidation
			}
			for _, ch := range strings.TrimPrefix(media.ID, "item-") {
				if !strings.ContainsRune("0123456789abcdef", ch) {
					return ports.ErrManagementValidation
				}
			}
			for _, width := range []string{"480", "960", "1920"} {
				want := "storefront/" + media.ID + "-" + width + ".jpg"
				if width == "1920" {
					want = media.ObjectKey
				}
				if media.Variants[width] != want {
					return ports.ErrManagementValidation
				}
			}
			category.Image, category.ObjectKey = "/media/storefront/"+media.ID, media.ObjectKey
		}
		if action == "create" && media == nil {
			return ports.ErrManagementValidation
		}
		if index >= 0 {
			config.Categories[index] = category
		} else {
			config.Categories = append(config.Categories, category)
		}
		if err := config.Validate(); err != nil {
			return err
		}
		if media != nil {
			variants, err := json.Marshal(media.Variants)
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO storefront_media(id,object_key,content_type,byte_size,alt,focal_x,focal_y,variants,created_by) VALUES($1,$2,'image/jpeg',$3,$4,50,50,$5,$6)`, media.ID, media.ObjectKey, media.ByteSize, category.Name, variants, actor); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO categories(slug,name,description,image_url,image_object_key,display_order,show_in_navigation) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name,image_url=EXCLUDED.image_url,image_object_key=EXCLUDED.image_object_key,display_order=EXCLUDED.display_order,show_in_navigation=EXCLUDED.show_in_navigation`, category.Slug, category.Name, category.Description, category.Image, category.ObjectKey, category.Order, category.Visible); err != nil {
			return err
		}
		data, err := json.Marshal(config)
		if err != nil {
			return err
		}
		var release int64
		if err = tx.QueryRow(ctx, `INSERT INTO storefront_releases(config,published_by,note) VALUES($1,$2,$3) RETURNING id`, data, actor, "Category "+action+": "+reason).Scan(&release); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE storefront_head SET release_id=$1 WHERE singleton`, release); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,$2,'category',$3,$4,jsonb_build_object('reason',$5::text,'previous_release',$6::bigint,'release',$7::bigint))`, fmt.Sprint(actor), "category."+action, category.Slug, ports.RequestID(ctx), reason, head, release)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrNotFound
		}
		return err
	})
}
