// A loopback-only rendering fixture. Browser tests stub the API separately;
// this process has no database, credentials, or payment integration.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/pages"
)

func main() {
	port := flag.Int("port", 18086, "loopback fixture port")
	flag.Parse()
	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("web/assets"))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cart" && r.URL.Path != "/checkout" {
			http.NotFound(w, r)
			return
		}
		quantity := 1
		if cookie, err := r.Cookie("layout_quantity"); err == nil {
			if value, err := strconv.Atoi(cookie.Value); err == nil && value >= 0 && value <= 3 {
				quantity = value
			}
		}
		p := viewmodels.CustomerPage{
			Route: r.URL.Path, Title: "Customer layout fixture", Brand: "WeevCrafts",
			UserName: "Layout customer", UserPhone: "9876543210", AccountUserID: 42, IsCustomer: true, Authenticated: true,
			CSRFToken: "layout-only", PaymentReady: true, PaymentMode: "razorpay",
			CartCount: quantity, ItemsTotal: "INR 99,999.00", ShippingTotal: "INR 99.00", DiscountTotal: "INR 0.00", Total: "INR 1,00,098.00",
		}
		if quantity > 0 {
			p.Cart = []viewmodels.CartItem{{
				Product:     viewmodels.Product{ID: 1, VariantID: 101, Slug: "layout-mug", Name: "Hand painted ceramic coffee mug with traditional floral patterns", Seller: "Independent Indian Ceramics Workshop", Location: "India", Badge: "In stock", Price: "INR 99,999.00", CompareAt: "INR 1,09,999.00", ImageURL: "/assets/images/customer/ceramic-mugs.webp"},
				VariantName: "Standard", Quantity: quantity, Delivery: "Delivery to serviceable PIN codes in India",
			}}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/cart" {
			_ = pages.Cart(p).Render(r.Context(), w)
		} else {
			_ = pages.Checkout(p).Render(r.Context(), w)
		}
	})
	log.Fatal(http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", *port), mux))
}
