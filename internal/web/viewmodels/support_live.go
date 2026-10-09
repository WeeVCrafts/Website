package viewmodels

type SellerSupportPage struct {
	Portal               SellerAdminPage
	Tickets              []CustomerSupportTicket
	Status               string
	Total                int
	PreviousURL, NextURL string
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
