package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/components"
)

const sellerParcelURL = "/seller-admin/setup?kind=parcel"

func (h *MarketplaceHandler) serveSellerParcelSetup(w http.ResponseWriter, r *http.Request, actor int64, csrf string, a ports.CommerceActor) {
	p := viewmodels.SellerParcelPage{CSRFToken: csrf}
	if r.Method == "POST" {
		p.Values = r.PostForm
		phase := p.Values.Get("phase")
		var err error
		switch phase {
		case "create":
			num := func(key string) int { n, _ := strconv.Atoi(p.Values.Get(key)); return n }
			err = h.Repository.CreateSellerParcel(r.Context(), actor, p.Values.Get("profile_name"), ports.Parcel{
				WeightGrams: num("weight_grams"), LengthCM: num("length_cm"), WidthCM: num("width_cm"), HeightCM: num("height_cm"),
			})
		case "preview", "save":
			var change ports.SellerParcelChange
			change, err = parseSellerParcelChange(p.Values)
			if err == nil && phase == "preview" {
				var preview ports.SellerParcelPreview
				preview, err = h.Repository.PreviewSellerParcelChange(r.Context(), actor, change)
				if err == nil {
					p.Preview = &preview
				}
			} else if err == nil {
				err = h.Repository.SaveSellerParcelChange(r.Context(), actor, change, p.Values.Get("preview_token"))
			}
		default:
			err = ports.ErrShippingInput
		}
		switch {
		case err == nil && phase != "preview":
			http.Redirect(w, r, sellerParcelURL+"&saved=1", http.StatusSeeOther)
			return
		case errors.Is(err, ports.ErrFulfillmentConflict):
			if phase == "create" {
				p.Error = "A profile with this name already exists. Choose another name."
			} else {
				p.Error = "One or more options changed. Review the current measurements and preview your selection again."
			}
		case errors.Is(err, ports.ErrShippingInput), errors.Is(err, ports.ErrNotFound):
			p.Error = "Choose an available profile and eligible options, or check the measurements entered."
		case err != nil:
			marketplaceHTTPError(w, err)
			return
		}
	} else if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	var err error
	p.Setup, err = h.Repository.SellerParcelSetup(r.Context(), actor, before)
	if err != nil {
		marketplaceHTTPError(w, err)
		return
	}
	if r.URL.Query().Get("saved") == "1" {
		p.Notice = "Saved. Delivery estimates now use the updated profiles for applicable options."
	}
	if p.Error != "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
	}
	h.renderCommerceShell(w, r, a, csrf, "Parcel profiles", "fulfillment", components.SellerParcelWorkspace(p))
}

func parseSellerParcelChange(values url.Values) (ports.SellerParcelChange, error) {
	target := values.Get("profile")
	change := ports.SellerParcelChange{Action: values.Get("action"), FollowDefault: target == "default"}
	if target != "default" && target != "none" {
		id, err := strconv.ParseInt(target, 10, 64)
		if err != nil || id <= 0 {
			return change, ports.ErrShippingInput
		}
		change.ProfileID = id
	}
	if len(values["option_id"]) > 25 {
		return change, ports.ErrShippingInput
	}
	for _, raw := range values["option_id"] {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return change, ports.ErrShippingInput
		}
		change.OptionIDs = append(change.OptionIDs, id)
	}
	return change, nil
}
