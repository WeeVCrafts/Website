package handlers

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/go-pdf/fpdf"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/assets"
)

func receiptPDF(order viewmodels.Order) ([]byte, error) {
	font, err := assets.FS.ReadFile("fonts/dm-sans-400.ttf")
	if err != nil {
		return nil, err
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("DM", "", font)
	pdf.SetMargins(18, 18, 18)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()
	pdf.SetFont("DM", "", 22)
	pdf.MultiCell(0, 12, "WeevCrafts — Order receipt", "", "L", false)
	pdf.SetFont("DM", "", 11)
	for _, line := range []string{"Order: " + order.Number, "Placed: " + order.Date, "Order status: " + order.Status, "Payment status: " + order.Payment, ""} {
		pdf.MultiCell(0, 7, line, "", "L", false)
	}
	for _, item := range order.Items {
		pdf.MultiCell(0, 7, fmt.Sprintf("%s\nQuantity: %d · Unit price: %s", item.Product.Name, item.Quantity, item.Product.Price), "B", "L", false)
		pdf.Ln(3)
	}
	pdf.Ln(5)
	for _, line := range []string{"Subtotal: " + order.Subtotal, "Shipping: " + order.Shipping, "Coupon discount: " + order.Discount, "Total paid / payable: " + order.Total, "", "This is an order receipt, not a GST tax invoice. Payment status above reflects the recorded order; the total is not proof of payment."} {
		pdf.MultiCell(0, 7, line, "", "L", false)
	}
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func serveReceiptPDF(w http.ResponseWriter, order viewmodels.Order) {
	data, err := receiptPDF(order)
	if err != nil {
		http.Error(w, "Receipt is unavailable. Please try again.", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="weevcrafts-order-receipt.pdf"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}
