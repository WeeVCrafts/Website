package viewmodels

import (
	"github.com/wecratfs/commerce/internal/ports"
	"net/url"
)

type CommerceSetupPage struct {
	Actor                          ports.CommerceActor
	CSRFToken, Error, Notice, Root string
	Rows                           []ports.SellerReadinessOption
	Next, NextReview               int64
	Submissions                    []ports.SetupSubmission
	Selected                       *ports.SetupSubmission
	Data                           ports.SetupSubmissionData
	Step, Kind                     string
	VariantID                      int64
	Values                         url.Values
	CurrentInvoice                 *ports.SellerInvoiceProfile
	CurrentData                    ports.SetupSubmissionData
	Errors                         map[string]string
}
