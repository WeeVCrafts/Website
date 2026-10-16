package viewmodels

import "github.com/wecratfs/commerce/internal/ports"

type ManagementPage struct {
	Admin                                        AdminPage
	Section, Action, Query, PreviousURL, NextURL string
	Page, Total                                  int
	Records                                      []ports.ManagementRecord
	Settings                                     ports.StorefrontSettings
}

type CategoryManagerPage struct {
	SelectedIDs map[int64]bool
	ProductPage int
	ManagementPage
	Error, EditorAction, EditorReason, SelectedCategory, ProductQuery, ProductPreviousURL, ProductNextURL string
	Editor                                                                                                ports.ManagementRecord
	Products                                                                                              []ports.CategoryProduct
	ProductTotal                                                                                          int
}
