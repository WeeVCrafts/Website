package viewmodels

import (
	"net/url"

	"github.com/wecratfs/commerce/internal/ports"
)

type SellerParcelPage struct {
	Setup                    ports.SellerParcelSetup
	Preview                  *ports.SellerParcelPreview
	CSRFToken, Notice, Error string
	Values                   url.Values
}

func (p SellerParcelPage) Value(key string) string { return p.Values.Get(key) }
