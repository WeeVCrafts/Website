package viewmodels

import domain "github.com/wecratfs/commerce/internal/domain/commerce"

type SellerSupportPage struct {
	QueueStatus, QueueNext string
	Portal                 SellerAdminPage
	Confirmations          []domain.SellerConfirmation
	RequestKey             string
	Tickets                []CustomerSupportTicket
	Status                 string
	Total                  int
	PreviousURL, NextURL   string
}

type ReturnLabelPage struct {
	Customer                                               CustomerPage
	ID, OrderNumber, Status, Reason, Created, CustomerName string
	SenderLines                                            []string
	Items                                                  []ReturnLabelItem
	Destinations                                           []string
	Instructions, SupportEmail                             string
}

type ReturnLabelItem struct {
	Name, SKU, Quantity, Seller string
}

type CompanySupportPage struct {
	Admin                  AdminPage
	Support                SupportPortalPage
	Confirmations          []domain.SellerConfirmation
	Sellers                []domain.CaseSeller
	RequestKey             string
	QueueStatus, QueueNext string
}
