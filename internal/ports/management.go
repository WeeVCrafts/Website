package ports

import (
	"context"
	"errors"
)

var ErrManagementValidation = errors.New("invalid management input")
var ErrManagementConflict = errors.New("record already exists or is still in use")

type ManagementRecord struct {
	ID, Name, Email, Status, Category string
	BasisPoints                       int
	Active                            bool
	Products, Orders                  int64
}

type ManagementResult struct {
	Records []ManagementRecord
	Total   int
}

type ManagementMutation struct {
	Section, Action, ID, Name, Category, Reason string
	BasisPoints                                 int
	Active                                      bool
	Settings                                    StorefrontSettings
}

type StorefrontSettings struct {
	SupportEmail        string `json:"support_email"`
	ShippingInformation string `json:"shipping_information"`
}

type ManagementRepository interface {
	ListManagement(context.Context, int64, string, string, int, int) (ManagementResult, error)
	MutateManagement(context.Context, int64, ManagementMutation) error
	StorefrontSettings(context.Context) (StorefrontSettings, error)
}
