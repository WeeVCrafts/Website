package viewmodels

import "github.com/wecratfs/commerce/internal/ports"

type MarketplacePage struct {
	Readiness   []ports.ShippingReadiness
	NextSetupID int64
	ports.MarketplaceView
	Root, Back, ActionPath, CSRFToken, Key, Notice string
	ShippingConfigured, BookingsEnabled            bool
}
