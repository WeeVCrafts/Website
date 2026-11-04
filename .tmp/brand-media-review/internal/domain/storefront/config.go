// Package storefront defines the supported, data-only storefront language.
// Catalog facts and transactional records deliberately do not live here.
package storefront

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

var (
	ErrConflict   = errors.New("this draft has changed; compare and reload the latest version")
	ErrLocked     = errors.New("submitted versions are locked; request changes before editing")
	ErrValidation = errors.New("invalid storefront configuration")
)

type Config struct {
	Trust        []Tile      `json:"trust"`
	PolicyLinks  []Link      `json:"policy_links"`
	FooterGroups []LinkGroup `json:"footer_groups"`
	Promotions   []Promotion `json:"promotions"`
	Schema       int         `json:"schema"`
	Slides       []Slide     `json:"slides"`
	Homepage     []Section   `json:"homepage"`
	Categories   []Category  `json:"categories"`
	Navigation   []Link      `json:"navigation"`
	Footer       []Link      `json:"footer"`
	Theme        Theme       `json:"theme"`
	Templates    []Template  `json:"templates"`
	Pages        []Page      `json:"pages"`
	Settings     Settings    `json:"settings"`
}
type LinkGroup struct {
	Title string `json:"title"`
	Text  string `json:"text"`
	Links []Link `json:"links"`
}
type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}
type Slide struct {
	ID          string `json:"id"`
	Order       int    `json:"order"`
	Title       string `json:"title"`
	Text        string `json:"text"`
	Kicker      string `json:"kicker"`
	Image       string `json:"image"`
	MobileImage string `json:"mobile_image"`
	Alt         string `json:"alt"`
	Button      Link   `json:"button"`
	Secondary   Link   `json:"secondary"`
	Hidden      bool   `json:"hidden"`
}
type Responsive struct {
	DesktopColumns int  `json:"desktop_columns"`
	TabletColumns  int  `json:"tablet_columns"`
	MobileColumns  int  `json:"mobile_columns"`
	Spacing        int  `json:"spacing"`
	MobileSpacing  int  `json:"mobile_spacing"`
	HideDesktop    bool `json:"hide_desktop"`
	HideTablet     bool `json:"hide_tablet"`
	HideMobile     bool `json:"hide_mobile"`
}
type Section struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Order       int        `json:"order"`
	Hidden      bool       `json:"hidden"`
	Title       string     `json:"title"`
	Text        string     `json:"text"`
	Image       string     `json:"image"`
	MobileImage string     `json:"mobile_image"`
	Alt         string     `json:"alt"`
	Button      Link       `json:"button"`
	Style       string     `json:"style"`
	Selection   Selection  `json:"selection"`
	Items       []Tile     `json:"items"`
	Responsive  Responsive `json:"responsive"`
}
type Tile struct {
	Title string `json:"title"`
	Text  string `json:"text"`
	Image string `json:"image"`
	Alt   string `json:"alt"`
	URL   string `json:"url"`
}
type Selection struct {
	Mode      string  `json:"mode"`
	Category  string  `json:"category"`
	Products  []int64 `json:"products"`
	Limit     int     `json:"limit"`
	Available bool    `json:"available"`
	MinPrice  int64   `json:"min_price"`
	MaxPrice  int64   `json:"max_price"`
}
type Category struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Image       string `json:"image"`
	ObjectKey   string `json:"object_key,omitempty"`
	Order       int    `json:"order"`
	Visible     bool   `json:"visible"`
}
type Theme struct {
	Logo        string `json:"logo"`
	Primary     string `json:"primary"`
	Accent      string `json:"accent"`
	Background  string `json:"background"`
	BodyFont    string `json:"body_font"`
	HeadingFont string `json:"heading_font"`
}

// Match is empty for the default, category:<slug> or product:<slug> for overrides.
type Template struct {
	HideDefaultIntro bool      `json:"hide_default_intro"`
	ID               string    `json:"id"`
	Kind             string    `json:"kind"`
	Match            string    `json:"match"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	Image            string    `json:"image"`
	Sections         []Section `json:"sections"`
}
type Page struct {
	Legacy       bool      `json:"legacy"`
	Slug         string    `json:"slug"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	SharingImage string    `json:"sharing_image"`
	Sections     []Section `json:"sections"`
	Hidden       bool      `json:"hidden"`
}
type Settings struct {
	SupportEmail        string   `json:"support_email"`
	ShippingInformation string   `json:"shipping_information"`
	Social              []Link   `json:"social"`
	Policy              Policy   `json:"policy"`
	Features            Features `json:"features"`
}
type Features struct {
	Wishlist  bool `json:"wishlist"`
	QuickView bool `json:"quick_view"`
	Reviews   bool `json:"reviews"`
	Coupons   bool `json:"coupons"`
}
type Policy struct {
	RestrictPayments  bool     `json:"restrict_payments"`
	ShippingCents     int64    `json:"shipping_cents"`
	FreeShippingCents int64    `json:"free_shipping_cents"`
	PINRestricted     bool     `json:"pin_restricted"`
	PINCodes          []string `json:"pin_codes"`
	ReturnDays        int      `json:"return_days"` // zero preserves legacy eligibility
	Payments          []string `json:"payments"`    // empty preserves configured options
}

func (p Policy) Shipping(subtotal int64) int64 {
	if subtotal <= 0 || (p.FreeShippingCents > 0 && subtotal >= p.FreeShippingCents) {
		return 0
	}
	return p.ShippingCents
}
func (p Policy) Serves(pin string) bool { return !p.PINRestricted || slices.Contains(p.PINCodes, pin) }
func (p Policy) PaymentEnabled(mode string) bool {
	return !p.RestrictPayments || slices.Contains(p.Payments, mode)
}

type Revision struct {
	ID      int64
	Config  Config
	Preview bool
}
type revisionKey struct{}
type expectedPolicyKey struct{}

func WithExpectedPolicy(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, expectedPolicyKey{}, id)
}
func ExpectedPolicy(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(expectedPolicyKey{}).(int64)
	return id, ok
}

func WithRevision(ctx context.Context, r Revision) context.Context {
	return context.WithValue(ctx, revisionKey{}, r)
}
func Current(ctx context.Context) (Revision, bool) {
	r, ok := ctx.Value(revisionKey{}).(Revision)
	return r, ok
}

var slug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,79}$`)
var color = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func SlugValid(s string) bool { return slug.MatchString(s) }
func SafeURL(s string) bool {
	if s == "" {
		return true
	}
	if strings.ContainsAny(s, "\r\n\\") {
		return false
	}
	if strings.HasPrefix(s, "/") {
		return !strings.HasPrefix(s, "//")
	}
	u, e := url.Parse(s)
	return e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
}
func (c Config) Validate() error {
	fail := func(s string) error { return fmt.Errorf("%w: %s", ErrValidation, s) }
	if c.Schema != 1 {
		return fail("unsupported version")
	}
	if len(c.Slides) > 20 || len(c.Homepage) > 60 || len(c.Categories) > 200 || len(c.Templates) > 200 || len(c.Pages) > 200 {
		return fail("too many items")
	}
	if !color.MatchString(c.Theme.Primary) || !color.MatchString(c.Theme.Accent) || !color.MatchString(c.Theme.Background) {
		return fail("theme colors must use #RRGGBB")
	}
	if contrast(c.Theme.Primary, "#ffffff") < 4.5 || contrast(c.Theme.Primary, c.Theme.Background) < 4.5 || contrast("#20362d", c.Theme.Background) < 4.5 || contrast(c.Theme.Accent, c.Theme.Background) < 4.5 {
		return fail("choose a light background and darker primary/accent colors with readable contrast")
	}
	if !slices.Contains([]string{"DM Sans", "Inter"}, c.Theme.BodyFont) || !slices.Contains([]string{"Playfair Display", "DM Sans", "Inter"}, c.Theme.HeadingFont) {
		return fail("choose a supported font")
	}
	if !SafeURL(c.Theme.Logo) {
		return fail("invalid logo URL")
	}
	ids := map[string]bool{}
	for _, s := range c.Slides {
		if s.Order < 0 || s.Order > 1000000 || !slug.MatchString(s.ID) || ids["slide:"+s.ID] || strings.TrimSpace(s.Title) == "" || len(s.Title) > 200 || len(s.Text) > 2000 || !SafeURL(s.Image) || !SafeURL(s.MobileImage) || !SafeURL(s.Button.URL) || !SafeURL(s.Secondary.URL) {
			return fail("check slide IDs, titles and destinations")
		}
		ids["slide:"+s.ID] = true
	}
	for _, s := range c.Categories {
		if s.Order < 0 || s.Order > 1000000 || !slug.MatchString(s.Slug) || ids["category:"+s.Slug] || strings.TrimSpace(s.Name) == "" || len(s.Name) > 120 || !SafeURL(s.Image) {
			return fail("check category names, slugs and photographs")
		}
		ids["category:"+s.Slug] = true
	}
	for _, l := range append(append(slices.Clone(c.Navigation), c.Footer...), c.Settings.Social...) {
		if l.Label == "" || len(l.Label) > 100 || l.URL == "" || !SafeURL(l.URL) {
			return fail("check link labels and HTTPS or storefront destinations")
		}
	}
	for _, g := range c.FooterGroups {
		if len(g.Title) > 100 || len(g.Text) > 1000 || len(g.Links) > 30 {
			return fail("footer group is too long")
		}
		for _, l := range g.Links {
			if l.Label == "" || l.URL == "" || !SafeURL(l.URL) {
				return fail("invalid footer link")
			}
		}
	}
	if len(c.Trust) > 20 || len(c.PolicyLinks) > 20 {
		return fail("too many trust items or policy links")
	}
	for _, tile := range c.Trust {
		if len(tile.Title) > 200 || len(tile.Text) > 2000 || !SafeURL(tile.Image) || !SafeURL(tile.URL) {
			return fail("check trust content images and links")
		}
	}
	for _, l := range c.PolicyLinks {
		if l.Label == "" || l.URL == "" || !SafeURL(l.URL) {
			return fail("check policy links")
		}
	}
	if len(c.Navigation) > 30 || len(c.Footer) > 60 {
		return fail("too many navigation links")
	}
	if err := validateSections(c.Homepage); err != nil {
		return err
	}
	for _, t := range c.Templates {
		if !slug.MatchString(t.ID) || ids["template:"+t.ID] || !slices.Contains(TemplateKinds, t.Kind) {
			return fail("invalid or duplicate page template")
		}
		ids["template:"+t.ID] = true
		if ids["assignment:"+t.Kind+":"+t.Match] {
			return fail("duplicate template assignment")
		}
		ids["assignment:"+t.Kind+":"+t.Match] = true
		if t.Match != "" && !strings.HasPrefix(t.Match, "category:") && !strings.HasPrefix(t.Match, "product:") {
			return fail("invalid template assignment")
		}
		if !SafeURL(t.Image) {
			return fail("invalid template image")
		}
		if e := validateSections(t.Sections); e != nil {
			return e
		}
	}
	for _, p := range c.Pages {
		if !slug.MatchString(p.Slug) || ids["page:"+p.Slug] || strings.TrimSpace(p.Title) == "" || !SafeURL(p.SharingImage) {
			return fail("check page slug, title and image")
		}
		ids["page:"+p.Slug] = true
		if e := validateSections(p.Sections); e != nil {
			return e
		}
	}
	if e := validatePromotions(c.Promotions); e != nil {
		return e
	}
	p := c.Settings.Policy
	if p.ShippingCents < 0 || p.ShippingCents > 1000000 || p.FreeShippingCents < 0 || p.FreeShippingCents > 100000000 || p.ReturnDays < 0 || p.ReturnDays > 365 || p.PINRestricted && len(p.PINCodes) == 0 {
		return fail("check shipping, PIN codes and return window")
	}
	for _, pin := range p.PINCodes {
		if len(pin) != 6 || pin[0] < '1' || pin[0] > '9' || strings.Trim(pin, "0123456789") != "" {
			return fail("PIN codes must have six digits")
		}
	}
	for _, mode := range p.Payments {
		if !slices.Contains([]string{"razorpay", "local", "no-payment"}, mode) {
			return fail("unsupported payment option")
		}
	}
	if len(c.Settings.SupportEmail) > 254 || !strings.Contains(c.Settings.SupportEmail, "@") {
		return fail("enter a support email")
	}
	return nil
}

var SectionTypes = []string{"hero", "categories", "products", "makers", "collections", "banner", "image-text", "rich-text", "faq", "trust"}
var TemplateKinds = []string{"category", "search", "product", "maker", "brand", "wishlist", "cart", "checkout", "account", "orders", "authentication"}

func validateSections(sections []Section) error {
	if len(sections) > 60 {
		return fmt.Errorf("%w: too many sections", ErrValidation)
	}
	seen := map[string]bool{}
	for _, s := range sections {
		if s.Order < 0 || s.Order > 1000000 || !slug.MatchString(s.ID) || seen[s.ID] || !slices.Contains(SectionTypes, s.Type) {
			return fmt.Errorf("%w: invalid or duplicate section", ErrValidation)
		}
		seen[s.ID] = true
		if len(s.Title) > 200 || len(s.Text) > 20000 || !SafeURL(s.Image) || !SafeURL(s.MobileImage) || !SafeURL(s.Button.URL) {
			return fmt.Errorf("%w: check section copy and destinations", ErrValidation)
		}
		if s.Type == "products" && (!slices.Contains([]string{"grid", "rail", "editorial", "band"}, s.Style) || !slices.Contains([]string{"featured", "newest", "category", "selected"}, s.Selection.Mode) || s.Selection.Limit < 1 || s.Selection.Limit > 24 || s.Selection.MinPrice < 0 || s.Selection.MaxPrice < 0 || (s.Selection.MaxPrice > 0 && s.Selection.MinPrice > s.Selection.MaxPrice) || len(s.Selection.Products) > 24) {
			return fmt.Errorf("%w: check product selection and layout", ErrValidation)
		}
		r := s.Responsive
		if r.DesktopColumns < 0 || r.DesktopColumns > 6 || r.TabletColumns < 0 || r.TabletColumns > 4 || r.MobileColumns < 0 || r.MobileColumns > 2 || r.Spacing < 0 || r.Spacing > 120 || r.MobileSpacing < 0 || r.MobileSpacing > 80 {
			return fmt.Errorf("%w: responsive values out of range", ErrValidation)
		}
		for _, i := range s.Items {
			if !SafeURL(i.Image) || !SafeURL(i.URL) || len(i.Text) > 20000 {
				return fmt.Errorf("%w: check section items", ErrValidation)
			}
		}
	}
	return nil
}
func OrderedSections(items []Section) []Section {
	r := slices.Clone(items)
	slices.SortFunc(r, func(a, b Section) int {
		if a.Order < b.Order {
			return -1
		}
		if a.Order > b.Order {
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	return r
}
func (c Config) OrderedSlides() []Slide {
	r := slices.Clone(c.Slides)
	slices.SortFunc(r, func(a, b Slide) int {
		if a.Order < b.Order {
			return -1
		}
		if a.Order > b.Order {
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	return slices.DeleteFunc(r, func(s Slide) bool { return s.Hidden })
}

func (c Config) OrderedCategories() []Category {
	r := slices.Clone(c.Categories)
	slices.SortFunc(r, func(a, b Category) int {
		if a.Order < b.Order {
			return -1
		}
		if a.Order > b.Order {
			return 1
		}
		return strings.Compare(a.Slug, b.Slug)
	})
	return r
}
func (c Config) Template(kind, category, product string) Template {
	var fallback, categoryTemplate Template
	for _, t := range c.Templates {
		if t.Kind != kind {
			continue
		}
		if product != "" && t.Match == "product:"+product {
			return t
		}
		if category != "" && t.Match == "category:"+category {
			categoryTemplate = t
		}
		if t.Match == "" {
			fallback = t
		}
	}
	if categoryTemplate.ID != "" {
		return categoryTemplate
	}
	return fallback
}

type Draft struct {
	ID, Version, BaseRelease, CreatedBy int64
	State, Note                         string
	Updated                             time.Time
	Config                              Config
}
type Release struct {
	ID        int64
	Note      string
	Published time.Time
}

func (c *Config) ApplyCompatibleDefaults() {
	if c.Trust != nil && c.PolicyLinks != nil && c.FooterGroups != nil && c.Pages != nil {
		return
	}
	d := Defaults()
	if c.Trust == nil {
		c.Trust = d.Trust
	}
	if c.PolicyLinks == nil {
		c.PolicyLinks = d.PolicyLinks
	}
	if c.FooterGroups == nil {
		c.FooterGroups = d.FooterGroups
	}
	if c.Pages == nil {
		c.Pages = AcceptedContentPages(c.Settings.ShippingInformation)
	}
}

func contrast(a, b string) float64 {
	lum := func(v string) float64 {
		n, _ := strconv.ParseUint(strings.TrimPrefix(v, "#"), 16, 32)
		channel := func(v uint64) float64 {
			x := float64(v) / 255
			if x <= 0.04045 {
				return x / 12.92
			}
			return math.Pow((x+0.055)/1.055, 2.4)
		}
		return .2126*channel(n>>16&255) + .7152*channel(n>>8&255) + .0722*channel(n&255)
	}
	x, y := lum(a), lum(b)
	if x < y {
		x, y = y, x
	}
	return (x + .05) / (y + .05)
}
