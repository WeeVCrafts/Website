package viewmodels

import (
	"github.com/wecratfs/commerce/internal/domain/storefront"
)

type StorefrontEditor struct {
	Published                                    storefront.Release
	Readiness                                    []StorefrontFinding
	Destinations                                 []storefront.Link
	PreviewURL                                   string
	Differences                                  []StorefrontDifference
	PaymentOptions                               []string
	HistoryPage                                  int
	MoreHistory                                  bool
	MediaQuery, PreviousMedia, NextMedia         string
	MediaPage                                    int
	ProductChoices                               []StorefrontProductChoice
	ProductQuery, PreviousProducts, NextProducts string
	ProductTotal, ProductPage                    int
	Promotion                                    storefront.Promotion
	Admin                                        AdminPage
	Draft                                        storefront.Draft
	Drafts                                       []storefront.Draft
	Releases                                     []storefront.Release
	Tab, Selected, Error                         string
	Section                                      storefront.Section
	Slide                                        storefront.Slide
	Category                                     storefront.Category
	Template                                     storefront.Template
	Page                                         storefront.Page
	Media                                        []StorefrontMedia
	Simulation                                   string
}

type StorefrontFinding struct {
	Label, Hint, Tab, Selected string
	Warning                    bool
}
type StorefrontMedia struct {
	ID, URL, Alt, Created string
	FocalX, FocalY        int
}

type StorefrontProductChoice struct {
	ID                         int64
	Name, Image, Status, Price string
	InStock, Selected          bool
}

type StorefrontDifference struct{ Field, Saved, Proposed string }
