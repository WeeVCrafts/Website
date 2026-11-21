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
	CheckoutEnabled                     bool
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
		if h.serveCommerceSurface(w, r) {
			return
		}
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
		if root == "/seller-admin/fulfillment" && number == "" && r.URL.Query().Get("tab") == "setup" && (r.Method == "GET" || r.Method == "HEAD") {
			http.Redirect(w, r, "/seller-admin/setup", http.StatusSeeOther)
			return
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
		var setupValues url.Values
		if number != "" && (r.Method == "GET" || r.Method == "HEAD") && r.URL.Query().Get("invoice") == "" && r.URL.Query().Get("packing_slip") == "" && r.URL.Query().Get("evidence") == "" {
			tab := r.URL.Query().Get("tab")
			if tab == "" {
				tab = "packages"
			}
			target := canonicalCommerceURL(view.Role, number, tab)
			if r.URL.Query().Get("saved") == "1" {
				target += "&saved=1"
			}
			http.Redirect(w, r, target, http.StatusSeeOther)
			return
		}
		var setupErrors map[string]string
		setupAction := ""
		responseStatus := http.StatusOK
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
			if isMarketplaceSetupAction(f.Get("action")) {
				if view.Role != "super_admin" {
					http.Error(w, "Forbidden", 403)
					return
				}
				setupErrors = validateSetup(f.Get("action"), f)
				if f.Get("setup_context") != "" && (f.Get("action") == "save_invoice_profile" || f.Get("action") == "save_shipping_profile") && f.Get("seller_id") != setupPostContext(f).Get("seller_id") {
					setupErrors["seller_id"] = "Choose the seller again before saving."
				}
				if f.Get("setup_context") != "" && (f.Get("action") == "save_tax_profile" || f.Get("action") == "classify_variant" || f.Get("action") == "configure_pilot" || f.Get("action") == "save_setup_parcel" || f.Get("action") == "save_setup_tax" || f.Get("action") == "activate_pilot_seller") {
					q := setupPostContext(f)
					sid, _ := strconv.ParseInt(q.Get("seller_id"), 10, 64)
					vid, _ := strconv.ParseInt(f.Get("variant_id"), 10, 64)
					if sid <= 0 || vid <= 0 {
						setupErrors["variant_id"] = "Choose a seller and product option."
					} else if d, err := h.Repository.FulfillmentSetup(r.Context(), session.UserID, ports.MarketplaceSetupQuery{SellerID: sid, VariantID: vid}); err != nil {
						setupErrors["variant_id"] = "This product option does not belong to the selected seller."
					} else if f.Get("action") == "save_setup_tax" {
						if !d.InvoiceSaved {
							setupErrors["form"] = "Save seller invoice details before verifying tax and return policy."
						} else if rate, err := parseTaxPercent(f); err == nil && d.Invoice.Registration == "unregistered" && rate != 0 {
							setupErrors["tax_rate_percent"] = "An unregistered seller cannot collect GST. Use the verified applicable tax treatment."
						}
					}
				}
			}
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
			if len(setupErrors) == 0 {
				switch c.Action {
				case "upload_claim_evidence":
					if r.MultipartForm == nil || len(r.MultipartForm.File["evidence"]) != 1 {
						http.Error(w, "Choose one video or photo", 422)
						return
					}
					e = h.uploadClaimEvidence(r.Context(), session.UserID, c.CaseID, r.MultipartForm.File["evidence"][0])
				case "publish_policy", "pause_shipping", "configure_pilot", "classify_variant", "activate_ready_catalog", "activate_pilot_seller", "restore_limited_catalog":
					if f.Get("confirmed") != "yes" {
						http.Error(w, "Confirm the policy or shipping change", 422)
						return
					}
					if c.Action == "configure_pilot" && !h.BookingsEnabled {
						setupErrors["form"] = "New shipping bookings are paused in the application configuration. Product setup can still be saved."
						break
					}
					if (c.Action == "activate_ready_catalog" || c.Action == "activate_pilot_seller") && (!h.BookingsEnabled || !h.CheckoutEnabled || !h.ShippingConfigured) {
						setupErrors["form"] = "Enable production checkout and live Shiprocket bookings before expanding the catalog."
						break
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
						setupErrors["form"] = "New shipping bookings are paused in the application configuration."
						break
					}
					e = h.Repository.ConfigureLiveCatalog(r.Context(), session.UserID, num("variant_id"), num("second_variant_id"), c.Reason)
				case "save_shipping_profile":
					if view.Role != "super_admin" || h.PickupChecker == nil {
						setupErrors["form"] = "Live Shiprocket configuration is required before a pickup can be verified."
						break
					}
					locations, checkErr := h.PickupChecker.PickupPostcodes(r.Context())
					if checkErr != nil {
						e = checkErr
						break
					}
					pin, found := locations[f.Get("pickup_location")]
					if !found || f.Get("postal_code") != "" && pin != f.Get("postal_code") {
						e = ports.ErrShippingInput
						break
					}
					e = h.Repository.SaveShippingProfile(r.Context(), session.UserID, c.SellerID, f.Get("pickup_location"), pin, c.Reason)
				case "save_invoice_profile":
					if view.Role != "super_admin" {
						http.Error(w, "Forbidden", 403)
						return
					}
					e = h.Repository.SaveInvoiceProfile(r.Context(), session.UserID, ports.SellerInvoiceProfile{SellerID: c.SellerID, LegalName: f.Get("legal_name"), Address: f.Get("address"), StateCode: f.Get("state_code"), GSTIN: strings.ToUpper(f.Get("gstin")), Registration: f.Get("registration"), Prefix: f.Get("prefix")}, c.Reason)
				case "save_setup_parcel":
					q := setupPostContext(f)
					sid, _ := strconv.ParseInt(q.Get("seller_id"), 10, 64)
					e = h.Repository.SaveSetupParcel(r.Context(), session.UserID, sid, num("variant_id"), c.Parcel)
				case "save_setup_tax":
					q := setupPostContext(f)
					sid, _ := strconv.ParseInt(q.Get("seller_id"), 10, 64)
					rate, _ := parseTaxPercent(f)
					e = h.Repository.SaveSetupTaxAndClass(r.Context(), session.UserID, sid, num("variant_id"), f.Get("hsn"), rate, f.Get("return_class"), c.Reason)
				case "save_tax_profile":
					if view.Role != "super_admin" {
						http.Error(w, "Forbidden", 403)
						return
					}
					rate, _ := parseTaxPercent(f)
					e = h.Repository.SaveTaxProfile(r.Context(), session.UserID, num("variant_id"), strings.TrimSpace(f.Get("hsn")), rate, c.Reason)
				default:
					e = h.Repository.Act(r.Context(), session.UserID, c)
				}
			}
			if isMarketplaceSetupAction(c.Action) && (e != nil || len(setupErrors) > 0) {
				setupValues = f
				setupAction = c.Action
				responseStatus = http.StatusUnprocessableEntity
				if e != nil {
					setupErrors["form"] = "Configuration could not be saved. Check the selected records, prerequisites and provider availability, then retry."
				}
				r.URL.RawQuery = setupPostContext(f).Encode()
			} else if e != nil {
				marketplaceHTTPError(w, e)
				return
			} else {
				target := root + "?saved=1"
				if isMarketplaceSetupAction(c.Action) {
					q := setupPostContext(f)
					if f.Get("intent") == "continue" && (c.Action == "save_invoice_profile" || c.Action == "save_shipping_profile" || c.Action == "save_setup_parcel" || c.Action == "save_setup_tax") {
						q.Del("pickup_choices")
						next := viewmodels.MarketplacePage{SetupContext: q}
						if err := h.loadSetup(r.Context(), session.UserID, &next); err != nil {
							marketplaceHTTPError(w, err)
							return
						}
						if next.Setup.Query.VariantID > 0 {
							q.Set("step", next.FirstMissingStep())
						} else if c.Action == "save_invoice_profile" && !next.Setup.PickupReady {
							q.Set("step", "pickup")
						} else {
							q.Del("step")
						}
					}
					q.Set("saved", "1")
					q.Set("saved_action", c.Action)
					target = root + "?" + q.Encode()
				}
				if c.Number != "" {
					target += "&order=" + url.QueryEscape(c.Number)
					if f.Get("workspace_tab") != "" {
						target = canonicalCommerceURL(view.Role, c.Number, f.Get("workspace_tab")) + "&saved=1"
						if c.Action == "assign_case" || c.Action == "resolve_case" {
							target += "&case_id=" + fmt.Sprint(c.CaseID)
						}
						if f.Get("focus") != "" {
							target += "&focus=" + url.QueryEscape(f.Get("focus"))
						}
					}
				}
				http.Redirect(w, r, target, 303)
				return
			}
		}
		if r.Method != "GET" && r.Method != "HEAD" && setupValues == nil {
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
		p.CheckoutEnabled = h.CheckoutEnabled
		p.Tab = "operations"
		if view.Role == "super_admin" && number == "" && r.URL.Query().Get("tab") != "operations" {
			p.Tab = "setup"
			p.SetupContext = setupContext(r.URL.Query())
			p.SetupValues = setupValues
			p.SetupErrors = setupErrors
			p.SetupAction = setupAction
			if err := h.loadSetup(r.Context(), session.UserID, &p); err != nil {
				marketplaceHTTPError(w, err)
				return
			}
		}
		if view.Role == "super_admin" && number == "" {
			before, _ := strconv.ParseInt(r.URL.Query().Get("setup_before"), 10, 64)
			p.Readiness, p.NextSetupID, e = h.Repository.ShippingReadinessFiltered(r.Context(), session.UserID, before, func() int64 {
				if p.Setup != nil {
					return p.Setup.Query.SellerID
				}
				return 0
			}(), r.URL.Query().Get("product_q"))
			if e != nil {
				marketplaceHTTPError(w, e)
				return
			}
		}
		if r.URL.Query().Get("saved") == "1" {
			p.Notice = "Request saved. Payment and carrier operations update after provider confirmation."
			if p.Tab == "setup" {
				switch r.URL.Query().Get("saved_action") {
				case "save_invoice_profile":
					p.Notice = "Seller invoice details verified and saved."
				case "save_setup_parcel":
					p.Notice = "Packed measurements saved."
				case "save_setup_tax":
					p.Notice = "Tax and return policy verified and saved."
				case "save_tax_profile":
					p.Notice = "Product tax settings verified and saved."
				case "save_shipping_profile":
					p.Notice = "Seller pickup verified with Shiprocket and saved."
				case "classify_variant":
					p.Notice = "Product return classification saved."
				default:
					p.Notice = "Shipping policy configuration saved."
				}
			}
		}
		ctx := templ.WithChildren(r.Context(), components.MarketplaceWorkspace(p))
		var document templ.Component
		switch view.Role {
		case "seller":
			document = sellerpages.Document(viewmodels.SellerAdminPage{Title: "Order fulfillment", Active: "fulfillment", DisplayName: "Seller", CSRFToken: session.CSRFToken, CanOrders: true, CanProducts: true, CanInventory: true, CanReturns: true, CanReports: true})
		case "admin", "super_admin":
			label := "Admin"
			if view.Role == "super_admin" {
				label = "Super Admin"
			}
			document = adminpages.AdminDocument(viewmodels.AdminPage{GuidedFulfillment: p.Tab == "setup", Title: "Fulfillment · " + strings.Title(p.Tab), Active: "fulfillment", NavActive: "fulfillment", PortalRoot: strings.TrimSuffix(root, "/fulfillment"), PortalLabel: label, RoleLabel: label, CSRFToken: session.CSRFToken, CanFinance: true, CanSecurity: view.Role == "super_admin", CanManageCatalog: true})
		default:
			document = pages.Document(viewmodels.CustomerPage{Title: "Order fulfillment", Route: "/orders", Brand: "WeevCrafts", Authenticated: true, IsCustomer: true, CSRFToken: session.CSRFToken})
		}
		if r.Method != "HEAD" {
			if responseStatus != http.StatusOK {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(responseStatus)
			}
			_ = document.Render(ctx, w)
		}
	})
}
func marketplaceHTTPError(w http.ResponseWriter, err error) {
	status, message := 503, "This operation is temporarily unavailable. Your saved request is preserved; check its status before retrying."
	switch {
	case errors.Is(err, ports.ErrForbidden):
		status, message = 403, "Forbidden"
	case errors.Is(err, ports.ErrNotFound) || errors.Is(err, ports.ErrOrderNotFound) || errors.Is(err, pgx.ErrNoRows):
		status, message = 404, "Record not found or unavailable for this account"
	case errors.Is(err, ports.ErrIdempotencyConflict):
		status, message = 409, "This request key was already used. Reload before making a different request."
	case errors.Is(err, ports.ErrInvalidState) || errors.Is(err, ports.ErrRefundState) || errors.Is(err, ports.ErrShippingInput):
		status, message = 409, "The request is unavailable at this stage. Check quantities, payment, package status and required configuration, then reload."
	}
	http.Error(w, message, status)
}
func MarketplaceInvoicePDF(d ports.InvoiceDocument) ([]byte, error) {
	regular, err := assets.FS.ReadFile("fonts/dm-sans-400.ttf")
	if err != nil {
		return nil, err
	}
	medium, err := assets.FS.ReadFile("fonts/dm-sans-600.ttf")
	if err != nil {
		return nil, err
	}
	serif, err := assets.FS.ReadFile("fonts/playfair-display-600.ttf")
	if err != nil {
		serif, _ = assets.FS.ReadFile("fonts/cormorant-garamond-600.ttf")
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("DM", "", regular)
	pdf.AddUTF8FontFromBytes("DM", "B", medium)
	if len(serif) > 0 {
		pdf.AddUTF8FontFromBytes("Serif", "B", serif)
	} else {
		pdf.AddUTF8FontFromBytes("Serif", "B", medium)
	}

	pdf.SetMargins(14, 12, 14)
	pdf.SetAutoPageBreak(false, 18)
	pdf.AliasNbPages("")

	pageW, _ := pdf.GetPageSize()
	contentW := pageW - 28
	isCredit := strings.EqualFold(d.Kind, "Credit note")

	pdf.SetFooterFunc(func() {
		pdfSetDraw(pdf, pdfLine)
		pdf.SetLineWidth(0.3)
		pdf.Line(14, 281, 196, 281)

		pdfSetText(pdf, pdfForest)
		pdf.SetFont("DM", "B", 7.5)
		pdf.SetXY(14, 283)
		footerTitle := "WeevCrafts · GST-Compliant Seller Invoice"
		if isCredit {
			footerTitle = "WeevCrafts · Seller Credit Note"
		}
		pdf.CellFormat(90, 5, footerTitle, "", 0, "L", false, 0, "")

		pdfSetText(pdf, pdfMuted)
		pdf.SetFont("DM", "", 7)
		pdf.SetXY(105, 283)
		pdf.CellFormat(91, 5, fmt.Sprintf("%s · Page %d of {nb}", d.Number, pdf.PageNo()), "", 0, "R", false, 0, "")
	})

	pdf.AddPage()
	pdf.SetCompression(true)

	// Paper background
	pdfSetFill(pdf, pdfPaper)
	pdf.Rect(0, 0, pageW, 297, "F")

	// Logo
	if logo, readErr := assets.FS.ReadFile("images/brand/weevcrafts-logo.png"); readErr == nil {
		_ = pdf.RegisterImageOptionsReader("weevcrafts-logo", fpdf.ImageOptions{ImageType: "png", ReadDpi: true}, bytes.NewReader(logo))
		pdf.ImageOptions("weevcrafts-logo", 14, 11, 14, 14, false, fpdf.ImageOptions{ImageType: "png", ReadDpi: true}, 0, "")
	}

	sellerName := firstNonEmpty(d.Seller.LegalName, "Kamala Arts")
	pdfSetText(pdf, pdfForest)
	pdf.SetFont("Serif", "B", 17)
	pdf.SetXY(31, 13)
	pdf.CellFormat(75, 7, sellerName, "", 0, "L", false, 0, "")

	pdfSetText(pdf, pdfMuted)
	pdf.SetFont("DM", "B", 6.5)
	pdf.SetXY(31, 20)
	subTag := "REGISTERED SELLER ON WEEVCRAFTS MARKETPLACE"
	if isCredit {
		subTag = "CREDIT NOTE ISSUED VIA WEEVCRAFTS MARKETPLACE"
	}
	pdf.CellFormat(75, 4, subTag, "", 0, "L", false, 0, "")

	// Right Badge
	badgeW := 64.0
	badgeX := pageW - 14 - badgeW
	badgeColor := pdfNavy
	badgeText := "TAX INVOICE (RULE 46)"
	numColor := pdfInk
	if isCredit {
		badgeColor = pdfClay
		badgeText = "GST CREDIT NOTE"
		numColor = pdfClay
	} else if strings.EqualFold(d.Seller.Registration, "unregistered") {
		badgeText = "BILL OF SUPPLY"
	}
	pdfSetFill(pdf, badgeColor)
	pdf.RoundedRect(badgeX, 11, badgeW, 6.5, 1.5, "1234", "F")
	pdfSetText(pdf, "ffffff")
	pdf.SetFont("DM", "B", 7)
	pdf.SetXY(badgeX, 11)
	pdf.CellFormat(badgeW, 6.5, badgeText, "", 0, "C", false, 0, "")

	pdfSetText(pdf, numColor)
	pdf.SetFont("DM", "B", 9.5)
	pdf.SetXY(badgeX, 18.5)
	pdf.CellFormat(badgeW, 5, d.Number, "", 0, "R", false, 0, "")

	pdfSetText(pdf, pdfMuted)
	pdf.SetFont("DM", "", 7.5)
	pdf.SetXY(badgeX, 23.5)
	dateSub := d.Date + " · Original for Recipient"
	if isCredit {
		dateSub = d.Date + " · Sec 34 CGST Act"
	}
	pdf.CellFormat(badgeW, 4, dateSub, "", 0, "R", false, 0, "")

	// Divider line
	divColor := pdfForest
	if isCredit {
		divColor = pdfClay
	}
	pdfSetDraw(pdf, divColor)
	pdf.SetLineWidth(0.6)
	pdf.Line(14, 29, pageW-14, 29)

	curY := 33.0

	if isCredit {
		pdfSetText(pdf, pdfClay)
		pdf.SetFont("Serif", "B", 16)
		pdf.SetXY(14, curY)
		pdf.CellFormat(contentW, 7, "A credit has been issued.", "", 0, "L", false, 0, "")

		pdfSetText(pdf, pdfMuted)
		pdf.SetFont("DM", "", 8)
		pdf.SetXY(14, curY+7)
		pdf.CellFormat(contentW, 4, "Issued against invoice "+d.RelatedNumber+" for approved refund/return claim.", "", 0, "L", false, 0, "")
		curY += 13.0
	}

	// Status banner
	bannerTint := pdfForestTint
	bannerBdr := "cce2d6"
	pillText := "● GST COMPLIANT"
	pillColor := pdfForestMid
	if isCredit {
		bannerTint = pdfClayTint
		bannerBdr = "f1cfc6"
		pillText = "● REFUND COMPLETED"
		pillColor = pdfClay
	}
	pdfSetFill(pdf, bannerTint)
	pdfSetDraw(pdf, bannerBdr)
	pdf.SetLineWidth(0.3)
	pdf.RoundedRect(14, curY, contentW, 8.5, 2, "1234", "DF")

	pdfSetFill(pdf, "ffffff")
	pdfSetDraw(pdf, bannerBdr)
	pdf.RoundedRect(17, curY+2, 34, 4.5, 1, "1234", "DF")
	pdfSetText(pdf, pillColor)
	pdf.SetFont("DM", "B", 6.5)
	pdf.SetXY(17, curY+2)
	pdf.CellFormat(34, 4.5, pillText, "", 0, "C", false, 0, "")

	pdfSetText(pdf, pdfInk)
	pdf.SetFont("DM", "", 7.5)
	pdf.SetXY(54, curY+2)
	stateInfo := "Place of Supply: " + firstNonEmpty(d.CustomerStateCode, "Inter-State") + " · Reverse Charge: No"
	if isCredit {
		stateInfo = "Tax Liability Adjusted · Credited to Original Payment Source"
	}
	pdf.CellFormat(70, 4.5, stateInfo, "", 0, "L", false, 0, "")

	pdfSetText(pdf, pdfMuted)
	pdf.SetFont("DM", "", 7)
	pdf.SetXY(contentW-38, curY+2)
	refNote := "Against Invoice: " + firstNonEmpty(d.RelatedNumber, "N/A")
	if !isCredit {
		refNote = "Prepaid Order · Razorpay Verified"
	}
	pdf.CellFormat(50, 4.5, refNote, "", 0, "R", false, 0, "")

	curY += 11.5

	if isCredit {
		cardW := (contentW - 6) / 3
		cardH := 24.0

		// Card 1: Issued By
		pdfSetFill(pdf, pdfCardBg)
		pdfSetDraw(pdf, pdfCardBdr)
		pdf.RoundedRect(14, curY, cardW, cardH, 2, "1234", "DF")
		receiptLabel(pdf, "Issued By (Seller)", 17, curY+3, cardW-6)
		pdfSetText(pdf, pdfInk)
		pdf.SetFont("DM", "B", 8)
		pdf.SetXY(17, curY+7.5)
		pdf.CellFormat(cardW-6, 4, d.Seller.LegalName, "", 0, "L", false, 0, "")
		pdfSetText(pdf, pdfMuted)
		pdf.SetFont("DM", "", 6.8)
		pdf.SetXY(17, curY+11.5)
		pdf.MultiCell(cardW-6, 3.1, fmt.Sprintf("%s\nGSTIN: %s | State: %s", d.Seller.Address, d.Seller.GSTIN, d.Seller.StateCode), "", "L", false)

		// Card 2: Issued To
		pdfSetFill(pdf, pdfCardBg)
		pdfSetDraw(pdf, pdfCardBdr)
		pdf.RoundedRect(14+cardW+3, curY, cardW, cardH, 2, "1234", "DF")
		receiptLabel(pdf, "Issued To (Buyer)", 17+cardW+3, curY+3, cardW-6)
		pdfSetText(pdf, pdfInk)
		pdf.SetFont("DM", "B", 8)
		pdf.SetXY(17+cardW+3, curY+7.5)
		pdf.CellFormat(cardW-6, 4, d.CustomerName, "", 0, "L", false, 0, "")
		pdfSetText(pdf, pdfMuted)
		pdf.SetFont("DM", "", 6.8)
		pdf.SetXY(17+cardW+3, curY+11.5)
		pdf.MultiCell(cardW-6, 3.1, fmt.Sprintf("%s\nState Code: %s", d.CustomerAddress, d.CustomerStateCode), "", "L", false)

		// Card 3: Reference
		pdfSetFill(pdf, pdfCardBg)
		pdfSetDraw(pdf, pdfCardBdr)
		pdf.RoundedRect(14+(cardW+3)*2, curY, cardW, cardH, 2, "1234", "DF")
		receiptLabel(pdf, "Reference Documents", 17+(cardW+3)*2, curY+3, cardW-6)
		pdfSetText(pdf, pdfInk)
		pdf.SetFont("DM", "B", 8)
		pdf.SetXY(17+(cardW+3)*2, curY+7.5)
		pdf.CellFormat(cardW-6, 4, "INV: "+d.RelatedNumber, "", 0, "L", false, 0, "")
		pdfSetText(pdf, pdfMuted)
		pdf.SetFont("DM", "", 6.8)
		pdf.SetXY(17+(cardW+3)*2, curY+11.5)
		pdf.MultiCell(cardW-6, 3.1, fmt.Sprintf("Original Date: %s\nAdjustment: Return Settled", d.Date), "", "L", false)

		curY += cardH + 5
	} else {
		cardW := (contentW - 4) / 2
		cardH := 25.0

		// Card 1: Supplier
		pdfSetFill(pdf, pdfCardBg)
		pdfSetDraw(pdf, pdfCardBdr)
		pdf.RoundedRect(14, curY, cardW, cardH, 2, "1234", "DF")
		receiptLabel(pdf, "Sold By / Supplier (Seller)", 17, curY+3, cardW-6)
		pdfSetText(pdf, pdfInk)
		pdf.SetFont("DM", "B", 8.5)
		pdf.SetXY(17, curY+7.5)
		pdf.CellFormat(cardW-6, 4, d.Seller.LegalName, "", 0, "L", false, 0, "")
		pdfSetText(pdf, pdfMuted)
		pdf.SetFont("DM", "", 7)
		pdf.SetXY(17, curY+12)
		pdf.MultiCell(cardW-6, 3.2, fmt.Sprintf("%s\nGSTIN: %s | State Code: %s", d.Seller.Address, d.Seller.GSTIN, d.Seller.StateCode), "", "L", false)

		// Card 2: Recipient
		pdfSetFill(pdf, pdfCardBg)
		pdfSetDraw(pdf, pdfCardBdr)
		pdf.RoundedRect(14+cardW+4, curY, cardW, cardH, 2, "1234", "DF")
		receiptLabel(pdf, "Billed To & Shipped To (Buyer)", 17+cardW+4, curY+3, cardW-6)
		pdfSetText(pdf, pdfInk)
		pdf.SetFont("DM", "B", 8.5)
		pdf.SetXY(17+cardW+4, curY+7.5)
		pdf.CellFormat(cardW-6, 4, d.CustomerName, "", 0, "L", false, 0, "")
		pdfSetText(pdf, pdfMuted)
		pdf.SetFont("DM", "", 7)
		pdf.SetXY(17+cardW+4, curY+12)
		pdf.MultiCell(cardW-6, 3.2, fmt.Sprintf("%s\nPlace of Supply: %s", d.CustomerAddress, d.CustomerStateCode), "", "L", false)

		curY += cardH + 5
	}

	// Table Header
	tableY := curY
	pdfSetFill(pdf, pdfCardBg)
	pdfSetDraw(pdf, pdfCardBdr)
	pdf.RoundedRect(14, tableY, contentW, 7, 1.5, "1234", "DF")

	receiptLabel(pdf, "#  Description of Goods / Services", 16, tableY+2, 76)
	pdfSetText(pdf, pdfForest)
	pdf.SetFont("DM", "B", 6.8)
	pdf.SetXY(92, tableY+2)
	pdf.CellFormat(16, 4, "HSN", "", 0, "C", false, 0, "")
	pdf.SetXY(108, tableY+2)
	pdf.CellFormat(12, 4, "QTY", "", 0, "C", false, 0, "")
	pdf.SetXY(120, tableY+2)
	pdf.CellFormat(22, 4, "TAXABLE", "", 0, "R", false, 0, "")
	pdf.SetXY(142, tableY+2)
	pdf.CellFormat(20, 4, "GST", "", 0, "R", false, 0, "")
	pdf.SetXY(162, tableY+2)
	pdf.CellFormat(32, 4, "TOTAL", "", 0, "R", false, 0, "")

	rowY := tableY + 7
	for i, line := range d.Lines {
		pdf.SetFont("DM", "B", 8)
		nameLines := pdf.SplitText(line.Name, 72)
		skuText := "SKU: " + line.SKU
		rowH := 6 + float64(len(nameLines))*3.8 + 3.2
		if rowH < 14 {
			rowH = 14
		}

		if rowY+rowH > 268 {
			pdf.AddPage()
			receiptLabel(pdf, d.Number+" · continued", 14, 16, 182)
			rowY = 24
		}

		pdfSetDraw(pdf, pdfLine)
		pdf.SetLineWidth(0.2)
		pdf.Line(14, rowY+rowH, pageW-14, rowY+rowH)

		// Item #
		pdfSetText(pdf, pdfMuted)
		pdf.SetFont("DM", "", 7.5)
		pdf.SetXY(15, rowY+2)
		pdf.CellFormat(6, 4, fmt.Sprintf("%d", i+1), "", 0, "L", false, 0, "")

		// Item Name & SKU
		pdfSetText(pdf, pdfInk)
		pdf.SetFont("DM", "B", 8)
		pdf.SetXY(21, rowY+2)
		pdf.MultiCell(70, 3.8, line.Name, "", "L", false)

		pdfSetText(pdf, pdfMuted)
		pdf.SetFont("DM", "", 6.5)
		pdf.SetXY(21, rowY+2.5+float64(len(nameLines))*3.8)
		pdf.CellFormat(70, 3, skuText, "", 0, "L", false, 0, "")

		// HSN & Qty
		pdfSetText(pdf, pdfInk)
		pdf.SetFont("DM", "", 7.5)
		pdf.SetXY(92, rowY+3)
		pdf.CellFormat(16, 5, line.HSN, "", 0, "C", false, 0, "")
		pdf.SetXY(108, rowY+3)
		pdf.CellFormat(12, 5, fmt.Sprintf("%d", line.Quantity), "", 0, "C", false, 0, "")

		// Taxable & GST
		pdf.SetXY(120, rowY+3)
		pdf.CellFormat(22, 5, fmt.Sprintf("₹%.2f", float64(line.TaxableCents)/100), "", 0, "R", false, 0, "")

		taxPct := float64(line.RateBasisPoints) / 100
		pdf.SetXY(142, rowY+3)
		pdf.CellFormat(20, 5, fmt.Sprintf("₹%.2f (%.1f%%)", float64(line.TaxCents)/100, taxPct), "", 0, "R", false, 0, "")

		// Total
		pdf.SetFont("DM", "B", 8)
		if isCredit {
			pdfSetText(pdf, pdfClay)
		} else {
			pdfSetText(pdf, pdfInk)
		}
		pdf.SetXY(162, rowY+3)
		pdf.CellFormat(32, 5, fmt.Sprintf("₹%.2f", float64(line.PaidCents)/100), "", 0, "R", false, 0, "")

		rowY += rowH
	}

	// Bottom totals and signature
	if rowY+65 > 270 {
		pdf.AddPage()
		receiptLabel(pdf, d.Number+" · totals", 14, 16, 182)
		rowY = 24
	}

	blockY := rowY + 5

	// Left: Declaration & Words
	leftW := 102.0
	pdfSetFill(pdf, pdfCardBg)
	pdfSetDraw(pdf, pdfCardBdr)
	pdf.RoundedRect(14, blockY, leftW, 14, 2, "1234", "DF")
	receiptLabel(pdf, "Amount in Words", 17, blockY+2.5, leftW-6)
	pdfSetText(pdf, pdfInk)
	pdf.SetFont("DM", "B", 7)
	pdf.SetXY(17, blockY+6.5)
	pdf.MultiCell(leftW-6, 3.2, pdfAmountInWords(d.TotalCents), "", "L", false)

	// Statutory notice
	pdfSetFill(pdf, pdfCardBg)
	pdfSetDraw(pdf, pdfCardBdr)
	pdf.RoundedRect(14, blockY+17, leftW, 20, 2, "1234", "DF")
	noticeAccent := pdfGold
	if isCredit {
		noticeAccent = pdfClay
	}
	pdfSetDraw(pdf, noticeAccent)
	pdf.SetLineWidth(0.8)
	pdf.Line(14, blockY+17, 14, blockY+37)

	receiptLabel(pdf, "Statutory Declaration", 17, blockY+19, leftW-6)
	pdfSetText(pdf, pdfMuted)
	pdf.SetFont("DM", "", 6.5)
	pdf.SetXY(17, blockY+23.5)
	declText := "We declare that this invoice shows the actual price of the goods described and that all particulars are true and correct. Handcrafted items may exhibit subtle variations in finish."
	if isCredit {
		declText = "This credit note adjusts the taxable turnover and output tax of referenced invoice " + d.RelatedNumber + " under Section 34 of the CGST Act. Refund settled to customer."
	}
	pdf.MultiCell(leftW-6, 3.1, declText, "", "L", false)

	// Right: Ledger Box
	ledgerX := 120.0
	ledgerW := contentW - 106.0
	pdfSetFill(pdf, pdfCardBg)
	pdfSetDraw(pdf, pdfCardBdr)
	pdf.RoundedRect(ledgerX, blockY, ledgerW, 23, 2, "1234", "DF")

	lRows := []struct{ l, v string }{
		{"Taxable Value", fmt.Sprintf("₹%.2f", float64(d.TaxableCents)/100)},
	}
	if d.CGSTCents > 0 || d.SGSTCents > 0 {
		lRows = append(lRows, struct{ l, v string }{"CGST", fmt.Sprintf("₹%.2f", float64(d.CGSTCents)/100)}, struct{ l, v string }{"SGST", fmt.Sprintf("₹%.2f", float64(d.SGSTCents)/100)})
	} else {
		lRows = append(lRows, struct{ l, v string }{"IGST", fmt.Sprintf("₹%.2f", float64(d.IGSTCents)/100)})
	}
	lY := blockY + 2.5
	for _, lr := range lRows {
		pdfSetText(pdf, pdfMuted)
		pdf.SetFont("DM", "", 7)
		pdf.SetXY(ledgerX+4, lY)
		pdf.CellFormat(36, 4, lr.l, "", 0, "L", false, 0, "")
		pdfSetText(pdf, pdfInk)
		pdf.SetXY(ledgerX+ledgerW-34, lY)
		pdf.CellFormat(30, 4, lr.v, "", 0, "R", false, 0, "")
		lY += 4.5
	}

	// Total Container
	totY := blockY + 25
	totBg := pdfForest
	totLbl := "INVOICE TOTAL"
	if isCredit {
		totBg = pdfClay
		totLbl = "TOTAL CREDITED"
	}
	pdfSetFill(pdf, totBg)
	pdf.RoundedRect(ledgerX, totY, ledgerW, 12, 2, "1234", "F")
	pdfSetText(pdf, "ffffff")
	pdf.SetFont("DM", "B", 7.5)
	pdf.SetXY(ledgerX+4, totY+3.5)
	pdf.CellFormat(36, 5, totLbl, "", 0, "L", false, 0, "")
	pdf.SetFont("DM", "B", 11)
	pdf.SetXY(ledgerX+ledgerW-38, totY+3.5)
	pdf.CellFormat(34, 5, fmt.Sprintf("₹%.2f", float64(d.TotalCents)/100), "", 0, "R", false, 0, "")

	// Signature Section
	signY := blockY + 41
	pdfSetText(pdf, pdfMuted)
	pdf.SetFont("DM", "", 6.5)
	pdf.SetXY(14, signY+4)
	pdf.MultiCell(102, 3, "This is a computer-generated document issued by the artisan seller through WeevCrafts Marketplace (Facilitator GSTIN: 29AABCW9821K1ZM).", "", "L", false)

	pdfSetDraw(pdf, pdfLineStrong)
	pdf.SetLineWidth(0.3)
	pdf.Line(ledgerX+12, signY+6, ledgerX+ledgerW-2, signY+6)
	pdf.SetFont("DM", "B", 6.5)
	pdf.SetXY(ledgerX+12, signY+7)
	pdf.CellFormat(ledgerW-14, 3.5, "AUTHORISED SIGNATORY", "", 0, "R", false, 0, "")
	pdfSetText(pdf, pdfForest)
	pdf.SetFont("DM", "", 6.5)
	pdf.SetXY(ledgerX+12, signY+10.5)
	pdf.CellFormat(ledgerW-14, 3.5, "For "+d.Seller.LegalName, "", 0, "R", false, 0, "")

	var b bytes.Buffer
	err = pdf.Output(&b)
	return b.Bytes(), err
}

func marketplacePackingSlipPDF(number string, p ports.MarketplacePackage) ([]byte, error) {
	regular, err := assets.FS.ReadFile("fonts/dm-sans-400.ttf")
	if err != nil {
		return nil, err
	}
	medium, err := assets.FS.ReadFile("fonts/dm-sans-600.ttf")
	if err != nil {
		return nil, err
	}
	serif, err := assets.FS.ReadFile("fonts/playfair-display-600.ttf")
	if err != nil {
		serif, _ = assets.FS.ReadFile("fonts/cormorant-garamond-600.ttf")
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("DM", "", regular)
	pdf.AddUTF8FontFromBytes("DM", "B", medium)
	if len(serif) > 0 {
		pdf.AddUTF8FontFromBytes("Serif", "B", serif)
	} else {
		pdf.AddUTF8FontFromBytes("Serif", "B", medium)
	}

	pdf.SetMargins(14, 12, 14)
	pdf.SetAutoPageBreak(false, 18)
	pdf.AliasNbPages("")

	pageW, _ := pdf.GetPageSize()
	contentW := pageW - 28

	pdf.SetFooterFunc(func() {
		pdfSetDraw(pdf, pdfLine)
		pdf.SetLineWidth(0.3)
		pdf.Line(14, 281, 196, 281)

		pdfSetText(pdf, pdfForest)
		pdf.SetFont("DM", "B", 7.5)
		pdf.SetXY(14, 283)
		pdf.CellFormat(90, 5, "WeevCrafts · Fulfilment & Dispatch Slip", "", 0, "L", false, 0, "")

		pdfSetText(pdf, pdfMuted)
		pdf.SetFont("DM", "", 7)
		pdf.SetXY(105, 283)
		pdf.CellFormat(91, 5, fmt.Sprintf("Order %s · Page %d of {nb}", number, pdf.PageNo()), "", 0, "R", false, 0, "")
	})

	pdf.AddPage()
	pdf.SetCompression(true)

	// Paper Background
	pdfSetFill(pdf, pdfPaper)
	pdf.Rect(0, 0, pageW, 297, "F")

	// Logo
	if logo, readErr := assets.FS.ReadFile("images/brand/weevcrafts-logo.png"); readErr == nil {
		_ = pdf.RegisterImageOptionsReader("weevcrafts-logo", fpdf.ImageOptions{ImageType: "png", ReadDpi: true}, bytes.NewReader(logo))
		pdf.ImageOptions("weevcrafts-logo", 14, 11, 14, 14, false, fpdf.ImageOptions{ImageType: "png", ReadDpi: true}, 0, "")
	}

	pdfSetText(pdf, pdfForest)
	pdf.SetFont("Serif", "B", 18)
	pdf.SetXY(31, 13)
	pdf.CellFormat(70, 7, "WeevCrafts", "", 0, "L", false, 0, "")

	pdfSetText(pdf, pdfMuted)
	pdf.SetFont("DM", "B", 6.5)
	pdf.SetXY(31, 20)
	pdf.CellFormat(70, 4, "LOGISTICS & FULFILMENT HUB", "", 0, "L", false, 0, "")

	// Right Badge
	pkgRef := fmt.Sprintf("PKG-%s-%04d", number, p.ID)
	badgeW := 64.0
	badgeX := pageW - 14 - badgeW
	pdfSetFill(pdf, pdfSlate)
	pdf.RoundedRect(badgeX, 11, badgeW, 6.5, 1.5, "1234", "F")
	pdfSetText(pdf, "ffffff")
	pdf.SetFont("DM", "B", 7)
	pdf.SetXY(badgeX, 11)
	pdf.CellFormat(badgeW, 6.5, "PACKAGE PACKING SLIP", "", 0, "C", false, 0, "")

	pdfSetText(pdf, pdfInk)
	pdf.SetFont("DM", "B", 9)
	pdf.SetXY(badgeX, 18.5)
	pdf.CellFormat(badgeW, 5, pkgRef, "", 0, "R", false, 0, "")

	pdfSetText(pdf, pdfMuted)
	pdf.SetFont("DM", "", 7.5)
	pdf.SetXY(badgeX, 23.5)
	pdf.CellFormat(badgeW, 4, "Package 1 of 1 · Ready to Pack", "", 0, "R", false, 0, "")

	// Divider line
	pdfSetDraw(pdf, pdfSlate)
	pdf.SetLineWidth(0.6)
	pdf.Line(14, 29, pageW-14, 29)

	// Status banner
	curY := 33.0
	pdfSetFill(pdf, "f1f5f9")
	pdfSetDraw(pdf, "cbd5e1")
	pdf.SetLineWidth(0.3)
	pdf.RoundedRect(14, curY, contentW, 8.5, 2, "1234", "DF")

	pdfSetFill(pdf, "ffffff")
	pdfSetDraw(pdf, "cbd5e1")
	pdf.RoundedRect(17, curY+2, 34, 4.5, 1, "1234", "DF")
	pdfSetText(pdf, pdfSlate)
	pdf.SetFont("DM", "B", 6.5)
	pdf.SetXY(17, curY+2)
	pdf.CellFormat(34, 4.5, "● READY FOR DISPATCH", "", 0, "C", false, 0, "")

	carrier := firstNonEmpty(p.Courier, "Blue Dart Express / Shiprocket")
	pdfSetText(pdf, pdfInk)
	pdf.SetFont("DM", "", 7.5)
	pdf.SetXY(54, curY+2)
	pdf.CellFormat(70, 4.5, "Carrier: "+carrier, "", 0, "L", false, 0, "")

	pdfSetText(pdf, pdfMuted)
	pdf.SetFont("DM", "", 7)
	pdf.SetXY(contentW-38, curY+2)
	pdf.CellFormat(50, 4.5, "Order Ref: "+number, "", 0, "R", false, 0, "")

	curY += 12.0

	// Summary Cards Grid (3 columns)
	cardW := (contentW - 6) / 3
	cardH := 25.0

	// Card 1: Ship To
	pdfSetFill(pdf, pdfCardBg)
	pdfSetDraw(pdf, pdfCardBdr)
	pdf.RoundedRect(14, curY, cardW, cardH, 2, "1234", "DF")
	receiptLabel(pdf, "Ship To (Destination)", 17, curY+3, cardW-6)
	pdfSetText(pdf, pdfInk)
	pdf.SetFont("DM", "B", 8)
	pdf.SetXY(17, curY+7.5)
	pdf.CellFormat(cardW-6, 4, "Customer Destination", "", 0, "L", false, 0, "")
	pdfSetText(pdf, pdfMuted)
	pdf.SetFont("DM", "", 7)
	pdf.SetXY(17, curY+12)
	destText := firstNonEmpty(p.DeliveryPostalCode, "PIN Code: Verified Delivery Destination")
	pdf.MultiCell(cardW-6, 3.2, destText, "", "L", false)

	// Card 2: Dispatched From
	pdfSetFill(pdf, pdfCardBg)
	pdfSetDraw(pdf, pdfCardBdr)
	pdf.RoundedRect(14+cardW+3, curY, cardW, cardH, 2, "1234", "DF")
	receiptLabel(pdf, "Dispatched From", 17+cardW+3, curY+3, cardW-6)
	pdfSetText(pdf, pdfInk)
	pdf.SetFont("DM", "B", 8)
	pdf.SetXY(17+cardW+3, curY+7.5)
	pdf.CellFormat(cardW-6, 4, "Artisan Dispatch Studio", "", 0, "L", false, 0, "")
	pdfSetText(pdf, pdfMuted)
	pdf.SetFont("DM", "", 7)
	pdf.SetXY(17+cardW+3, curY+12)
	pdf.MultiCell(cardW-6, 3.2, "Handcrafted Fulfillment Station\nVerified Maker Community", "", "L", false)

	// Card 3: Waybill Barcode
	pdfSetFill(pdf, pdfCardBg)
	pdfSetDraw(pdf, pdfCardBdr)
	pdf.RoundedRect(14+(cardW+3)*2, curY, cardW, cardH, 2, "1234", "DF")
	receiptLabel(pdf, "Courier Waybill Scan", 17+(cardW+3)*2, curY+3, cardW-6)
	awbText := firstNonEmpty(p.AWB, fmt.Sprintf("AWB-%d", p.ID))
	pdfDrawBarcode(pdf, awbText, 17+(cardW+3)*2, curY+7, cardW-6, 15)

	curY += cardH + 5

	// Items Table (NO PRICING SHOWN)
	tableY := curY
	pdfSetFill(pdf, pdfCardBg)
	pdfSetDraw(pdf, pdfCardBdr)
	pdf.RoundedRect(14, tableY, contentW, 7, 1.5, "1234", "DF")

	pdfSetText(pdf, pdfForest)
	pdf.SetFont("DM", "B", 7)
	pdf.SetXY(16, tableY+2)
	pdf.CellFormat(12, 4, "CHECK", "", 0, "C", false, 0, "")
	pdf.SetXY(30, tableY+2)
	pdf.CellFormat(96, 4, "ITEM DESCRIPTION & SKU", "", 0, "L", false, 0, "")
	pdf.SetXY(128, tableY+2)
	pdf.CellFormat(16, 4, "QTY", "", 0, "C", false, 0, "")
	pdf.SetXY(146, tableY+2)
	pdf.CellFormat(46, 4, "PACKAGING SPECIFICATION", "", 0, "L", false, 0, "")

	rowY := tableY + 7
	for _, item := range p.Items {
		pdf.SetFont("DM", "B", 8.5)
		nameLines := pdf.SplitText(item.Name, 90)
		rowH := 6 + float64(len(nameLines))*4 + 3.5
		if rowH < 14 {
			rowH = 14
		}

		if rowY+rowH > 268 {
			pdf.AddPage()
			receiptLabel(pdf, "Order "+number+" · packing continued", 14, 16, 182)
			rowY = 24
		}

		pdfSetDraw(pdf, pdfLine)
		pdf.SetLineWidth(0.2)
		pdf.Line(14, rowY+rowH, pageW-14, rowY+rowH)

		// Checkbox square
		pdfSetDraw(pdf, pdfForest)
		pdf.SetLineWidth(0.3)
		pdf.RoundedRect(19, rowY+3, 5, 5, 0.5, "1234", "D")
		pdfSetText(pdf, pdfForest)
		pdf.SetFont("DM", "B", 7)
		pdf.SetXY(19, rowY+3)
		pdf.CellFormat(5, 5, "✓", "", 0, "C", false, 0, "")

		// Item Name & SKU
		pdfSetText(pdf, pdfInk)
		pdf.SetFont("DM", "B", 8)
		pdf.SetXY(30, rowY+2.5)
		pdf.MultiCell(90, 4, item.Name, "", "L", false)

		pdfSetText(pdf, pdfMuted)
		pdf.SetFont("DM", "", 6.8)
		pdf.SetXY(30, rowY+3+float64(len(nameLines))*4)
		pdf.CellFormat(90, 3, "SKU: "+item.SKU, "", 0, "L", false, 0, "")

		// Qty
		pdfSetText(pdf, pdfInk)
		pdf.SetFont("DM", "B", 9)
		pdf.SetXY(128, rowY+4)
		pdf.CellFormat(16, 5, fmt.Sprintf("%d", item.Quantity), "", 0, "C", false, 0, "")

		// Packaging spec
		pdfSetText(pdf, pdfMuted)
		pdf.SetFont("DM", "", 7)
		pdf.SetXY(146, rowY+4)
		pdf.CellFormat(46, 5, "Inspected & securely packaged", "", 0, "L", false, 0, "")

		rowY += rowH
	}

	// Sign-off cards
	if rowY+55 > 270 {
		pdf.AddPage()
		receiptLabel(pdf, "Order "+number+" · packing sign-off", 14, 16, 182)
		rowY = 24
	}

	benchY := rowY + 5
	bCardW := (contentW - 4) / 2
	bCardH := 24.0

	// Quality sign-off
	pdfSetFill(pdf, pdfCardBg)
	pdfSetDraw(pdf, pdfCardBdr)
	pdf.RoundedRect(14, benchY, bCardW, bCardH, 2, "1234", "DF")
	receiptLabel(pdf, "Packing Bench Quality Sign-Off", 17, benchY+3, bCardW-6)
	pdfSetText(pdf, pdfMuted)
	pdf.SetFont("DM", "", 7)
	pdf.SetXY(17, benchY+7.5)
	pdf.MultiCell(bCardW-6, 3.3, "Packed By: Artisan Dispatch Team\nInspection: 100% Item & Quality Verified\nSecurity: Sealed with tamper-evident tape", "", "L", false)

	// Care advisory
	pdfSetFill(pdf, pdfCardBg)
	pdfSetDraw(pdf, pdfCardBdr)
	pdf.RoundedRect(14+bCardW+4, benchY, bCardW, bCardH, 2, "1234", "DF")
	pdfSetDraw(pdf, pdfForest)
	pdf.SetLineWidth(0.8)
	pdf.Line(14+bCardW+4, benchY, 14+bCardW+4, benchY+bCardH)
	receiptLabel(pdf, "Artisan Care & Handling Advisory", 17+bCardW+4, benchY+3, bCardW-6)
	pdfSetText(pdf, pdfInk)
	pdf.SetFont("DM", "", 6.8)
	pdf.SetXY(17+bCardW+4, benchY+7.5)
	pdf.MultiCell(bCardW-6, 3.2, "Handcrafted items require gentle unboxing. Please allow artworks to settle on a clean, flat surface. Keep natural dyed fabrics away from damp or direct moisture.", "", "L", false)

	// Inspection Notice
	noticeY := benchY + bCardH + 4
	pdfSetFill(pdf, pdfCardBg)
	pdfSetDraw(pdf, pdfCardBdr)
	pdf.RoundedRect(14, noticeY, contentW, 14, 2, "1234", "DF")
	pdfSetDraw(pdf, pdfClay)
	pdf.SetLineWidth(0.8)
	pdf.Line(14, noticeY, 14, noticeY+14)

	receiptLabel(pdf, "Delivery Inspection Notice", 17, noticeY+2.5, contentW-6)
	pdfSetText(pdf, pdfMuted)
	pdf.SetFont("DM", "", 6.8)
	pdf.SetXY(17, noticeY+6.5)
	pdf.MultiCell(contentW-6, 3.1, "Please inspect the outer package before accepting delivery. If you observe crushing, tearing or tampering, photograph the parcel immediately and report to WeevCrafts within 7 days for priority resolution.", "", "L", false)

	var b bytes.Buffer
	err = pdf.Output(&b)
	return b.Bytes(), err
}
