package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-pdf/fpdf"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	adminpages "github.com/wecratfs/commerce/web/admin"
	"github.com/wecratfs/commerce/web/assets"
	"github.com/wecratfs/commerce/web/components"
	"github.com/wecratfs/commerce/web/pages"
	sellerpages "github.com/wecratfs/commerce/web/selleradmin"
)

type MarketplaceHandler struct {
	Storage                             ports.ObjectStorage
	Repository                          *postgres.MarketplaceRepository
	Sessions                            *applicationauth.SessionService
	ShippingConfigured, BookingsEnabled bool
	PickupChecker                       interface {
		PickupPostcodes(context.Context) (map[string]string, error)
	}
}

func marketplaceRoute(path string) (root, number, back string, ok bool) {
	for _, base := range []string{"/account", "/seller-admin", "/admin", "/super-admin"} {
		if path == base+"/fulfillment" || path == base+"/fulfillment/action" {
			return base + "/fulfillment", "", base, true
		}
	}
	for _, base := range []string{"/orders/", "/seller-admin/orders/"} {
		if strings.HasPrefix(path, base) && strings.HasSuffix(path, "/fulfillment") {
			n := strings.TrimSuffix(strings.TrimPrefix(path, base), "/fulfillment")
			if n == "" || strings.Contains(n, "/") {
				return "", "", "", false
			}
			if base == "/orders/" {
				return "/account/fulfillment", n, "/orders", true
			}
			return "/seller-admin/fulfillment", n, "/seller-admin", true
		}
	}
	return "", "", "", false
}
func (h *MarketplaceHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		root, number, back, ok := marketplaceRoute(r.URL.Path)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		if h.Repository == nil || h.Sessions == nil {
			http.Error(w, "Fulfillment temporarily unavailable", 503)
			return
		}
		cookie, e := r.Cookie("wecratfs_session")
		if e != nil {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), 303)
			return
		}
		session, e := h.Sessions.Get(r.Context(), cookie.Value)
		if e != nil {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), 303)
			return
		}
		if number == "" {
			number = r.URL.Query().Get("order")
		}
		if len(number) > 120 {
			http.Error(w, "Invalid order", 400)
			return
		}
		before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
		caseBefore, _ := strconv.ParseInt(r.URL.Query().Get("case_before"), 10, 64)
		queue := r.URL.Query().Get("queue")
		view, e := h.Repository.ViewPage(r.Context(), session.UserID, number, before, caseBefore, queue)
		if e != nil {
			marketplaceHTTPError(w, e)
			return
		}
		allowed := strings.HasPrefix(root, "/account/") && view.Role == "customer" || strings.HasPrefix(root, "/seller-admin/") && view.Role == "seller" || strings.HasPrefix(root, "/super-admin/") && view.Role == "super_admin" || strings.HasPrefix(root, "/admin/") && (view.Role == "admin" || view.Role == "super_admin")
		if !allowed {
			http.Error(w, "Forbidden", 403)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/action") {
			if r.Method != "POST" {
				w.Header().Set("Allow", "POST")
				http.Error(w, "Method not allowed", 405)
				return
			}
			limit := int64(32768)
			if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				limit = 201 << 20
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			parseErr := error(nil)
			if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				parseErr = r.ParseMultipartForm(1 << 20)
				if r.MultipartForm != nil {
					defer r.MultipartForm.RemoveAll()
				}
			} else {
				parseErr = r.ParseForm()
			}
			if parseErr != nil {
				http.Error(w, "Invalid request", 400)
				return
			}
			if session.CSRFToken == "" || subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(r.PostForm.Get("csrf_token"))) != 1 {
				http.Error(w, "Forbidden", 403)
				return
			}
			f := r.PostForm
			num := func(k string) int64 { n, _ := strconv.ParseInt(f.Get(k), 10, 64); return n }
			c := ports.MarketplaceCommand{Action: f.Get("action"), Number: f.Get("order_number"), Reason: strings.TrimSpace(f.Get("reason")), Key: f.Get("request_key"), SellerID: num("seller_id"), ItemID: num("item_id"), PackageID: num("package_id"), RefundID: num("refund_id"), CaseID: num("case_id"), Revision: num("revision"), Quantity: int(num("quantity")), Parcel: ports.Parcel{WeightGrams: int(num("weight_grams")), LengthCM: int(num("length_cm")), WidthCM: int(num("width_cm")), HeightCM: int(num("height_cm"))}, Items: map[int64]int{}}
			c.Kind, c.Resolution, c.EvidenceException, c.QCEvidence = f.Get("claim_kind"), f.Get("resolution"), strings.TrimSpace(f.Get("evidence_exception")), strings.TrimSpace(f.Get("qc_evidence"))
			for k := range f {
				if strings.HasPrefix(k, "item_") {
					id, _ := strconv.ParseInt(strings.TrimPrefix(k, "item_"), 10, 64)
					q := num(k)
					if id > 0 && q > 0 && q <= 50000 {
						c.Items[id] = int(q)
					}
				}
			}
			for _, action := range []string{"cancel", "reject", "ready", "approve_refund", "reject_refund", "retry_failed_refund", "claim_status", "prepare_replacement"} {
				if c.Action == action && f.Get("confirmed") != "yes" {
					http.Error(w, "Confirm the action before submitting", 422)
					return
				}
			}
			c.Status = f.Get("status")
			c.Restock = f.Get("restock") == "yes"
			c.InspectionPassed = f.Get("inspection_passed") == "yes"
			c.Carrier = f.Get("carrier")
			c.Tracking = f.Get("tracking_number")
			c.PreferredDate = f.Get("preferred_date")
			switch c.Action {
			case "upload_claim_evidence":
				if r.MultipartForm == nil || len(r.MultipartForm.File["evidence"]) != 1 {
					http.Error(w, "Choose one video or photo", 422)
					return
				}
				e = h.uploadClaimEvidence(r.Context(), session.UserID, c.CaseID, r.MultipartForm.File["evidence"][0])
			case "publish_policy", "pause_shipping", "configure_pilot", "classify_variant":
				if f.Get("confirmed") != "yes" {
					http.Error(w, "Confirm the policy or shipping change", 422)
					return
				}
				if c.Action == "configure_pilot" && !h.BookingsEnabled {
					http.Error(w, "Live booking is paused in the application configuration", 409)
					return
				}
				if c.Action == "classify_variant" {
					c.Reason = f.Get("return_class")
				}
				e = h.Repository.ConfigurePolicy(r.Context(), session.UserID, c.Action, c.Reason, num("customer_id"), num("variant_id"))
			case "configure_live_catalog":
				if f.Get("confirmed") != "yes" {
					http.Error(w, "Confirm the launch catalog before submitting", 422)
					return
				}
				if !h.BookingsEnabled {
					http.Error(w, "Live booking is paused in the application configuration", 409)
					return
				}
				e = h.Repository.ConfigureLiveCatalog(r.Context(), session.UserID, num("variant_id"), num("second_variant_id"), c.Reason)
			case "save_shipping_profile":
				if view.Role != "super_admin" || h.PickupChecker == nil {
					http.Error(w, "Live shipping setup is required", 409)
					return
				}
				locations, checkErr := h.PickupChecker.PickupPostcodes(r.Context())
				if checkErr != nil {
					marketplaceHTTPError(w, checkErr)
					return
				}
				if pin, found := locations[f.Get("pickup_location")]; !found || pin != f.Get("postal_code") {
					http.Error(w, "Pickup location and PIN do not match Shiprocket", 422)
					return
				}
				e = h.Repository.SaveShippingProfile(r.Context(), session.UserID, c.SellerID, f.Get("pickup_location"), f.Get("postal_code"), c.Reason)
			case "save_invoice_profile":
				if view.Role != "super_admin" {
					http.Error(w, "Forbidden", 403)
					return
				}
				e = h.Repository.SaveInvoiceProfile(r.Context(), session.UserID, ports.SellerInvoiceProfile{SellerID: c.SellerID, LegalName: f.Get("legal_name"), Address: f.Get("address"), StateCode: f.Get("state_code"), GSTIN: strings.ToUpper(f.Get("gstin")), Registration: f.Get("registration"), Prefix: f.Get("prefix")}, c.Reason)
			case "save_tax_profile":
				if view.Role != "super_admin" {
					http.Error(w, "Forbidden", 403)
					return
				}
				e = h.Repository.SaveTaxProfile(r.Context(), session.UserID, num("variant_id"), f.Get("hsn"), int(num("rate_basis_points")), c.Reason)
			default:
				e = h.Repository.Act(r.Context(), session.UserID, c)
			}
			if e != nil {
				marketplaceHTTPError(w, e)
				return
			}
			target := root + "?saved=1"
			if c.Number != "" {
				target += "&order=" + url.QueryEscape(c.Number)
			}
			http.Redirect(w, r, target, 303)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", 405)
			return
		}
		if evidence, _ := strconv.ParseInt(r.URL.Query().Get("evidence"), 10, 64); evidence > 0 {
			h.serveClaimEvidence(w, r, session.UserID, evidence)
			return
		}
		if slip := r.URL.Query().Get("packing_slip"); slip != "" {
			if view.Role == "customer" {
				http.Error(w, "Forbidden", 403)
				return
			}
			id, _ := strconv.ParseInt(slip, 10, 64)
			for _, pkg := range view.Packages {
				if pkg.ID == id {
					data, err := marketplacePackingSlipPDF(view.Number, pkg)
					if err != nil {
						http.Error(w, "Packing slip unavailable", 503)
						return
					}
					w.Header().Set("Content-Type", "application/pdf")
					w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=packing-slip-%d.pdf", id))
					w.Header().Set("X-Content-Type-Options", "nosniff")
					if r.Method != "HEAD" {
						_, _ = w.Write(data)
					}
					return
				}
			}
			http.NotFound(w, r)
			return
		}
		if invoice := r.URL.Query().Get("invoice"); invoice != "" {
			id, _ := strconv.ParseInt(invoice, 10, 64)
			documents := view.Invoices
			if r.URL.Query().Get("kind") == "credit" {
				documents = view.CreditNotes
			}
			for _, i := range documents {
				if i.ID == id {
					var d ports.InvoiceDocument
					if json.Unmarshal(i.Snapshot, &d) != nil {
						http.Error(w, "Invoice unavailable", 503)
						return
					}
					data := i.PDF
					hash := sha256.Sum256(data)
					err := error(nil)
					if fmt.Sprintf("%x", hash) != i.SHA256 || len(data) < 4 {
						err = ports.ErrInvalidState
					}
					if err != nil {
						http.Error(w, "Invoice unavailable", 503)
						return
					}
					w.Header().Set("Content-Type", "application/pdf")
					w.Header().Set("X-Content-Type-Options", "nosniff")
					w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=invoice-%d.pdf", id))
					if r.Method != "HEAD" {
						_, _ = w.Write(data)
					}
					return
				}
			}
			http.NotFound(w, r)
			return
		}
		keyBytes := make([]byte, 16)
		if _, err := rand.Read(keyBytes); err != nil {
			http.Error(w, "Request unavailable", 503)
			return
		}
		p := viewmodels.MarketplacePage{MarketplaceView: view, Root: root, Back: back, ActionPath: root + "/action", CSRFToken: session.CSRFToken, Key: hex.EncodeToString(keyBytes), ShippingConfigured: h.ShippingConfigured, BookingsEnabled: h.BookingsEnabled}
		if view.Role == "super_admin" && number == "" {
			before, _ := strconv.ParseInt(r.URL.Query().Get("setup_before"), 10, 64)
			p.Readiness, p.NextSetupID, e = h.Repository.ShippingReadiness(r.Context(), session.UserID, before)
			if e != nil {
				marketplaceHTTPError(w, e)
				return
			}
		}
		if r.URL.Query().Get("saved") == "1" {
			p.Notice = "Request saved. Payment and carrier operations update after provider confirmation."
		}
		ctx := templ.WithChildren(r.Context(), components.MarketplaceWorkspace(p))
		var document templ.Component
		switch view.Role {
		case "seller":
			document = sellerpages.Document(viewmodels.SellerAdminPage{Title: "Order fulfillment", Active: "orders", DisplayName: "Seller", CSRFToken: session.CSRFToken, CanOrders: true, CanProducts: true, CanInventory: true, CanReturns: true, CanReports: true})
		case "admin", "super_admin":
			label := "Admin"
			if view.Role == "super_admin" {
				label = "Super Admin"
			}
			document = adminpages.AdminDocument(viewmodels.AdminPage{Title: "Order fulfillment", Active: "orders", NavActive: "orders", PortalRoot: strings.TrimSuffix(root, "/fulfillment"), PortalLabel: label, RoleLabel: label, CSRFToken: session.CSRFToken, CanFinance: true, CanSecurity: view.Role == "super_admin", CanManageCatalog: true})
		default:
			document = pages.Document(viewmodels.CustomerPage{Title: "Order fulfillment", Route: "/orders", Brand: "WeevCrafts", Authenticated: true, IsCustomer: true, CSRFToken: session.CSRFToken})
		}
		if r.Method != "HEAD" {
			_ = document.Render(ctx, w)
		}
	})
}
func marketplaceHTTPError(w http.ResponseWriter, err error) {
	status, message := 503, "This operation is temporarily unavailable. Your saved request is preserved; check its status before retrying."
	switch {
	case errors.Is(err, ports.ErrForbidden):
		status, message = 403, "Forbidden"
	case errors.Is(err, ports.ErrOrderNotFound) || errors.Is(err, pgx.ErrNoRows):
		status, message = 404, "Record not found or unavailable for this account"
	case errors.Is(err, ports.ErrIdempotencyConflict):
		status, message = 409, "This request key was already used. Reload before making a different request."
	case errors.Is(err, ports.ErrInvalidState) || errors.Is(err, ports.ErrRefundState) || errors.Is(err, ports.ErrShippingInput):
		status, message = 409, "The request is unavailable at this stage. Check quantities, payment, package status and required configuration, then reload."
	}
	http.Error(w, message, status)
}
func MarketplaceInvoicePDF(d ports.InvoiceDocument) ([]byte, error) {
	font, err := assets.FS.ReadFile("fonts/dm-sans-400.ttf")
	if err != nil {
		return nil, err
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("DM", "", font)
	pdf.SetFont("DM", "", 10)
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()
	for _, line := range []string{d.Kind + " " + d.Number, "Related invoice: " + d.RelatedNumber, "Issued " + d.Date, "Seller: " + d.Seller.LegalName, d.Seller.Address, "GSTIN: " + d.Seller.GSTIN, "Bill to: " + d.CustomerName, d.CustomerAddress} {
		pdf.MultiCell(180, 6, line, "", "L", false)
	}
	pdf.Ln(5)
	for _, line := range d.Lines {
		pdf.MultiCell(180, 6, fmt.Sprintf("%s | SKU %s | HSN %s | Qty %d\nTaxable INR %.2f | Tax %.2f%% INR %.2f | Paid INR %.2f", line.Name, line.SKU, line.HSN, line.Quantity, float64(line.TaxableCents)/100, float64(line.RateBasisPoints)/100, float64(line.TaxCents)/100, float64(line.PaidCents)/100), "B", "L", false)
	}
	pdf.Ln(5)
	pdf.MultiCell(180, 7, fmt.Sprintf("Taxable INR %.2f\nCGST INR %.2f | SGST INR %.2f | IGST INR %.2f\nInvoice total INR %.2f", float64(d.TaxableCents)/100, float64(d.CGSTCents)/100, float64(d.SGSTCents)/100, float64(d.IGSTCents)/100, float64(d.TotalCents)/100), "", "L", false)
	pdf.MultiCell(180, 6, "Generated by WeevCrafts on behalf of the seller. Delivery charges, where applicable, are shown separately on the order payment receipt.", "", "L", false)
	var b bytes.Buffer
	err = pdf.Output(&b)
	return b.Bytes(), err
}

func marketplacePackingSlipPDF(number string, p ports.MarketplacePackage) ([]byte, error) {
	font, err := assets.FS.ReadFile("fonts/dm-sans-400.ttf")
	if err != nil {
		return nil, err
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("DM", "", font)
	pdf.SetFont("DM", "", 11)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()
	pdf.MultiCell(180, 8, fmt.Sprintf("Packing slip\nOrder %s\nPackage %d", number, p.ID), "", "L", false)
	for _, item := range p.Items {
		pdf.MultiCell(180, 8, fmt.Sprintf("%s | SKU %s | Quantity %d", item.Name, item.SKU, item.Quantity), "B", "L", false)
	}
	var b bytes.Buffer
	err = pdf.Output(&b)
	return b.Bytes(), err
}
