package commerce

import "time"

type SellerConfirmation struct {
	ID, TicketID, SellerID                                             int64
	CaseNumber, OrderNumber, SellerName, Question, Status, CloseReason string
	Carrier, TrackingNumber, FulfillmentStatus                         string
	CreatedAt                                                          time.Time
	Items                                                              []string
	History                                                            []SellerFulfillmentEvent
	Messages                                                           []SellerConfirmationMessage
}
type SellerConfirmationMessage struct {
	Author, Body string
	CreatedAt    time.Time
}
type CaseSeller struct {
	ID                                               int64
	Name, Carrier, TrackingNumber, FulfillmentStatus string
}

type SellerFulfillmentEvent struct {
	Status, Carrier, TrackingNumber, Note string
	CreatedAt                             time.Time
}
