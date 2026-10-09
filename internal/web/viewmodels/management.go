package viewmodels

import "github.com/wecratfs/commerce/internal/ports"

type ManagementPage struct {
	Admin                                        AdminPage
	Section, Action, Query, PreviousURL, NextURL string
	Page, Total                                  int
	Records                                      []ports.ManagementRecord
	Settings                                     ports.StorefrontSettings
}
