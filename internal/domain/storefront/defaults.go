package storefront

func Defaults() Config {
	c := Config{Schema: 1, Theme: Theme{Logo: "/assets/images/brand/weevcrafts-logo.png", Primary: "#123f32", Accent: "#9d4031", Background: "#fffdf8", BodyFont: "DM Sans", HeadingFont: "Playfair Display"}, Settings: Settings{SupportEmail: "support@weevcrafts.com", ShippingInformation: "Delivery options and shipping charges are shown at checkout.", Policy: Policy{ShippingCents: 9900, FreeShippingCents: 150000}, Features: Features{true, true, true, true}}}
	c.Slides = []Slide{
		{ID: "craft", Order: 10, Title: "Crafted in India. Made to be cherished.", Text: "Discover handmade pieces from independent Indian makers — timeless crafts, richer homes and stronger communities.", Kicker: "HANDMADE · AUTHENTIC · MEANINGFUL", Image: "/assets/images/customer/craft-banner.webp", MobileImage: "/assets/images/customer/craft-mobile-banner.webp", Alt: "Indian artisan working in a craft studio", Button: Link{"Shop collections", "/products"}, Secondary: Link{"Meet the makers", "/makers"}},
		{ID: "textiles", Order: 20, Title: "Woven for everyday moments.", Text: "Explore textiles from independent Indian makers.", Image: "/assets/images/customer/textiles-banner.webp", Alt: "Handwoven textiles beside a wooden loom", Button: Link{"Explore textiles", "/products?category=textiles"}, Secondary: Link{"Meet the makers", "/makers"}},
		{ID: "living", Order: 30, Title: "Objects with a little more meaning.", Text: "Find handmade pieces for the spaces you love.", Image: "/assets/images/customer/living-banner.webp", Alt: "Handcrafted objects in a sunlit Indian home", Button: Link{"Explore home & living", "/products?category=home"}},
		{ID: "gifts", Order: 40, Title: "Gifts with a handmade heart.", Text: "Find thoughtful pieces to mark the moments that matter.", Image: "/assets/images/customer/gifts-banner.webp", Alt: "Wrapped gifts, ceramics and a wooden keepsake box", Button: Link{"Discover gifts", "/products?category=gifts"}},
		{ID: "metalwork", Order: 50, Title: "Traditions that shine.", Text: "Discover brass and metalwork made for meaningful spaces.", Image: "/assets/images/customer/metalwork-banner.webp", Alt: "Brass lamps and metalwork in a sunlit interior", Button: Link{"Explore metalwork", "/products?category=brass-metalwork"}},
	}
	r := Responsive{DesktopColumns: 6, TabletColumns: 3, MobileColumns: 2, Spacing: 24, MobileSpacing: 20}
	c.Homepage = []Section{
		{ID: "hero", Type: "hero", Order: 10},
		{ID: "categories", Type: "categories", Order: 20},
		{ID: "featured", Type: "products", Order: 30, Title: "Featured from our makers", Text: "Handpicked pieces, made with love across India.", Button: Link{"View all", "/products"}, Style: "grid", Selection: Selection{Mode: "featured", Limit: 6, Available: true}, Responsive: r},
		{ID: "arrivals", Type: "products", Order: 40, Title: "New arrivals", Text: "Fresh additions to discover and make your own.", Button: Link{"View all", "/products?sort=newest&availability=in-stock"}, Style: "rail", Selection: Selection{Mode: "newest", Limit: 8, Available: true}, Responsive: r},
		{ID: "by-category", Type: "products", Order: 50, Title: "Shop by category", Text: "Find your next piece, one craft at a time.", Style: "editorial", Selection: Selection{Mode: "category", Limit: 5, Available: true}, Responsive: r},
		{ID: "gifts", Type: "products", Order: 60, Title: "Gifts under ₹1,500", Text: "Thoughtful pieces for the people you cherish.", Style: "band", Selection: Selection{Mode: "category", Category: "gifts", Limit: 6, Available: true, MaxPrice: 149999}, Responsive: r},
		{ID: "makers", Type: "makers", Order: 70, Title: "Meet the Makers", Text: "Incredible people. Timeless skills. Brighter tomorrows.", Button: Link{"View all makers", "/makers"}},
		{ID: "collections", Type: "collections", Order: 80, Title: "Curated Collections", Text: "Thoughtfully chosen for meaningful living.", Items: []Tile{
			{Title: "For every occasion", Text: "Handloom pieces to cherish", Image: "/assets/images/customer/saree-maroon.webp", URL: "/products?category=textiles"},
			{Title: "For your home", Text: "Decor that tells a story", Image: "/assets/images/customer/living-banner.webp", URL: "/products?category=home"},
			{Title: "Thoughtful gifts", Text: "Handmade, heartfelt", Image: "/assets/images/customer/wooden-box.webp", URL: "/products?q=gift"},
			{Title: "Slow living", Text: "Mindful crafts for everyday life", Image: "/assets/images/customer/terracotta-planter.webp", URL: "/products?category=crafts"},
			{Title: "Festive favourites", Text: "Traditional craft, modern homes", Image: "/assets/images/customer/brass-elephant.webp", URL: "/products?q=brass"},
		}},
	}
	for i := range c.Slides {
		if c.Slides[i].Kicker == "" {
			c.Slides[i].Kicker = "HANDMADE · AUTHENTIC · MEANINGFUL"
		}
		if c.Slides[i].Secondary.URL == "" {
			c.Slides[i].Secondary = Link{"Meet the makers", "/makers"}
		}
	}
	for i := range c.Homepage {
		if c.Homepage[i].Type == "collections" {
			c.Homepage[i].Responsive = Responsive{DesktopColumns: 5, TabletColumns: 3, MobileColumns: 2, Spacing: 24, MobileSpacing: 20}
		}
	}
	c.Trust = []Tile{{Title: "Crafted in India", Text: "Independent Indian makers"}, {Title: "Secure Payments", Text: "Status shown in your orders"}, {Title: "Returns & Refunds", Text: "See our returns policy"}, {Title: "Domestic Delivery", Text: "To serviceable PIN codes"}, {Title: "Artisan made", Text: "Discover the people behind each piece"}}
	c.PolicyLinks = []Link{{"Privacy Policy", "/privacy"}, {"Terms of Service", "/terms"}}
	c.Navigation = []Link{{"Meet the Makers", "/makers"}}
	c.Footer = []Link{{"About us", "/about"}, {"Contact us", "/contact"}, {"Shipping", "/shipping"}, {"Returns & refunds", "/pages/returns"}, {"Frequently asked questions", "/faq"}, {"Terms & conditions", "/terms"}, {"Privacy policy", "/privacy"}}
	c.FooterGroups = []LinkGroup{
		{Title: "Shop", Links: []Link{{"All Categories", "/categories"}, {"New Arrivals", "/products?sort=newest"}, {"Home & Living", "/products?category=home"}, {"Thoughtful Gifts", "/products?q=gift"}}},
		{Title: "Help", Links: []Link{{"Track Order", "/orders"}, {"Returns & Refunds", "/pages/returns"}, {"Shipping Information", "/shipping"}, {"FAQs", "/faq"}}},
		{Title: "About", Links: []Link{{"Our Story", "/our-story"}, {"Meet the Makers", "/makers"}, {"Sustainability", "/sustainability"}, {"Contact Us", "/contact"}}},
		{Title: "Stay connected", Text: "Stories from the makers, thoughtful finds and news from WeevCrafts.", Links: []Link{{"Contact us", "/contact"}}},
	}
	for _, kind := range TemplateKinds {
		c.Templates = append(c.Templates, Template{ID: kind, Kind: kind})
	}
	c.Pages = AcceptedContentPages(c.Settings.ShippingInformation)
	return c
}
