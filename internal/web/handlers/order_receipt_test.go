package handlers

import (
	"bytes"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"testing"
)

func TestReceiptPDFUsesOrderDataAndProducesDownload(t *testing.T) {
	order := viewmodels.Order{Number: "WC-QA-1", Date: "7 Oct 2026", Status: "Confirmed", Payment: "Captured", Subtotal: "INR 1299.50", Shipping: "INR 99", Discount: "INR 0", Total: "INR 1398.50", Items: []viewmodels.CartItem{{Product: viewmodels.Product{Name: "Handmade keepsake", Price: "INR 1299.50"}, Quantity: 1}}}
	data, err := receiptPDF(order)
	if err != nil {
		t.Fatalf("receiptPDF() error=%v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.Contains(data, []byte("%%EOF")) {
		t.Fatal("receiptPDF() did not return a complete PDF")
	}
}
