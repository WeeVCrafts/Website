package handlers

import (
	"bytes"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"image"
	"image/png"
	"strings"
	"testing"
)

func TestReceiptThumbnailRejectsInvalidDataAndPreservesBounds(t *testing.T) {
	if receiptThumbnail([]byte("<svg>not a supported photograph</svg>")) != nil {
		t.Fatal("invalid image accepted")
	}
	var source bytes.Buffer
	if err := png.Encode(&source, image.NewRGBA(image.Rect(0, 0, 400, 200))); err != nil {
		t.Fatal(err)
	}
	result := receiptThumbnail(source.Bytes())
	config, _, err := image.DecodeConfig(bytes.NewReader(result))
	if err != nil || config.Width != 180 || config.Height != 180 {
		t.Fatalf("bad thumbnail dimensions: %+v %v", config, err)
	}
}

func TestReceiptPDFPaginatesLongMultiSellerOrders(t *testing.T) {
	order := viewmodels.Order{Number: "WC-LARGE-ORDER", PaymentState: "pending", Payment: "Pending", Subtotal: "INR 9999999.00", Shipping: "INR 0", Discount: "INR 0", Total: "INR 9999999.00"}
	for i := 0; i < 35; i++ {
		order.Items = append(order.Items, viewmodels.CartItem{Product: viewmodels.Product{Name: strings.Repeat("Handmade keepsake ", 10), Seller: strings.Repeat("Independent maker ", 3), Price: "INR 999999.00", SKU: "UNIQUE-SKU"}, Quantity: 10, LineTotalCents: 99999900})
	}
	data, err := receiptPDF(order)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(data, []byte("/Type /Page\n")) < 2 {
		t.Fatal("long order did not paginate")
	}
}

func TestReceiptPDFUsesOrderDataAndProducesDownload(t *testing.T) {
	order := viewmodels.Order{Number: "WC-QA-1", Date: "7 Oct 2026", Status: "Confirmed", Payment: "Captured", Subtotal: "INR 1299.50", Shipping: "INR 99", Discount: "INR 0", Total: "INR 1398.50", Items: []viewmodels.CartItem{{Product: viewmodels.Product{Name: "Handmade keepsake", Price: "INR 1299.50"}, Quantity: 1}}}
	data, err := receiptPDF(order)
	if err != nil {
		t.Fatalf("receiptPDF() error=%v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.Contains(data, []byte("%%EOF")) {
		t.Fatal("receiptPDF() did not return a complete PDF")
	}
	if pages := bytes.Count(data, []byte("/Type /Page\n")); pages != 1 {
		t.Fatalf("ordinary single-item receipt should fit on one page, got %d", pages)
	}
}
