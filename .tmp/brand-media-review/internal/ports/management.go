package ports

import (
	"context"
	"errors"
)

var ErrManagementValidation = errors.New("invalid management input")
var ErrManagementConflict = errors.New("record already exists or is still in use")

type ManagementRecord struct {
	ImageURL, ImageObjectKey          string
	DisplayOrder                      int
	ShowInNavigation                  bool
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
	ImageURL, ImageObjectKey                    string
	ReplaceImage                                bool
	DisplayOrder                                int
	ShowInNavigation                            bool
	Section, Action, ID, Name, Category, Reason string
	BasisPoints                                 int
	Active                                      bool
	Settings                                    StorefrontSettings
	SaveSocialLinks                             bool
}

type CategoryProduct struct {
	ID                            int64
	Name, Seller, PrimaryCategory string
	Assigned                      bool
}

type StorefrontSettings struct {
	SupportEmail        string `json:"support_email"`
	ShippingInformation string `json:"shipping_information"`
	InstagramURL        string `json:"instagram_url"`
	FacebookURL         string `json:"facebook_url"`
	YouTubeURL          string `json:"youtube_url"`
	XURL                string `json:"x_url"`
	WhatsAppURL         string `json:"whatsapp_url"`
}

type SocialLink struct{ Name, URL string }

func (s StorefrontSettings) SocialLinks() []SocialLink {
	links := []SocialLink{{"Instagram", s.InstagramURL}, {"Facebook", s.FacebookURL}, {"YouTube", s.YouTubeURL}, {"X", s.XURL}, {"WhatsApp", s.WhatsAppURL}}
	result := make([]SocialLink, 0, 5)
	for _, link := range links {
		if link.URL != "" {
			result = append(result, link)
		}
	}
	return result
}

type ManagementRepository interface {
	ListManagement(context.Context, int64, string, string, int, int) (ManagementResult, error)
	MutateManagement(context.Context, int64, ManagementMutation) error
	StorefrontSettings(context.Context) (StorefrontSettings, error)
}
