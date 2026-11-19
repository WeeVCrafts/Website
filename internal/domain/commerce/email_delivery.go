package commerce

import "time"

type EmailDeliverySummary struct {
	ID, OrderNumber, Kind, ProviderID, State, WorkflowReference string
	StartedAt, StateAt                                          time.Time
}
