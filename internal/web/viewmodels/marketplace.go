package viewmodels

import (
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"net/url"
)

type MarketplacePickupChoice struct{ Name, PIN string }

type MarketplacePage struct {
	Readiness   []ports.ShippingReadiness
	NextSetupID int64
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
