package handlers

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestMarketplaceTaxInvoicePDFProducesValidPDF(t *testing.T) {
	doc := ports.InvoiceDocument{
		Number:            "INV/2026-27/0412",
		Date:              "20 Nov 2026",
		Currency:          "INR",
		Kind:              "Tax invoice",
		RelatedNumber:     "WC-20261120-0344",
		CustomerName:      "Ananya Sharma",
		CustomerAddress:   "48 Palm Grove Road, Indiranagar, Bengaluru, Karnataka - 560038",
		CustomerStateCode: "29",
		Seller: ports.SellerInvoiceProfile{
			SellerID:     101,
			LegalName:    "Kamala Arts",
			Address:      "Studio 14, Mithila Craft Enclave, Madhubani, Bihar - 847211",
			StateCode:    "10",
			GSTIN:        "10ABCDE1234F1Z5",
			Registration: "registered",
			Prefix:       "INV",
		},
		Lines: []ports.InvoiceLine{
			{
				ItemID:          1,
				Name:            "Tree of Life — Hand-Painted Madhubani Art",
				SKU:             "WC-113CA55A",
				HSN:             "9701",
				Quantity:        1,
				RateBasisPoints: 500,
				PaidCents:       194250,
				TaxableCents:    185000,
				TaxCents:        9250,
			},
		},
		TotalCents:   194250,
		TaxableCents: 185000,
		TaxCents:     9250,
		IGSTCents:    9250,
	}

	data, err := MarketplaceInvoicePDF(doc)
	if err != nil {
		t.Fatalf("MarketplaceInvoicePDF() error = %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.Contains(data, []byte("%%EOF")) {
		t.Fatal("MarketplaceInvoicePDF() did not produce a complete PDF")
	}
	if len(data) < 1000 {
		t.Fatalf("PDF unexpectedly small: %d bytes", len(data))
	}
}

func TestMarketplaceCreditNotePDFProducesValidPDF(t *testing.T) {
	doc := ports.InvoiceDocument{
		Number:            "CN/2026-27/0087",
		Date:              "24 Nov 2026",
		Currency:          "INR",
		Kind:              "Credit note",
		RelatedNumber:     "INV/2026-27/0412",
		CustomerName:      "Ananya Sharma",
		CustomerAddress:   "Bengaluru, Karnataka - 560038",
		CustomerStateCode: "29",
		Seller: ports.SellerInvoiceProfile{
			SellerID:  101,
			LegalName: "Kamala Arts",
			Address:   "Madhubani, Bihar - 847211",
			StateCode: "10",
			GSTIN:     "10ABCDE1234F1Z5",
		},
		Lines: []ports.InvoiceLine{
			{
				ItemID:          1,
				Name:            "Tree of Life — Return Settled",
				SKU:             "WC-113CA55A",
				HSN:             "9701",
				Quantity:        1,
				RateBasisPoints: 500,
				PaidCents:       194250,
				TaxableCents:    185000,
				TaxCents:        9250,
			},
		},
		TotalCents:   194250,
		TaxableCents: 185000,
		TaxCents:     9250,
		IGSTCents:    9250,
	}

	data, err := MarketplaceInvoicePDF(doc)
	if err != nil {
		t.Fatalf("MarketplaceInvoicePDF(Credit Note) error = %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.Contains(data, []byte("%%EOF")) {
		t.Fatal("Credit Note did not produce a complete PDF")
	}
}

func TestMarketplacePackingSlipPDFProducesValidPDF(t *testing.T) {
	pkg := ports.MarketplacePackage{
		ID:     142,
		Status: "ready_to_pack",
		Shipment: ports.Shipment{
			Courier:            "Blue Dart Express",
			AWB:                "BD-982140918",
			DeliveryPostalCode: "560038",
			PickupLocation:     "Madhubani Dispatch Station",
		},
		Items: []ports.MarketplacePackageItem{
			{
				Name:     "Tree of Life — Hand-Painted Wall Art",
				SKU:      "WC-113CA55A",
				Quantity: 1,
			},
			{
				Name:     "Handwoven Cotton Tote",
				SKU:      "WC-88B41C0D",
				Quantity: 2,
			},
		},
	}

	data, err := marketplacePackingSlipPDF("WC-20261120-0344", pkg)
	if err != nil {
		t.Fatalf("marketplacePackingSlipPDF() error = %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.Contains(data, []byte("%%EOF")) {
		t.Fatal("marketplacePackingSlipPDF() did not produce a complete PDF")
	}
}

func TestGenerateSamplePDFs(t *testing.T) {
	outDir := t.TempDir()

	// 1. Order Receipt
	order := viewmodels.Order{
		Number:       "WC-20261120-0344",
		Date:         "20 Nov 2026, 15:44 IST",
		Status:       "Confirmed",
		Payment:      "UPI (Razorpay)",
		ContactName:  "Ananya Sharma",
		ContactPhone: "48 Palm Grove Road, Indiranagar, Bengaluru - 560038\nPhone: +91 98450 23189",
		Subtotal:     "₹3,130.00",
		Shipping:     "₹55.72",
		Discount:     "- ₹0.00",
		Total:        "₹3,185.72",
		Items: []viewmodels.CartItem{
			{
				Product: viewmodels.Product{
					Name:     "Tree of Life — Hand-Painted Madhubani Art",
					Price:    "₹1,850.00",
					SKU:      "WC-113CA55A",
					Seller:   "Kamala Arts",
					Category: "Mithila Painting on Handmade Paper",
					ImageURL: "/assets/images/customer/madhubani-tree.png",
				},
				Quantity:       1,
				LineTotalCents: 185000,
			},
			{
				Product: viewmodels.Product{
					Name:     "Handwoven Indigo Cotton Craft Tote",
					Price:    "₹640.00",
					SKU:      "WC-88B41C0D",
					Seller:   "Kala Weaves",
					Category: "Natural Dye Handloom Cotton",
					ImageURL: "/assets/images/customer/blue-tote.png",
				},
				Quantity:       2,
				LineTotalCents: 128000,
			},
		},
	}
	orderBytes, err := receiptPDF(order)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "01-order-receipt.pdf"), orderBytes, 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Tax Invoice
	taxDoc := ports.InvoiceDocument{
		Number:            "INV/2026-27/0412",
		Date:              "20 Nov 2026",
		Currency:          "INR",
		Kind:              "Tax invoice",
		RelatedNumber:     "WC-20261120-0344",
		CustomerName:      "Ananya Sharma",
		CustomerAddress:   "48 Palm Grove Road, Indiranagar, Bengaluru, Karnataka - 560038",
		CustomerStateCode: "29",
		Seller: ports.SellerInvoiceProfile{
			SellerID:     101,
			LegalName:    "Kamala Arts",
			Address:      "Studio 14, Mithila Craft Enclave, Madhubani, Bihar - 847211",
			StateCode:    "10",
			GSTIN:        "10ABCDE1234F1Z5",
			Registration: "registered",
			Prefix:       "INV",
		},
		Lines: []ports.InvoiceLine{
			{
				ItemID:          1,
				Name:            "Tree of Life — Hand-Painted Madhubani Art",
				SKU:             "WC-113CA55A",
				HSN:             "9701",
				Quantity:        1,
				RateBasisPoints: 500,
				PaidCents:       194250,
				TaxableCents:    185000,
				TaxCents:        9250,
			},
			{
				ItemID:          2,
				Name:            "Handwoven Indigo Cotton Craft Tote",
				SKU:             "WC-88B41C0D",
				HSN:             "4202",
				Quantity:        2,
				RateBasisPoints: 500,
				PaidCents:       134400,
				TaxableCents:    128000,
				TaxCents:        6400,
			},
			{
				ItemID:          3,
				Name:            "Marketplace Logistics & Shipping Charges",
				SKU:             "LOG-EXPRESS",
				HSN:             "9968",
				Quantity:        1,
				RateBasisPoints: 1800,
				PaidCents:       6575,
				TaxableCents:    5572,
				TaxCents:        1003,
			},
		},
		TotalCents:   335225,
		TaxableCents: 318572,
		TaxCents:     16653,
		IGSTCents:    16653,
	}
	taxBytes, err := MarketplaceInvoicePDF(taxDoc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "02-seller-tax-invoice.pdf"), taxBytes, 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Credit Note
	cnDoc := ports.InvoiceDocument{
		Number:            "CN/2026-27/0087",
		Date:              "24 Nov 2026",
		Currency:          "INR",
		Kind:              "Credit note",
		RelatedNumber:     "INV/2026-27/0412",
		CustomerName:      "Ananya Sharma",
		CustomerAddress:   "Bengaluru, Karnataka - 560038",
		CustomerStateCode: "29",
		Seller: ports.SellerInvoiceProfile{
			SellerID:  101,
			LegalName: "Kamala Arts",
			Address:   "Studio 14, Mithila Craft Enclave, Madhubani, Bihar - 847211",
			StateCode: "10",
			GSTIN:     "10ABCDE1234F1Z5",
		},
		Lines: []ports.InvoiceLine{
			{
				ItemID:          1,
				Name:            "Tree of Life — Hand-Painted Wall Art (Damage Claim)",
				SKU:             "WC-113CA55A",
				HSN:             "9701",
				Quantity:        1,
				RateBasisPoints: 500,
				PaidCents:       194250,
				TaxableCents:    185000,
				TaxCents:        9250,
			},
			{
				ItemID:          2,
				Name:            "Outbound Logistics Charges (Refunded in Full)",
				SKU:             "LOG-EXPRESS",
				HSN:             "9968",
				Quantity:        1,
				RateBasisPoints: 1800,
				PaidCents:       6575,
				TaxableCents:    5572,
				TaxCents:        1003,
			},
		},
		TotalCents:   200825,
		TaxableCents: 190572,
		TaxCents:     10253,
		IGSTCents:    10253,
	}
	cnBytes, err := MarketplaceInvoicePDF(cnDoc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "03-credit-note.pdf"), cnBytes, 0644); err != nil {
		t.Fatal(err)
	}

	// 4. Packing Slip
	pkg := ports.MarketplacePackage{
		ID:     142,
		Status: "ready_to_pack",
		Shipment: ports.Shipment{
			Courier:            "Blue Dart Express (Shiprocket)",
			AWB:                "BD-982140918",
			DeliveryPostalCode: "560038",
			PickupLocation:     "Ananya Sharma, Indiranagar, Bengaluru - 560038",
		},
		Items: []ports.MarketplacePackageItem{
			{
				Name:     "Tree of Life — Hand-Painted Wall Art",
				SKU:      "WC-113CA55A",
				Quantity: 1,
			},
			{
				Name:     "Handwoven Indigo Cotton Craft Tote",
				SKU:      "WC-88B41C0D",
				Quantity: 2,
			},
			{
				Name:     "Artisan Story & Authenticity Certificate",
				SKU:      "CERT-AUTH",
				Quantity: 1,
			},
		},
	}
	pkgBytes, err := marketplacePackingSlipPDF("WC-20261120-0344", pkg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "04-packing-slip.pdf"), pkgBytes, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestPDFIndianNumberingWords(t *testing.T) {
	cases := []struct {
		cents    int64
		expected string
	}{
		{0, "Zero Rupees Only"},
		{10000, "Indian Rupees One Hundred Only"},
		{335225, "Indian Rupees Three Thousand Three Hundred Fifty-Two and Twenty-Five Paise Only"},
		{185000, "Indian Rupees One Thousand Eight Hundred Fifty Only"},
		{10000000, "Indian Rupees One Lakh Only"},
	}
	for _, c := range cases {
		result := pdfAmountInWords(c.cents)
		if !strings.EqualFold(result, c.expected) {
			t.Errorf("pdfAmountInWords(%d) = %q, expected %q", c.cents, result, c.expected)
		}
	}
}
