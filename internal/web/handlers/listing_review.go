package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/a-h/templ"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	"github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	adminpages "github.com/wecratfs/commerce/web/admin"
	"github.com/wecratfs/commerce/web/components"
	sellerpages "github.com/wecratfs/commerce/web/selleradmin"
)

const adminListingReview = "/super-admin/products/listing-review"
const sellerListingReview = "/seller-admin/products/listing-review"

func (h *LivePortalHandler) serveListingReview(w http.ResponseWriter, r *http.Request, actorID int64, role identity.Role, csrf string) {
	admin := r.URL.Path == adminListingReview
	if (admin && role != identity.RoleSuperAdmin) || (!admin && role != identity.RoleSeller) {
		denyPortalAccess(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	idText := r.URL.Query().Get("product_id")
	var message string
	if r.URL.Query().Get("saved") == "1" {
		message = "Review saved."
	}
	repository := postgres.NewCommerceRepository(h.pool)
	if r.Method == http.MethodPost && admin {
		r.Body = http.MaxBytesReader(w, r.Body, 32768)
		if !commerceSafePost(r, csrf) {
			http.Error(w, "Invalid session or request.", http.StatusForbidden)
			return
		}
		productID, _ := strconv.ParseInt(idText, 10, 64)
		revision, _ := strconv.ParseInt(r.PostForm.Get("revision"), 10, 64)
		decision := r.PostForm.Get("decision")
		reason := strings.TrimSpace(r.PostForm.Get("reason"))
		if productID <= 0 || revision <= 0 || (decision != "approve" && decision != "changes") ||
			utf8.RuneCountInString(reason) > 1000 || (decision == "changes" && utf8.RuneCountInString(reason) < 4) {
			http.Error(w, "Choose a decision and include a reason when requesting changes.", http.StatusUnprocessableEntity)
			return
		}
		err := repository.ReviewListingChange(r.Context(), actorID, productID, revision, decision == "approve", reason)
		if err == nil {
			http.Redirect(w, r, adminListingReview+"?product_id="+strconv.FormatInt(productID, 10)+"&saved=1", http.StatusSeeOther)
			return
		}
		if !errors.Is(err, ports.ErrInvalidState) {
			marketplaceHTTPError(w, err)
			return
		}
		w.WriteHeader(http.StatusConflict)
		message = "The listing or proposal changed. Review the latest values before trying again."
	} else if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body templ.Component
	title := "Listing reviews"
	if idText == "" && admin {
		rows, err := h.query(r, `SELECT p.id,p.name,s.display_name,
			EXISTS(SELECT 1 FROM listing_price_changes pc WHERE pc.product_id=p.id),
			c.cover_proposed IS NOT NULL OR EXISTS(SELECT 1 FROM listing_media_changes mc WHERE mc.product_id=p.id)
			FROM listing_change_reviews c JOIN products p ON p.id=c.product_id
			JOIN sellers s ON s.id=p.seller_id
			WHERE c.status='pending' AND p.status='approved' AND s.status='active'
			ORDER BY c.updated_at,c.product_id LIMIT 100`)
		if err != nil {
			http.Error(w, "The review queue is temporarily unavailable.", http.StatusServiceUnavailable)
			return
		}
		defer rows.Close()
		var queue []viewmodels.ListingReviewRow
		for rows.Next() {
			var row viewmodels.ListingReviewRow
			if err := rows.Scan(&row.ID, &row.Product, &row.Seller, &row.Price, &row.Media); err != nil {
				http.Error(w, "The review queue is temporarily unavailable.", http.StatusServiceUnavailable)
				return
			}
			queue = append(queue, row)
		}
		if rows.Err() != nil {
			http.Error(w, "The review queue is temporarily unavailable.", http.StatusServiceUnavailable)
			return
		}
		body = components.ListingReviewQueue(queue)
	} else {
		productID, _ := strconv.ParseInt(idText, 10, 64)
		if productID <= 0 {
			http.NotFound(w, r)
			return
		}
		change, err := repository.ListingChange(r.Context(), actorID, productID)
		if err != nil {
			marketplaceHTTPError(w, err)
			return
		}
		title = "Review listing changes"
		body = components.ListingReviewDetails(change, admin, csrf, message, listingReviewMediaView)
	}
	ctx := templ.WithChildren(r.Context(), body)
	name := h.userDisplayName(r, actorID)
	var document templ.Component
	if admin {
		document = adminpages.AdminDocument(viewmodels.AdminPage{OwnContentHeading: true, Title: title, Active: "products",
			NavActive: "products", PortalRoot: "/super-admin", PortalLabel: "Super Admin", RoleLabel: "Super Admin",
			DisplayName: name, CSRFToken: csrf, CanFinance: true, CanManageCatalog: true, CanSecurity: true})
	} else {
		document = sellerpages.Document(viewmodels.SellerAdminPage{OwnContentHeading: true, Title: title, Active: "products",
			DisplayName: name, RoleLabel: "Seller", CSRFToken: csrf, CanProducts: true})
	}
	_ = document.Render(ctx, w)
}

func listingReviewMediaView(path string) string {
	if strings.HasPrefix(path, "/media/products/") {
		return productImagePreviewURL(path)
	}
	return ""
}
