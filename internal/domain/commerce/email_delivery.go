package commerce

import "time"

type EmailDeliverySummary struct {
	ID, OrderNumber, Kind, ProviderID, State string
	StartedAt, StateAt                       time.Time
}
