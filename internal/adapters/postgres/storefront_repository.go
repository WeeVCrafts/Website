package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/ports"
)

type StorefrontRepository struct{ pool *pgxpool.Pool }
type StorefrontDraft = storefront.Draft
type StorefrontRelease = storefront.Release

func NewStorefrontRepository(pool *pgxpool.Pool) *StorefrontRepository {
	return &StorefrontRepository{pool}
}

// Bootstrap runs once under the head lock and copies actual category/settings
// records. A restart never overwrites an operator's draft or published design.
func (r *StorefrontRepository) Bootstrap(ctx context.Context) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(77490649)`); e != nil {
			return e
		}
		var exists bool
		if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM storefront_head)`).Scan(&exists); e != nil || exists {
			return e
		}
		c := storefront.Defaults()
		rows, e := tx.Query(ctx, `SELECT slug,name,description,image_url,image_object_key,display_order,show_in_navigation FROM categories ORDER BY display_order,slug`)
		if e != nil {
			return e
		}
		for rows.Next() {
			var v storefront.Category
			if e = rows.Scan(&v.Slug, &v.Name, &v.Description, &v.Image, &v.ObjectKey, &v.Order, &v.Visible); e != nil {
				rows.Close()
				return e
			}
			c.Categories = append(c.Categories, v)
		}
		rows.Close()
		if e = rows.Err(); e != nil {
			return e
		}
		settings, e := ReadStorefrontSettings(ctx, r.pool)
		if e != nil {
			return e
		}
		c.Settings.SupportEmail = settings.SupportEmail
		c.Settings.ShippingInformation = settings.ShippingInformation
		for _, l := range settings.SocialLinks() {
			c.Settings.Social = append(c.Settings.Social, storefront.Link{Label: l.Name, URL: l.URL})
		}
		c.Promotions, e = readStorefrontPromotions(ctx, tx)
		if e != nil {
			return e
		}
		if e = c.Validate(); e != nil {
			return e
		}
		data, e := json.Marshal(c)
		if e != nil {
			return e
		}
		var id int64
		if e = tx.QueryRow(ctx, `INSERT INTO storefront_releases(config,note) VALUES($1,'Initial accepted storefront') RETURNING id`, data).Scan(&id); e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO storefront_head(singleton,release_id) VALUES(true,$1)`, id)
		return e
	})
}
func ReadPublishedStorefront(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) (storefront.Revision, error) {
	if rev, ok := storefront.Current(ctx); ok {
		return rev, nil
	}
	var rev storefront.Revision
	var data []byte
	e := q.QueryRow(ctx, `SELECT r.id,r.config FROM storefront_head h JOIN storefront_releases r ON r.id=h.release_id WHERE h.singleton`).Scan(&rev.ID, &data)
	if errors.Is(e, pgx.ErrNoRows) {
		rev.Config = storefront.Defaults()
		return rev, nil
	}
	if e == nil {
		e = json.Unmarshal(data, &rev.Config)
		rev.Config.ApplyCompatibleDefaults()
	}
	return rev, e
}
func storefrontAuthorize(ctx context.Context, tx pgx.Tx, actor int64, super bool) error {
	var allowed bool
	// The actor row also serializes role revocation with every editor mutation.
	if e := tx.QueryRow(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, actor).Scan(&actor); e != nil {
		return ports.ErrForbidden
	}
	e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.id=$1 AND u.status='active' AND (ur.role_slug='super_admin' OR (NOT $2 AND ur.role_slug='admin')))`, actor, super).Scan(&allowed)
	if e != nil {
		return e
	}
	if !allowed {
		return ports.ErrForbidden
	}
	return nil
}
func storefrontAudit(ctx context.Context, tx pgx.Tx, actor, id int64, action, note string) error {
	_, e := tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,$2,'storefront',$3,$4,jsonb_build_object('note',$5::text))`, fmt.Sprint(actor), "storefront."+action, fmt.Sprint(id), ports.RequestID(ctx), note)
	return e
}
func (r *StorefrontRepository) NewDraft(ctx context.Context, actor, restore int64) (id int64, err error) {
	err = WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if e := storefrontAuthorize(ctx, tx, actor, false); e != nil {
			return e
		}
		var base int64
		var data []byte
		if e := tx.QueryRow(ctx, `SELECT h.release_id,r.config FROM storefront_head h JOIN storefront_releases r ON r.id=CASE WHEN $1::bigint>0 THEN $1 ELSE h.release_id END WHERE h.singleton`, restore).Scan(&base, &data); e != nil {
			return e
		}
		if e := tx.QueryRow(ctx, `INSERT INTO storefront_drafts(base_release,config,created_by,updated_by) VALUES($1,$2,$3,$3) RETURNING id`, base, data, actor).Scan(&id); e != nil {
			return e
		}
		return storefrontAudit(ctx, tx, actor, id, "draft_created", fmt.Sprintf("Restore source release %d", restore))
	})
	return
}
func scanStorefrontDraft(row pgx.Row) (d StorefrontDraft, e error) {
	var data []byte
	e = row.Scan(&d.ID, &d.Version, &d.BaseRelease, &d.CreatedBy, &d.State, &d.Note, &d.Updated, &data)
	if e == nil {
		e = json.Unmarshal(data, &d.Config)
		d.Config.ApplyCompatibleDefaults()
	}
	return
}

const draftColumns = `id,version,base_release,created_by,state,review_note,updated_at,config`

func (r *StorefrontRepository) Draft(ctx context.Context, actor, id int64) (d StorefrontDraft, err error) {
	err = WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if e := storefrontAuthorize(ctx, tx, actor, false); e != nil {
			return e
		}
		var e error
		d, e = scanStorefrontDraft(tx.QueryRow(ctx, `SELECT `+draftColumns+` FROM storefront_drafts WHERE id=$1`, id))
		return e
	})
	return
}
func (r *StorefrontRepository) List(ctx context.Context, actor int64) ([]StorefrontDraft, []StorefrontRelease, error) {
	return r.ListAtPage(ctx, actor, 1)
}
func (r *StorefrontRepository) ListAtPage(ctx context.Context, actor int64, page int) (drafts []StorefrontDraft, releases []StorefrontRelease, err error) {
	err = WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if e := storefrontAuthorize(ctx, tx, actor, false); e != nil {
			return e
		}
		rows, e := tx.Query(ctx, `SELECT `+draftColumns+` FROM storefront_drafts ORDER BY updated_at DESC,id DESC LIMIT 30 OFFSET $1`, max(0, min(page-1, 100000))*30)
		if e != nil {
			return e
		}
		for rows.Next() {
			d, e := scanStorefrontDraft(rows)
			if e != nil {
				rows.Close()
				return e
			}
			drafts = append(drafts, d)
		}
		rows.Close()
		if e = rows.Err(); e != nil {
			return e
		}
		rows, e = tx.Query(ctx, `SELECT id,note,published_at FROM storefront_releases ORDER BY id DESC LIMIT 30 OFFSET $1`, max(0, min(page-1, 100000))*30)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var v StorefrontRelease
			if e = rows.Scan(&v.ID, &v.Note, &v.Published); e != nil {
				return e
			}
			releases = append(releases, v)
		}
		return rows.Err()
	})
	return
}
func (r *StorefrontRepository) Save(ctx context.Context, actor, id, expected int64, c storefront.Config) error {
	if e := c.Validate(); e != nil {
		return e
	}
	data, e := json.Marshal(c)
	if e != nil {
		return e
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if e := storefrontAuthorize(ctx, tx, actor, false); e != nil {
			return e
		}
		d, e := scanStorefrontDraft(tx.QueryRow(ctx, `SELECT `+draftColumns+` FROM storefront_drafts WHERE id=$1 FOR UPDATE`, id))
		if e != nil {
			return e
		}
		if d.Version != expected {
			return storefront.ErrConflict
		}
		if d.State != "draft" {
			return storefront.ErrLocked
		}
		if _, e = tx.Exec(ctx, `UPDATE storefront_drafts SET config=$2,version=version+1,updated_by=$3,updated_at=now() WHERE id=$1`, id, data, actor); e != nil {
			return e
		}
		return storefrontAudit(ctx, tx, actor, id, "draft_saved", "")
	})
}
func (r *StorefrontRepository) Transition(ctx context.Context, actor, id, expected int64, action, note string) error {
	note = strings.TrimSpace(note)
	if len(note) > 2000 {
		return fmt.Errorf("%w: use a note up to 2000 characters", storefront.ErrValidation)
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if e := storefrontAuthorize(ctx, tx, actor, action != "submit"); e != nil {
			return e
		}
		d, e := scanStorefrontDraft(tx.QueryRow(ctx, `SELECT `+draftColumns+` FROM storefront_drafts WHERE id=$1 FOR UPDATE`, id))
		if e != nil {
			return e
		}
		if d.Version != expected {
			return storefront.ErrConflict
		}
		next := ""
		switch action {
		case "submit":
			if d.State != "draft" {
				return storefront.ErrLocked
			}
			if e = d.Config.Validate(); e != nil {
				return e
			}
			next = "submitted"
		case "request-changes":
			if d.State != "submitted" || len(note) < 4 {
				return ports.ErrInvalidState
			}
			next = "draft"
		case "publish":
			// Own reviewed drafts may be published; another person's draft must
			// first be submitted to prevent publishing their work in progress.
			if d.State != "submitted" && !(d.State == "draft" && d.CreatedBy == actor) {
				return ports.ErrInvalidState
			}
			if len(note) < 4 {
				return fmt.Errorf("%w: add a publication note", storefront.ErrValidation)
			}
			if e = d.Config.Validate(); e != nil {
				return e
			}
			var head int64
			if e = tx.QueryRow(ctx, `SELECT release_id FROM storefront_head WHERE singleton FOR UPDATE`).Scan(&head); e != nil {
				return e
			}
			if head != d.BaseRelease {
				return storefront.ErrConflict
			}
			if e = validateStorefrontReferences(ctx, tx, d.Config); e != nil {
				return e
			}
			if e = publishStorefrontCategories(ctx, tx, d.Config.Categories); e != nil {
				return e
			}
			if e = publishStorefrontPromotions(ctx, tx, actor, d.Config.Promotions); e != nil {
				return e
			}
			data, _ := json.Marshal(d.Config)
			var release int64
			if e = tx.QueryRow(ctx, `INSERT INTO storefront_releases(config,published_by,note) VALUES($1,$2,$3) RETURNING id`, data, actor, note).Scan(&release); e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, `UPDATE storefront_head SET release_id=$1 WHERE singleton`, release); e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, `UPDATE storefront_drafts SET published_release=$2 WHERE id=$1`, id, release); e != nil {
				return e
			}
			next = "published"
		default:
			return ports.ErrInvalidState
		}
		if _, e = tx.Exec(ctx, `UPDATE storefront_drafts SET state=$2,version=version+1,review_note=$3,updated_by=$4,updated_at=now() WHERE id=$1`, id, next, note, actor); e != nil {
			return e
		}
		return storefrontAudit(ctx, tx, actor, id, action, note)
	})
}
func publishStorefrontCategories(ctx context.Context, tx pgx.Tx, categories []storefront.Category) error {
	keep := []string{}
	for _, c := range categories {
		keep = append(keep, c.Slug)
	}
	var referenced bool
	if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM categories c WHERE NOT(c.slug=ANY($1::text[])) AND (EXISTS(SELECT 1 FROM products p WHERE p.category_slug=c.slug) OR EXISTS(SELECT 1 FROM product_categories pc WHERE pc.category_slug=c.slug) OR EXISTS(SELECT 1 FROM commission_rules cr WHERE cr.category_slug=c.slug)))`, keep).Scan(&referenced); e != nil {
		return e
	}
	if referenced {
		return fmt.Errorf("%w: a removed category is still assigned to products or commission rules; hide it instead", storefront.ErrValidation)
	}
	if _, e := tx.Exec(ctx, `DELETE FROM categories WHERE NOT(slug=ANY($1::text[]))`, keep); e != nil {
		return e
	}
	for _, c := range categories {
		if _, e := tx.Exec(ctx, `INSERT INTO categories(slug,name,description,image_url,image_object_key,display_order,show_in_navigation) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name,description=EXCLUDED.description,image_url=EXCLUDED.image_url,image_object_key=EXCLUDED.image_object_key,display_order=EXCLUDED.display_order,show_in_navigation=EXCLUDED.show_in_navigation`, c.Slug, c.Name, c.Description, c.Image, c.ObjectKey, c.Order, c.Visible); e != nil {
			return e
		}
	}
	return nil
}
