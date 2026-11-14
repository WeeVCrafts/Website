package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/a-h/templ"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	adminpages "github.com/wecratfs/commerce/web/admin"
	"github.com/wecratfs/commerce/web/pages"
	"github.com/wecratfs/commerce/web/selleradmin"
)

const confirmationOrder = "QA-PAYMENT"

func registerRecoveryRoutes(mux *http.ServeMux) {
	mug := viewmodels.Product{
		ID: 1, VariantID: 101, Slug: "layout-mug", Name: "Ceramic mug",
		Price: "₹500", PriceCents: 50000, Seller: "Fixture maker", InStock: true, Available: 8,
		ImageURL: "/assets/images/customer/ceramic-mugs.webp",
		Variants: []viewmodels.ProductVariant{
			{ID: "101", Name: "Blue", SKU: "MUG-B", Price: "₹500", Available: "8", InStock: true},
			{ID: "102", Name: "Red", SKU: "MUG-R", Price: "₹500", Available: "2", InStock: true},
			{ID: "103", Name: "White", SKU: "MUG-W", Price: "₹500", Available: "0"},
		},
	}
	mux.HandleFunc("GET /search", func(w http.ResponseWriter, r *http.Request) {
		p := recoveryCustomerPage(r)
		p.Query = r.URL.Query().Get("q")
		renderFixture(w, r, pages.Listing(p))
	})
	mux.HandleFunc("GET /products/layout-mug", func(w http.ResponseWriter, r *http.Request) {
		p := recoveryCustomerPage(r)
		p.Product = mug
		renderFixture(w, r, pages.ProductDetail(p))
	})
	mux.HandleFunc("GET /ui/search-suggest", func(w http.ResponseWriter, r *http.Request) {
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		var products []viewmodels.Product
		switch query {
		case "ceramic":
			last := mug
			last.Name, last.Slug = "Ceramic bowl", "layout-bowl"
			products = []viewmodels.Product{mug, last}
		case "mugs":
			products = []viewmodels.Product{mug}
		}
		renderFixture(w, r, pages.SearchSuggestions(products, query))
	})
	mux.HandleFunc("GET /orders/"+confirmationOrder+"/confirmation", func(w http.ResponseWriter, r *http.Request) {
		renderFixture(w, r, pages.OrderConfirmation(recoveryPaymentPage(r)))
	})
	mux.HandleFunc("GET /ui/orders/status", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("order") != confirmationOrder {
			http.NotFound(w, r)
			return
		}
		renderFixture(w, r, pages.OrderConfirmationContent(recoveryPaymentPage(r)))
	})

	var orders sync.Map
	mux.HandleFunc("GET /seller-admin/orders/{order}", func(w http.ResponseWriter, r *http.Request) {
		number := r.PathValue("order")
		status, tracking, revision := "out_for_delivery", "QA-OLD-TRACKING", "7"
		if _, delivered := orders.Load(number); delivered {
			status, tracking, revision = "delivered", "QA-NEW-TRACKING", "8"
		}
		returnPage, _ := strconv.Atoi(r.URL.Query().Get("page"))
		order := viewmodels.SellerAdminOrderDetail{
			SellerID: "42", Number: number, Date: "14 November", Customer: "Fixture customer",
			OrderStatus: "paid", Status: status, Payment: "captured", Revision: revision,
			RecipientName: "Fixture customer", Line1: "1 Craft Lane", City: "Pune",
			State: "Maharashtra", PostalCode: "411001", CountryCode: "IN",
			Carrier: "India Post", TrackingNumber: tracking, Subtotal: "₹500",
			Items: []viewmodels.SellerAdminOrderItem{{Name: mug.Name, SKU: mug.Variants[0].SKU, Quantity: "1", UnitPrice: "₹500", LineTotal: "₹500"}},
		}
		if number == "QA-SHIPPING" {
			order.Status = "packing"
			order.Shiprocket = &viewmodels.SellerShiprocketView{PickupLocation: "Fixture depot", CanCreate: true, CanManage: true}
			if r.URL.Query().Has("created") || r.URL.Query().Get("couriers") == "1" {
				order.Shiprocket.CanCreate = false
				order.Shiprocket.Managed = true
				order.Shiprocket.Reference = "WC-FIXTURE"
				order.Shiprocket.Status = "new"
				order.Shiprocket.CanChooseCourier = true
			}
			if r.URL.Query().Get("couriers") == "1" {
				order.Shiprocket.CourierOptionsRequested = true
				order.Shiprocket.CourierOptions = []viewmodels.SellerCourierOption{{ID: "51", Name: "Xpressbees Surface", Rate: "₹54.00", Days: "4"}}
			}
			if r.URL.Query().Has("picked-up") {
				order.Shiprocket.CanCreate = false
				order.Shiprocket.CanChooseCourier = false
				order.Shiprocket.Managed = true
				order.Shiprocket.Reference = "WC-FIXTURE"
				order.Shiprocket.AWB = "TEST-WAYBILL"
				order.Shiprocket.PickupReference = "TEST-PICKUP"
				if r.URL.Query().Has("manifest") {
					order.Shiprocket.ManifestURL = "https://shiprocket.in/test-manifest.pdf"
				}
			}
		}
		p := viewmodels.SellerAdminPage{
			Active: "orders", Workspace: "orders", Title: "Seller order fixture", DisplayName: "Fixture seller",
			SellerName: "Fixture maker", SellerStatus: "active", RoleLabel: "Seller", CanOrders: true, CanFulfill: true,
			CSRFToken: "fixture-only", OrderDetail: &order, Query: r.URL.Query().Get("q"),
			Tab: r.URL.Query().Get("tab"), OrderReturnPage: returnPage,
		}
		renderFixture(w, r, selleradmin.PortalOrderDetail(p))
	})
	mux.HandleFunc("POST /seller-admin/orders/fulfill", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("csrf_token") != "fixture-only" || r.FormValue("status") != "delivered" || r.FormValue("expected_revision") != "7" {
			http.Error(w, "Invalid fixture transition", http.StatusBadRequest)
			return
		}
		number := r.FormValue("order_number")
		if number == "" {
			http.Error(w, "Missing fixture order", http.StatusBadRequest)
			return
		}
		orders.Store(number, true)
		http.Redirect(w, r, "/seller-admin/orders/"+number, http.StatusSeeOther)
	})
	mux.HandleFunc("POST /seller-admin/orders/shiprocket", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("csrf_token") != "fixture-only" || r.FormValue("order_number") != "QA-SHIPPING" {
			http.Error(w, "Invalid parcel fixture", http.StatusBadRequest)
			return
		}
		switch r.FormValue("action") {
		case "create":
			http.Redirect(w, r, "/seller-admin/orders/QA-SHIPPING?shiprocket&created", http.StatusSeeOther)
		case "manifest":
			http.Redirect(w, r, "/seller-admin/orders/QA-SHIPPING?picked-up&manifest", http.StatusSeeOther)
		default:
			http.Error(w, "Shiprocket declined this action.", http.StatusUnprocessableEntity)
		}
	})
	mux.HandleFunc("POST /admin/delivery-exceptions", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(8192); err != nil {
			http.Error(w, "Invalid queue fixture", http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()
		if r.FormValue("csrf_token") != "fixture-only" || r.FormValue("action") != "resolve" || r.FormValue("kind") != "cancelled_parcel" {
			http.Error(w, "Invalid resolution fixture", http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, "/admin/delivery-exceptions?notice=saved", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /admin/orders", func(w http.ResponseWriter, r *http.Request) {
		renderFixture(w, r, adminpages.PortalOrders(fixtureAdminOrderPage(r)))
	})
	mux.HandleFunc("GET /admin/delivery-exceptions", func(w http.ResponseWriter, r *http.Request) {
		p := fixtureAdminOrderPage(r)
		p.CSRFToken = "fixture-only"
		p.ActorID = 1
		cases := []ports.DeliveryException{{OrderID: 8, SellerID: 42, OrderNumber: "QA-SHIPPING", Kind: "cancelled_parcel", ShipmentStatus: "CANCELLED", CanResolve: true}}
		renderFixture(w, r, adminpages.DeliveryExceptions(p, cases))
	})
	mux.HandleFunc("GET /ui/admin/orders/status", func(w http.ResponseWriter, r *http.Request) {
		p := fixtureAdminOrderPage(r)
		if p.OrderQueueUnavailable {
			http.Error(w, "Order queue unavailable.", http.StatusServiceUnavailable)
			return
		}
		renderFixture(w, r, adminpages.OrderQueueRegion(p))
	})
}

func fixtureAdminOrderPage(r *http.Request) viewmodels.AdminPage {
	query := r.URL.Query()
	p := viewmodels.AdminPage{
		Active: "orders", NavActive: "orders", Workspace: "orders", PortalRoot: "/admin",
		Title: "Admin order queue fixture", CanManageDelivery: true, DisplayName: "Fixture operator", PortalLabel: "Admin", RoleLabel: "Admin",
		Query: query.Get("q"), OrderStage: query.Get("stage"), OrderQueueUnavailable: query.Has("unavailable"),
	}
	if query.Has("populated") {
		p.Orders = []viewmodels.AdminOrder{{Number: "QA-ADMIN-ORDER", Customer: "Fixture customer", Status: "paid"}}
	}
	return p
}

func recoveryCustomerPage(r *http.Request) viewmodels.CustomerPage {
	return viewmodels.CustomerPage{
		Route: r.URL.Path, Title: "Marketplace recovery fixture", Brand: "Weevcrafts",
		IsCustomer: true, Authenticated: true, AccountUserID: 42, UserName: "Fixture customer",
		CSRFToken: "fixture-only", PaymentReady: true, PaymentMode: "razorpay",
	}
}

func recoveryPaymentPage(r *http.Request) viewmodels.CustomerPage {
	p := recoveryCustomerPage(r)
	p.SelectedOrder = viewmodels.Order{Number: confirmationOrder, State: "pending_payment", PaymentState: "pending", PaymentInProgress: true, Total: "₹500"}
	if r.URL.Query().Get("fixture_payment") == "captured" {
		p.SelectedOrder.State = "paid"
		p.SelectedOrder.PaymentState = "captured"
		p.SelectedOrder.PaymentInProgress = false
	}
	return p
}

func renderFixture(w http.ResponseWriter, r *http.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		log.Print(err)
	}
}
