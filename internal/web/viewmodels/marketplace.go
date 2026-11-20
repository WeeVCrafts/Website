package viewmodels

import (
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"net/url"
)

type MarketplacePickupChoice struct{ Name, PIN string }

type MarketplacePage struct {
	Workspace                    bool
	OrderTab, OrderURL, Focus    string
	RequestTask                  bool
	TaskItemID                   int64
	Summary                      *ports.CommerceOrderSummary
	Requests                     ports.RequestPage
	RequestSearch, RequestBucket string
	Readiness                    []ports.ShippingReadiness
	NextSetupID                  int64
	ports.MarketplaceView
	Root, Back, ActionPath, CSRFToken, Key, Notice string
	ShippingConfigured, BookingsEnabled            bool
	CheckoutEnabled                                bool
	Tab                                            string
	Setup                                          *ports.MarketplaceSetup
	States                                         []domain.IndianState
	Pickups                                        []MarketplacePickupChoice
	PickupError                                    string
	SetupValues                                    url.Values
	SetupErrors                                    map[string]string
	SetupContext                                   url.Values
	SetupAction                                    string
	SetupStep                                      string
}

type GuidedSetupStep struct {
	Key, Label string
	Complete   bool
}

func (p MarketplacePage) GuidedSteps() []GuidedSetupStep {
	r := p.Setup
	steps := []GuidedSetupStep{{Key: "choose", Label: "Choose product"}, {Key: "invoice", Label: "Invoice details"}, {Key: "pickup", Label: "Pickup location"}, {Key: "parcel", Label: "Packed measurements"}, {Key: "tax", Label: "Tax & return policy"}, {Key: "review", Label: "Review"}}
	if r == nil {
		return steps
	}
	steps[0].Complete = r.Query.VariantID > 0
	steps[1].Complete = r.InvoiceReady
	steps[2].Complete = r.PickupReady
	if r.Selected != nil {
		s := r.Selected
		steps[1].Complete = s.InvoiceReady
		steps[2].Complete = s.PickupReady
		steps[3].Complete = s.ParcelReady()
		steps[4].Complete = s.TaxReady && s.Classified
	}
	steps[5].Complete = steps[1].Complete && steps[2].Complete && steps[3].Complete && steps[4].Complete
	return steps
}

func (p MarketplacePage) FirstMissingStep() string {
	for _, s := range p.GuidedSteps() {
		if s.Key != "choose" && s.Key != "review" && !s.Complete {
			return s.Key
		}
	}
	return "review"
}
func (p MarketplacePage) StepComplete(key string) bool {
	for _, s := range p.GuidedSteps() {
		if s.Key == key {
			return s.Complete
		}
	}
	return false
}
func (p MarketplacePage) StepNumber() int {
	for i, s := range p.GuidedSteps() {
		if s.Key == p.SetupStep {
			return i + 1
		}
	}
	return 6
}
func (p MarketplacePage) StepLabel(key string) string {
	for _, s := range p.GuidedSteps() {
		if s.Key == key {
			return s.Label
		}
	}
	switch key {
	case "pilot":
		return "Pilot setup"
	case "launch":
		return "Launch catalog"
	case "policy":
		return "Publish policy"
	case "pause":
		return "Pause shipping"
	}
	return "Choose product"
}
func (p MarketplacePage) OverviewURL() string {
	return p.SetupURL("variant_id", "", "step", "", "pickup_choices", "")
}
func (p MarketplacePage) PurchaseReady() bool {
	return p.Setup != nil && p.Setup.Selected != nil && p.Setup.Selected.Ready() && p.LiveOptionAllowed(*p.Setup.Selected)
}

func (p MarketplacePage) CatalogIncludes(option ports.ShippingReadiness) bool {
	switch p.Policy.LiveCatalogScope {
	case "ready":
		return true
	case "seller":
		return option.SellerID == p.Policy.LiveCatalogSellerID
	default:
		return option.Selected
	}
}

func (p MarketplacePage) LiveOptionAllowed(option ports.ShippingReadiness) bool {
	return p.CatalogIncludes(option) && p.CheckoutEnabled && p.BookingsEnabled &&
		p.ShippingConfigured && p.Policy.PublishedAt != nil && p.Policy.ShippingMode == "live"
}

func (p MarketplacePage) SetupValue(name, fallback string) string {
	if values, ok := p.SetupValues[name]; ok {
		if len(values) > 0 {
			return values[0]
		}
		return ""
	}
	return fallback
}

func (p MarketplacePage) FormValue(action, name, fallback string) string {
	if p.SetupAction == action {
		return p.SetupValue(name, fallback)
	}
	return fallback
}

func (p MarketplacePage) FieldError(action, name string) string {
	if p.SetupAction == action {
		return p.SetupErrors[name]
	}
	return ""
}

func (p MarketplacePage) SetupURL(pairs ...string) string {
	q := url.Values{}
	for k, v := range p.SetupContext {
		q[k] = append([]string(nil), v...)
	}
	q.Set("tab", "setup")
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] == "" {
			q.Del(pairs[i])
		} else {
			q.Set(pairs[i], pairs[i+1])
		}
	}
	return p.Root + "?" + q.Encode()
}
