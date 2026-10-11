package viewmodels

import (
	"github.com/wecratfs/commerce/internal/ports"
	"net/url"
	"time"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainprivacy "github.com/wecratfs/commerce/internal/domain/privacy"
)

type Product struct {
	ID, VariantID                                                                                                           int64
	PriceCents, CompareAtCents                                                                                              int64
	Available                                                                                                               int
	Slug, Name, Description, DeliveryLabel, Seller, SellerSlug, BrandName, Location, Category, ImageURL, ImageSrcSet, Badge string
	SKU                                                                                                                     string
	CSRFToken                                                                                                               string
	Price, CompareAt, Discount                                                                                              string
	Rating                                                                                                                  string
	Reviews                                                                                                                 int
	InStock, HasCompareAt, Saved                                                                                            bool
	Variants                                                                                                                []ProductVariant
	Media                                                                                                                   []ProductMedia
}

type ProductMedia struct {
	ID, URL, ContentType, Size string
	IsVideo                    bool
}

type CustomerSupportTicket struct {
	ID                                                                      int64
	Number, Subject, InitialMessage, Status, Priority, Created, OrderNumber string
	Messages                                                                []CustomerSupportMessage
	Attachments                                                             []CustomerSupportAttachment
}

type CustomerSupportMessage struct {
	Author, Role, Body, Created string
	Attachments                 []CustomerSupportAttachment
}

type CustomerSupportAttachment struct {
	ID, FileName, ContentType, Size, URL string
}

type NotificationPreferences struct {
	OrderUpdates, MakerMessages, Offers, MakerStories bool
}

type ProductReview struct {
	ID, ProductID, OrderID                         int64
	ProductName, ProductSlug, SellerName           string
	Rating                                         int
	Title, Body, Status, Created, ImageURL, Author string
	VerifiedPurchase                               bool
}

type Seller struct {
	Slug, Name, Location, ImageURL, Description string
	PortraitURL, Technique                      string
	Rating                                      string
	Products                                    int
	ID                                          int64
	Status                                      string
	Following                                   bool
}

type Brand struct {
	Slug, Name, Tagline, Location, ImageURL, Description string
	Products                                             int
}

type Category struct {
	Slug, Name, Description, ImageURL string
	Products                          int
}

type CartItem struct {
	Product     Product
	VariantName string
	Quantity    int
	Delivery    string
}

type Order struct {
	Shipments                                          []domaincommerce.OrderFulfillment
	Number, Status, State, Payment, Date, PaymentState string
	ReturnStatus, RefundStatus                         string
	Total                                              string
	Subtotal, Shipping, Discount                       string
	Items                                              []CartItem
	CanCancel, CanRequestReturn, CanRequestRefund      bool
}

type PaymentActivity struct {
	Reference, Amount, Date, Method, OrderNumber, Status string
}

type Return struct {
	CanPrintLabel                                                     bool
	ID, Status, Reason, Requested, ProductName, ImageURL, OrderNumber string
	Amount                                                            string
}

type Address struct {
	ID, Name, Phone, Line1, Line2, City, State, PostalCode, CountryCode, Lines string
}

type CustomerPage struct {
	CatalogueEmpty                                  bool
	UserPhone, AvatarURL, ProfileError              string
	IsCustomer, ProfileIncomplete                   bool
	SocialLinks                                     []ports.SocialLink
	SupportEmail, ShippingInformation               string
	Sessions                                        []CustomerSession
	Notifications                                   []CustomerNotification
	NotificationsNextURL                            string
	Route, Title, Query, Category, Sort, Notice     string
	CouponCode                                      string
	Brand, Tagline, Description                     string
	UserName, UserEmail, MemberSince                string
	RoleLabel, PortalLabel                          string
	PublicURL                                       string
	PaymentMode, PaymentPublicKey                   string
	AccountUserID                                   int64
	PaymentReady                                    bool
	NoPaymentDemo                                   bool
	CanCreateSupport, CanFollowSeller               bool
	CSRFToken                                       string
	RefundKey                                       string
	Filters                                         url.Values
	Product                                         Product
	OrderCount                                      int
	SavedProducts                                   []Product
	ItemsTotal, ShippingTotal, DiscountTotal, Total string
	Products                                        []Product
	FilterCounts                                    map[string]int
	Categories                                      []Category
	Sellers                                         []Seller
	Brands                                          []Brand
	Cart                                            []CartItem
	Orders                                          []Order
	PaymentActivity                                 []PaymentActivity
	SelectedOrder                                   Order
	Returns                                         []Return
	SupportTickets                                  []CustomerSupportTicket
	NotificationPreferences                         NotificationPreferences
	PrivacyCenter                                   domainprivacy.Center
	PendingReviews, WrittenReviews, ProductReviews  []ProductReview
	ReviewCount                                     int
	Address                                         Address
	Addresses                                       []Address
	CartCount, WishlistCount                        int
	Page, Pages, TotalResults                       int
	CheckoutStep                                    int
	Authenticated, Mock                             bool
	Now                                             time.Time
}

type CustomerSession struct {
	ID, Browser, IPAddress, LastSeen, Expires string
	Current                                   bool
}

type CustomerNotification struct {
	ID                      int64
	Title, Body, Date, Link string
	Unread                  bool
}
