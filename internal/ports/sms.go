package ports

import "context"

// SMS delivery is separate from the order/payment state and email delivery.
type SMSJob struct {
	ID, UserID                            int64
	TemplateKey, Phone, Reference, Update string
}

type SMSTemplate struct {
	ID        string            `json:"template_id"`
	DLTID     string            `json:"dlt_template_id"`
	Variables map[string]string `json:"variables"`
}

type SMSResult struct {
	// accepted means provider acceptance, never handset delivery.
	Outcome, ProviderID string
}

type SMSSender interface {
	SendTransactionalSMS(context.Context, SMSJob) (SMSResult, error)
}

type SMSStore interface {
	ClaimSMS(context.Context) (SMSJob, bool, error)
	FinishSMS(context.Context, int64, SMSResult) error
}
