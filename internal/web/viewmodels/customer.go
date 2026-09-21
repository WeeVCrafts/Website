package viewmodels

import (
	"net/url"
	"time"
)

type Product struct {
	ID, VariantID                                                                   int64
	PriceCents, CompareAtCents                                                      int64
	Available                                                                       int
	Slug, Name, Seller, BrandName, Location, Category, ImageURL, ImageSrcSet, Badge string
	SKU                                                                             string
	CSRFToken                                                                       string
	Price, CompareAt, Discount                                                      string
	Rating                                                                          string
	Reviews                                                                         int
	InStock, HasCompareAt                                                           bool
	Variants                                                                        []ProductVariant
	Media                                                                           []ProductMedia
}

type ProductMedia struct {
	ID, URL, ContentType, Size string
	IsVideo                    bool
}

type CustomerSupportTicket struct {
	ID                                                                      int64
	Number, Subject, InitialMessage, Status, Priority, Created, OrderNumber string
	Messages                                                                []CustomerSupportMessage
}

type CustomerSupportMessage struct {
	Author, Role, Body, Created string
}

type Seller struct {
	Slug, Name, Location, ImageURL, Description string
	Rating                                      string
	Products                                    int
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
	Number, Status, State, Payment, Date string
	ReturnStatus                         string
	Total                                string
	Items                                []CartItem
	CanRequestReturn                     bool
}

type PaymentActivity struct {
	Reference, Amount, Date, Method, OrderNumber, Status string
}

type Return struct {
	ID, Status, Reason, Requested, ProductName, ImageURL, OrderNumber string
	Amount                                                            string
}

type Address struct {
	ID, Name, Phone, Line1, Line2, City, State, PostalCode, CountryCode, Lines string
}

type CustomerPage struct {
	Route, Title, Query, Category, Sort, Notice     string
	Brand, Tagline, Description                     string
	UserName, UserEmail, MemberSince                string
	RoleLabel, PortalLabel                          string
	PaymentMode, PaymentPublicKey                   string
	PaymentReady                                    bool
	CanCreateSupport                                bool
	CSRFToken                                       string
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
	Address                                         Address
	Addresses                                       []Address
	CartCount, WishlistCount                        int
	Page, Pages                                     int
	CheckoutStep                                    int
	Authenticated, Mock                             bool
	Now                                             time.Time
}
