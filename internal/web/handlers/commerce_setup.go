package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/components"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (h *MarketplaceHandler) serveCommerceSetup(w http.ResponseWriter, r *http.Request, actor int64, csrf string, a ports.CommerceActor) {
	review := strings.HasPrefix(r.URL.Path, "/super-admin/")
	if review && a.Role != "super_admin" || !review && (a.Role != "seller" || a.SellerID <= 0) {
		http.Error(w, "Forbidden", 403)
		return
	}
	p := viewmodels.CommerceSetupPage{Actor: a, CSRFToken: csrf, Root: r.URL.Path, Errors: map[string]string{}}
	if r.Method == "POST" {
		r.Body = http.MaxBytesReader(w, r.Body, 32768)
		if !commerceSafePost(r, csrf) {
			http.Error(w, "Invalid request or session. Reload before saving.", 403)
			return
		}
	}
	if !review && r.URL.Query().Get("kind") == "parcel" {
		h.serveSellerParcelSetup(w, r, actor, csrf, a)
		return
	}
	id := commerceSubmissionID(r.URL.Path)
	if !review {
		id, _ = strconv.ParseInt(r.URL.Query().Get("submission_id"), 10, 64)
		if r.Method == "POST" {
			id, _ = strconv.ParseInt(r.PostForm.Get("submission_id"), 10, 64)
		}
	}
	if review && r.URL.Path != "/super-admin/fulfillment/reviews" && id <= 0 {
		http.NotFound(w, r)
		return
	}
	var err error
	if id > 0 {
		var list []ports.SetupSubmission
		list, _, err = h.Repository.SetupSubmissions(r.Context(), actor, id, 0)
		if err != nil {
			marketplaceHTTPError(w, err)
			return
		}
		if len(list) != 1 {
			http.NotFound(w, r)
			return
		}
		p.Selected = &list[0]
		p.Kind = p.Selected.Kind
		p.VariantID = p.Selected.VariantID
		p.Data = decodeSubmissionData(p.Selected)
	}
	if !review {
		before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
		selectedVariant := p.VariantID
		if selectedVariant == 0 {
			selectedVariant, _ = strconv.ParseInt(r.URL.Query().Get("variant_id"), 10, 64)
		}
		p.Rows, p.Next, err = h.Repository.SellerReadiness(r.Context(), actor, before, selectedVariant)
		if err != nil {
			marketplaceHTTPError(w, err)
			return
		}
		if p.Selected == nil {
			p.Kind = r.URL.Query().Get("kind")
			p.VariantID, _ = strconv.ParseInt(r.URL.Query().Get("variant_id"), 10, 64)
			if p.Kind == "tax" && p.VariantID <= 0 {
				http.Error(w, "Choose a product option", 400)
				return
			}
			if p.Kind != "" && p.Kind != "invoice" && p.Kind != "tax" {
				http.NotFound(w, r)
				return
			}
			if p.Kind != "" {
				p.Selected, err = h.Repository.ActiveSetupSubmission(r.Context(), actor, p.Kind, func() int64 {
					if p.Kind == "tax" {
						return p.VariantID
					}
					return 0
				}())
				if err != nil {
					marketplaceHTTPError(w, err)
					return
				}
				if p.Selected != nil {
					p.Data = decodeSubmissionData(p.Selected)
				} else {
					p.Data, p.CurrentInvoice, err = h.Repository.SetupDefaults(r.Context(), actor, a.SellerID, p.VariantID, p.Kind)
					if err != nil {
						marketplaceHTTPError(w, err)
						return
					}
				}
			}
		}
	}
	if review && p.Selected != nil {
		p.CurrentData, p.CurrentInvoice, err = h.Repository.SetupDefaults(r.Context(), actor, p.Selected.SellerID, p.Selected.VariantID, p.Selected.Kind)
		if err != nil {
			marketplaceHTTPError(w, err)
			return
		}
	}
	if r.Method == "POST" {
		f := r.PostForm
		p.Values = f
		if review && p.Selected == nil && f.Get("decision") == "approve_matching_tax" {
			if f.Get("confirmed") != "yes" {
				p.Error = "Confirm that you verified the shared tax and return details for every option."
			} else {
				revisions := matchingTaxRevisions(f["review"])
				err = h.Repository.ApproveMatchingTaxSubmissions(r.Context(), actor, revisions, f.Get("review_note"))
				if err == nil {
					http.Redirect(w, r, "/super-admin/fulfillment/reviews?saved=1", http.StatusSeeOther)
					return
				}
				p.Error = "These submissions could not be approved together. Check the verified values, review note and current submission status."
			}
		} else if review {
			approve := f.Get("decision") == "approve"
			if p.Selected == nil || f.Get("decision") != "approve" && f.Get("decision") != "changes" {
				http.Error(w, "Invalid review", 400)
				return
			}
			if approve && f.Get("confirmed") != "yes" {
				p.Error = "Confirm that you verified these details before approving."
			} else {
				revision, _ := strconv.ParseInt(f.Get("revision"), 10, 64)
				err = h.Repository.ReviewSetupSubmission(r.Context(), actor, p.Selected.ID, revision, approve, f.Get("review_note"))
				if err == nil {
					http.Redirect(w, r, "/super-admin/fulfillment/reviews/"+strconv.FormatInt(p.Selected.ID, 10)+"?saved=1", 303)
					return
				}
				p.Error = "The review could not be saved. Reload to check its current status and verify the required details."
			}
		} else {
			p.Kind = f.Get("kind")
			p.VariantID, _ = strconv.ParseInt(f.Get("variant_id"), 10, 64)
			if p.Kind != "invoice" && p.Kind != "tax" {
				http.Error(w, "Invalid setup task", 400)
				return
			}
			p.Values = f
			p.Data = ports.SetupSubmissionData{LegalName: strings.TrimSpace(f.Get("legal_name")), Address: strings.TrimSpace(f.Get("address")), StateCode: f.Get("state_code"), Registration: f.Get("registration"), GSTIN: strings.ToUpper(strings.TrimSpace(f.Get("gstin"))), HSN: strings.TrimSpace(f.Get("hsn")), ReturnClass: f.Get("return_class")}
			submit := f.Get("intent") == "submit"
			action := "save_invoice_profile"
			if p.Kind == "tax" {
				action = "save_setup_tax"
				raw := strings.TrimSpace(f.Get("tax_rate_percent"))
				p.Data.DraftRatePercent = &raw
				if raw != "" {
					var rateErr error
					p.Data.RateBasisPoints, rateErr = parseTaxPercent(f)
					if rateErr != nil {
						p.Errors["tax_rate_percent"] = "Enter a rate from 0 to 100 with at most two decimal places."
					}
				}
			} else {
				p.VariantID = 0
			}
			if submit {
				v := url.Values{}
				for key, values := range f {
					v[key] = append([]string(nil), values...)
				}
				v.Set("seller_id", fmt.Sprint(a.SellerID))
				v.Set("reason", "Seller requested verification")
				v.Set("prefix", "S"+strings.ToUpper(strconv.FormatInt(a.SellerID, 36)))
				p.Errors = validateSetup(action, v)
			}
			if len(p.Errors) == 0 {
				payload, _ := json.Marshal(p.Data)
				in := ports.SetupSubmissionInput{Kind: p.Kind, SellerID: a.SellerID, VariantID: p.VariantID, Payload: payload, Submit: submit, Note: f.Get("note")}
				in.ID, _ = strconv.ParseInt(f.Get("submission_id"), 10, 64)
				in.ExpectedRevision, _ = strconv.ParseInt(f.Get("revision"), 10, 64)
				var saved int64
				saved, err = h.Repository.SaveSetupSubmission(r.Context(), actor, in)
				if err == nil {
					http.Redirect(w, r, "/seller-admin/setup?submission_id="+strconv.FormatInt(saved, 10)+"&saved=1", 303)
					return
				}
				if errors.Is(err, ports.ErrFulfillmentConflict) {
					p.Error = "These details changed or were already submitted. Reload to review the current version; your entered values are shown below."
				} else {
					p.Error = "The details could not be saved. Check the values and reload to verify the current status before retrying."
				}
			} else {
				p.Error = "Check the highlighted details before submitting for review."
			}
		}
	} else if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	if r.URL.Query().Get("saved") == "1" {
		p.Notice = "Saved. Review status and remaining checkout requirements are shown below."
	}
	if p.Kind == "" && p.Selected == nil {
		if !review {
			p.Shared, err = h.Repository.SellerFulfillmentSetup(r.Context(), actor)
			if err != nil {
				marketplaceHTTPError(w, err)
				return
			}
		}
		before, _ := strconv.ParseInt(r.URL.Query().Get("review_before"), 10, 64)
		p.Submissions, p.NextReview, err = h.Repository.SetupSubmissions(r.Context(), actor, 0, before)
		if err != nil {
			marketplaceHTTPError(w, err)
			return
		}
	}
	if p.Error != "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(422)
	}
	title, active := "Fulfillment setup", "fulfillment"
	if review {
		title, active = "Seller setup review", "fulfillment"
	}
	h.renderCommerceShell(w, r, a, csrf, title, active, components.CommerceSetupWorkspace(p))
}

func matchingTaxRevisions(values []string) map[int64]int64 {
	if len(values) < 2 || len(values) > 25 {
		return nil
	}
	revisions := make(map[int64]int64, len(values))
	for _, value := range values {
		key, current, ok := strings.Cut(value, ":")
		id, idErr := strconv.ParseInt(key, 10, 64)
		revision, revisionErr := strconv.ParseInt(current, 10, 64)
		if !ok || idErr != nil || revisionErr != nil || id <= 0 || revision <= 0 || revisions[id] != 0 {
			return nil
		}
		revisions[id] = revision
	}
	return revisions
}
