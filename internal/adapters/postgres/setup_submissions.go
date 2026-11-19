package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"regexp"
	"strconv"
	"strings"
)

func validSetupSubmission(kind string, data ports.SetupSubmissionData) bool {
	if kind == "invoice" {
		return len(strings.TrimSpace(data.LegalName)) >= 2 && len(data.LegalName) <= 160 && len(strings.TrimSpace(data.Address)) >= 10 && len(data.Address) <= 1000 && domain.ValidIndianStateCode(data.StateCode) && ((data.Registration == "unregistered" && data.GSTIN == "") || (data.Registration == "registered" && regexp.MustCompile(`^[0-9]{2}[A-Z0-9]{13}$`).MatchString(data.GSTIN)))
	}
	return kind == "tax" && regexp.MustCompile(`^[0-9]{4,8}$`).MatchString(data.HSN) && data.RateBasisPoints >= 0 && data.RateBasisPoints <= 10000 && (data.ReturnClass == "textile_craft" || data.ReturnClass == "fragile_art")
}

func (r *MarketplaceRepository) SaveSetupSubmission(ctx context.Context, actor int64, in ports.SetupSubmissionInput) (id int64, err error) {
	var data ports.SetupSubmissionData
	if len(in.Payload) > 8192 || json.Unmarshal(in.Payload, &data) != nil || len(in.Note) > 1000 || in.Kind != "invoice" && in.Kind != "tax" {
		return 0, ports.ErrInvalidState
	}
	data.GSTIN = strings.ToUpper(strings.TrimSpace(data.GSTIN))
	data.LegalName = strings.TrimSpace(data.LegalName)
	data.Address = strings.TrimSpace(data.Address)
	data.HSN = strings.TrimSpace(data.HSN)
	if in.Submit {
		data.DraftRatePercent = nil
	}
	if in.Submit && !validSetupSubmission(in.Kind, data) {
		return 0, ports.ErrInvalidState
	}
	payload, _ := json.Marshal(data)
	err = WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		a, e := marketplaceIdentity(ctx, tx, actor)
		if e != nil {
			return e
		}
		if a.role != "seller" || a.seller <= 0 || (in.SellerID > 0 && in.SellerID != a.seller) {
			return ports.ErrForbidden
		}
		var locked int64
		if e = tx.QueryRow(ctx, `SELECT id FROM sellers WHERE id=$1 AND owner_user_id=$2 AND status='active' FOR UPDATE`, a.seller, actor).Scan(&locked); e != nil {
			return e
		}
		if in.Kind == "tax" {
			if e = tx.QueryRow(ctx, `SELECT v.id FROM product_variants v JOIN products p ON p.id=v.product_id WHERE v.id=$1 AND p.seller_id=$2 AND v.status='active' AND p.status<>'archived' FOR SHARE OF p,v`, in.VariantID, a.seller).Scan(&locked); e != nil {
				return ports.ErrForbidden
			}
		} else if in.VariantID != 0 {
			return ports.ErrInvalidState
		}
		var previous ports.SetupSubmission
		var oldPayload []byte
		e = tx.QueryRow(ctx, `SELECT id,status,revision,payload,note FROM marketplace_setup_submissions WHERE seller_id=$1 AND kind=$2 AND COALESCE(variant_id,0)=$3 AND status IN ('draft','submitted','changes_requested') FOR UPDATE`, a.seller, in.Kind, in.VariantID).Scan(&previous.ID, &previous.Status, &previous.Revision, &oldPayload, &previous.Note)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if previous.ID > 0 {
			var old ports.SetupSubmissionData
			_ = json.Unmarshal(oldPayload, &old)
			if previous.Status == "submitted" {
				if in.Submit && old == data && previous.Note == in.Note && (in.ID == 0 || in.ID == previous.ID) {
					id = previous.ID
					return nil
				}
				return ports.ErrFulfillmentConflict
			}
			if in.ID != previous.ID || in.ExpectedRevision != previous.Revision {
				return ports.ErrFulfillmentConflict
			}
			if previous.Status == "changes_requested" {
				if _, e = tx.Exec(ctx, `UPDATE marketplace_setup_submissions SET status='superseded',revision=revision+1,updated_at=now() WHERE id=$1`, previous.ID); e != nil {
					return e
				}
				previous.ID = 0
			}
		} else if in.ID > 0 {
			return ports.ErrFulfillmentConflict
		}
		status := "draft"
		if in.Submit {
			status = "submitted"
		}
		if previous.ID == 0 {
			e = tx.QueryRow(ctx, `INSERT INTO marketplace_setup_submissions(seller_id,variant_id,kind,status,payload,note,created_by,submitted_at) VALUES($1,NULLIF($2,0),$3,$4,$5,$6,$7,CASE WHEN $4='submitted' THEN now() END) RETURNING id`, a.seller, in.VariantID, in.Kind, status, payload, in.Note, actor).Scan(&id)
		} else {
			e = tx.QueryRow(ctx, `UPDATE marketplace_setup_submissions SET payload=$2,note=$3,status=$4,revision=revision+1,submitted_at=CASE WHEN $4='submitted' THEN now() END,updated_at=now() WHERE id=$1 RETURNING id`, previous.ID, payload, in.Note, status).Scan(&id)
		}
		if e != nil {
			return e
		}
		if in.Submit {
			if in.Kind == "invoice" {
				_, e = tx.Exec(ctx, `UPDATE seller_invoice_profiles SET checkout_review_hold=true WHERE seller_id=$1`, a.seller)
			} else {
				_, e = tx.Exec(ctx, `UPDATE variant_tax_profiles SET checkout_review_hold=true WHERE variant_id=$1`, in.VariantID)
			}
			if e != nil {
				return e
			}
			if e = setupReviewNotice(ctx, tx, id, "Seller details submitted"); e != nil {
				return e
			}
		}
		return marketplaceAudit(ctx, tx, actor, 0, "setup_"+status, fmt.Sprintf("Submission %d (%s)", id, in.Kind))
	})
	return id, err
}

func (r *MarketplaceRepository) ReviewSetupSubmission(ctx context.Context, actor, id, revision int64, approve bool, note string) error {
	note = strings.TrimSpace(note)
	if len(note) < 4 || len(note) > 1000 {
		return ports.ErrInvalidState
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		a, err := marketplaceIdentity(ctx, tx, actor)
		if err != nil {
			return err
		}
		if a.role != "super_admin" {
			return ports.ErrForbidden
		}
		// Match checkout's seller-first lock ordering before locking the review.
		var seller, variant, current int64
		var kind, status, previousNote string
		var payload []byte
		if err = tx.QueryRow(ctx, `SELECT seller_id FROM marketplace_setup_submissions WHERE id=$1`, id).Scan(&seller); err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, `SELECT id FROM sellers WHERE id=$1 AND status='active' FOR UPDATE`, seller).Scan(&seller); err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, `SELECT kind,status,revision,COALESCE(variant_id,0),payload,review_note FROM marketplace_setup_submissions WHERE id=$1 FOR UPDATE`, id).Scan(&kind, &status, &current, &variant, &payload, &previousNote); err != nil {
			return err
		}
		if status == "approved" && approve && previousNote == note {
			return nil
		}
		if current != revision || status != "submitted" {
			return ports.ErrFulfillmentConflict
		}
		status = "changes_requested"
		if approve {
			var data ports.SetupSubmissionData
			if json.Unmarshal(payload, &data) != nil || !validSetupSubmission(kind, data) {
				return ports.ErrInvalidState
			}
			if kind == "invoice" {
				prefix := "S" + strings.ToUpper(strconv.FormatInt(seller, 36))
				if len(prefix) > 6 {
					return ports.ErrInvalidState
				}
				_, err = tx.Exec(ctx, `INSERT INTO seller_invoice_profiles(seller_id,legal_name,address,state_code,gstin,registration,prefix,verified_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(seller_id) DO UPDATE SET legal_name=EXCLUDED.legal_name,address=EXCLUDED.address,state_code=EXCLUDED.state_code,gstin=EXCLUDED.gstin,registration=EXCLUDED.registration,verified_by=EXCLUDED.verified_by,verified_at=now(),revision=seller_invoice_profiles.revision+1,checkout_review_hold=false`, seller, data.LegalName, data.Address, data.StateCode, data.GSTIN, data.Registration, prefix, actor)
			} else {
				var registration string
				if err = tx.QueryRow(ctx, `SELECT registration FROM seller_invoice_profiles WHERE seller_id=$1 AND NOT checkout_review_hold FOR SHARE`, seller).Scan(&registration); err != nil {
					return ports.ErrInvalidState
				}
				if registration == "unregistered" && data.RateBasisPoints != 0 {
					return ports.ErrInvalidState
				}
				var owned int64
				if err = tx.QueryRow(ctx, `SELECT v.id FROM product_variants v JOIN products p ON p.id=v.product_id WHERE v.id=$1 AND p.seller_id=$2 AND v.status='active' FOR UPDATE OF v`, variant, seller).Scan(&owned); err != nil {
					return ports.ErrInvalidState
				}
				_, err = tx.Exec(ctx, `INSERT INTO variant_tax_profiles(variant_id,hsn,rate_basis_points,verified_by) VALUES($1,$2,$3,$4) ON CONFLICT(variant_id) DO UPDATE SET hsn=EXCLUDED.hsn,rate_basis_points=EXCLUDED.rate_basis_points,verified_by=EXCLUDED.verified_by,verified_at=now(),revision=variant_tax_profiles.revision+1,checkout_review_hold=false`, variant, data.HSN, data.RateBasisPoints, actor)
				if err == nil {
					_, err = tx.Exec(ctx, `UPDATE product_variants SET return_class=$2,return_class_request=$2,updated_at=now() WHERE id=$1`, variant, data.ReturnClass)
				}
			}
			if err != nil {
				return err
			}
			status = "approved"
		}
		if _, err = tx.Exec(ctx, `UPDATE marketplace_setup_submissions SET status=$2,review_note=$3,reviewed_by=$4,reviewed_at=now(),revision=revision+1,updated_at=now() WHERE id=$1`, id, status, note, actor); err != nil {
			return err
		}
		if err = setupReviewNotice(ctx, tx, id, "Seller details "+strings.ReplaceAll(status, "_", " ")); err != nil {
			return err
		}
		return marketplaceAudit(ctx, tx, actor, 0, "setup_"+status, fmt.Sprintf("Submission %d: %s", id, note))
	})
}

func setupReviewNotice(ctx context.Context, tx pgx.Tx, id int64, title string) error {
	_, err := tx.Exec(ctx, `INSERT INTO notifications(user_id,kind,title,body,link_url,source_key)
 SELECT u.id,'account',$2,'Seller setup has a recorded update. Open the submission to see the next required action.',
 CASE WHEN ur.role_slug='super_admin' THEN '/super-admin/fulfillment/reviews/'||m.id ELSE '/seller-admin/setup?submission_id='||m.id END,
 'setup-review:'||m.id||':'||m.revision||':'||u.id||':'||$2
 FROM marketplace_setup_submissions m JOIN sellers s ON s.id=m.seller_id JOIN users u ON u.status='active' JOIN user_roles ur ON ur.user_id=u.id
 WHERE m.id=$1 AND ((ur.role_slug='super_admin' AND $2='Seller details submitted') OR u.id=s.owner_user_id) ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING`, id, title)
	return err
}

func (r *MarketplaceRepository) SetupSubmissions(ctx context.Context, actor, id, before int64) (list []ports.SetupSubmission, next int64, err error) {
	a, err := marketplaceIdentity(ctx, r.pool, actor)
	if err != nil {
		return nil, 0, err
	}
	if a.role != "seller" && a.role != "super_admin" {
		return nil, 0, ports.ErrForbidden
	}
	rows, err := r.pool.Query(ctx, `SELECT m.id,m.seller_id,COALESCE(m.variant_id,0),m.revision,m.kind,m.status,s.display_name,COALESCE(p.name,'Seller invoice details'),m.note,m.review_note,m.payload,m.submitted_at,m.created_at FROM marketplace_setup_submissions m JOIN sellers s ON s.id=m.seller_id LEFT JOIN product_variants v ON v.id=m.variant_id LEFT JOIN products p ON p.id=v.product_id WHERE ($1 OR m.seller_id=$2) AND ($3=0 OR m.id=$3) AND (NOT $1 OR $3<>0 OR m.status IN ('submitted','changes_requested')) AND ($4=0 OR m.id<$4) ORDER BY m.id DESC LIMIT 26`, a.role == "super_admin", a.seller, id, before)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var x ports.SetupSubmission
		if err = rows.Scan(&x.ID, &x.SellerID, &x.VariantID, &x.Revision, &x.Kind, &x.Status, &x.Seller, &x.Product, &x.Note, &x.ReviewNote, &x.Payload, &x.SubmittedAt, &x.CreatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, x)
	}
	if len(list) > 25 {
		next = list[24].ID
		list = list[:25]
	}
	return list, next, rows.Err()
}
