package handlers

import (
	"bytes"
	"fmt"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	"image/png"
	"net/http"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/assets"
)

const (
	receiptForest = "124f43"
	receiptClay   = "b65a45"
	receiptInk    = "20322d"
	receiptMuted  = "5e675f"
	receiptLine   = "ddd6c8"
	receiptPaper  = "fbfaf6"
)

func receiptColor(value string) (int, int, int) {
	var r, g, b int
	_, _ = fmt.Sscanf(value, "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

func receiptSetText(pdf *fpdf.Fpdf, color string) {
	r, g, b := receiptColor(color)
	pdf.SetTextColor(r, g, b)
}
func receiptSetFill(pdf *fpdf.Fpdf, color string) {
	r, g, b := receiptColor(color)
	pdf.SetFillColor(r, g, b)
}
func receiptSetDraw(pdf *fpdf.Fpdf, color string) {
	r, g, b := receiptColor(color)
	pdf.SetDrawColor(r, g, b)
}

func receiptLabel(pdf *fpdf.Fpdf, text string, x, y, width float64) {
	receiptSetText(pdf, receiptForest)
	pdf.SetFont("DM", "B", 8)
	pdf.SetXY(x, y)
	pdf.CellFormat(width, 5, strings.ToUpper(text), "", 0, "L", false, 0, "")
}

func receiptValueRight(pdf *fpdf.Fpdf, text string, x, y, width float64) {
	receiptSetText(pdf, receiptInk)
	receiptFitFont(pdf, "", 10, text, width)
	pdf.SetXY(x, y)
	pdf.CellFormat(width, 6, text, "", 0, "R", false, 0, "")
}

func receiptFitFont(pdf *fpdf.Fpdf, style string, size float64, text string, width float64) {
	pdf.SetFont("DM", style, size)
	for ; size > 6 && pdf.GetStringWidth(text) > width-2; size -= 0.5 {
		pdf.SetFontSize(size - 0.5)
	}
}

func receiptPDF(order viewmodels.Order, loaders ...func(string) ([]byte, error)) ([]byte, error) {
	regular, err := assets.FS.ReadFile("fonts/dm-sans-400.ttf")
	if err != nil {
		return nil, err
	}
	medium, err := assets.FS.ReadFile("fonts/dm-sans-600.ttf")
	if err != nil {
		return nil, err
	}
	serif, err := assets.FS.ReadFile("fonts/cormorant-garamond-600.ttf")
	if err != nil {
		return nil, err
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("DM", "", regular)
	pdf.AddUTF8FontFromBytes("DM", "B", medium)
	pdf.AddUTF8FontFromBytes("Cormorant", "", serif)
	pdf.SetMargins(14, 12, 14)
	pdf.SetAutoPageBreak(false, 20)
	pdf.AliasNbPages("")
	pdf.SetFooterFunc(func() {
		receiptSetDraw(pdf, receiptLine)
		pdf.Line(14, 278, 196, 278)
		receiptSetText(pdf, receiptForest)
		pdf.SetFont("Cormorant", "", 13)
		pdf.SetXY(14, 281)
		pdf.CellFormat(125, 7, "Thank you for supporting independent makers.", "", 0, "L", false, 0, "")
		receiptSetText(pdf, receiptMuted)
		pdf.SetFont("DM", "", 8)
		pdf.SetXY(145, 283)
		pdf.CellFormat(51, 5, fmt.Sprintf("WeevCrafts · Page %d of {nb}", pdf.PageNo()), "", 0, "R", false, 0, "")
	})
	pdf.AddPage()
	pdf.SetCompression(true)
	pageW, _ := pdf.GetPageSize()
	contentW := pageW - 28
	receiptSetFill(pdf, receiptPaper)
	pdf.Rect(0, 0, pageW, 297, "F")
	if logo, readErr := assets.FS.ReadFile("images/brand/weevcrafts-logo.png"); readErr == nil {
		_ = pdf.RegisterImageOptionsReader("weevcrafts-logo", fpdf.ImageOptions{ImageType: "png", ReadDpi: true}, bytes.NewReader(logo))
		pdf.ImageOptions("weevcrafts-logo", 14, 13, 30, 0, false, fpdf.ImageOptions{ImageType: "png", ReadDpi: true}, 0, "")
	}
	receiptSetText(pdf, receiptForest)
	pdf.SetFont("Cormorant", "", 22)
	pdf.SetXY(48, 18)
	pdf.CellFormat(95, 9, "WeevCrafts", "", 0, "L", false, 0, "")
	receiptSetText(pdf, receiptMuted)
	pdf.SetFont("DM", "", 7)
	pdf.SetXY(48, 27)
	pdf.CellFormat(100, 5, "INDIAN ARTISAN MARKETPLACE", "", 0, "L", false, 0, "")
	receiptSetText(pdf, receiptForest)
	pdf.SetFont("DM", "B", 9)
	pdf.SetXY(pageW-74, 20)
	pdf.CellFormat(60, 5, "ORDER RECEIPT", "", 0, "R", false, 0, "")
	receiptSetDraw(pdf, receiptClay)
	pdf.SetLineWidth(0.8)
	pdf.Line(pageW-47, 29, pageW-14, 29)
	receiptSetDraw(pdf, receiptForest)
	pdf.SetLineWidth(0.4)
	pdf.Line(14, 37, pageW-14, 37)
	receiptSetText(pdf, receiptForest)
	pdf.SetFont("Cormorant", "", 30)
	pdf.SetXY(14, 47)
	pdf.CellFormat(contentW, 13, "Thank you for your order.", "", 0, "L", false, 0, "")
	receiptSetText(pdf, receiptMuted)
	pdf.SetFont("DM", "", 12)
	pdf.SetXY(14, 61)
	pdf.CellFormat(contentW, 7, "A handmade piece. A little more meaning.", "", 0, "L", false, 0, "")
	receiptSetFill(pdf, "e6f1e6")
	pdf.RoundedRect(14, 73, 76, 10, 5, "1234", "F")
	receiptSetText(pdf, receiptForest)
	pdf.SetFont("DM", "B", 9)
	pdf.SetXY(20, 76)
	pdf.CellFormat(65, 5, "Payment "+strings.Title(strings.TrimSpace(order.Payment)), "", 0, "L", false, 0, "")
	metaY := 91.0
	metaCols := []struct {
		label, value string
		x, w         float64
	}{
		{"Order number", order.Number, 14, 70}, {"Order date", order.Date, 90, 50}, {"Currency", "INR", 148, 40},
		{"Seller", receiptSeller(order), 14, 70}, {"Payment status", order.Payment, 90, 50}, {"Order status", order.Status, 148, 40},
	}
	for row := 0; row < len(metaCols); row += 3 {
		rowH := 25.0
		for _, item := range metaCols[row : row+3] {
			pdf.SetFont("DM", "", 10)
			if height := 15 + float64(len(pdf.SplitText(item.value, item.w)))*5; height > rowH {
				rowH = height
			}
		}
		for col, item := range metaCols[row : row+3] {
			receiptLabel(pdf, item.label, item.x, metaY, item.w)
			receiptSetText(pdf, receiptInk)
			pdf.SetFont("DM", "", 10)
			pdf.SetXY(item.x, metaY+6)
			pdf.MultiCell(item.w, 5, item.value, "", "L", false)
			if col != 2 {
				receiptSetDraw(pdf, receiptLine)
				pdf.SetLineWidth(0.3)
				pdf.Line(item.x+item.w+5, metaY-1, item.x+item.w+5, metaY+rowH-10)
			}
		}
		metaY += rowH
	}
	tableY := metaY + 2
	receiptSetFill(pdf, "f2eee4")
	pdf.RoundedRect(14, tableY, contentW, 14, 3, "1234", "F")
	receiptLabel(pdf, "Item", 20, tableY+4, 95)
	receiptLabel(pdf, "Qty", 120, tableY+4, 20)
	receiptLabel(pdf, "Unit price", 136, tableY+4, 30)
	receiptLabel(pdf, "Amount", 166, tableY+4, 30)
	rowY := tableY + 14
	for itemIndex, item := range order.Items {
		pdf.SetFont("DM", "", 10)
		nameLines := pdf.SplitText(item.Product.Name, 71)
		detail := strings.TrimSpace(item.VariantName + " " + item.Product.SKU)
		if detail == "" {
			detail = firstNonEmpty(item.Product.Category, "Handmade craft")
		}
		detail = detail + " · " + item.Product.Seller
		pdf.SetFont("DM", "", 8)
		detailLines := pdf.SplitText(detail, 71)
		rowH := 10 + float64(len(nameLines))*5 + float64(len(detailLines))*4
		if rowH < 25 {
			rowH = 25
		}
		if rowY+rowH > 268 {
			pdf.AddPage()
			receiptLabel(pdf, "Order "+order.Number+" · continued", 14, 16, 182)
			rowY = 30
		}
		receiptSetDraw(pdf, receiptLine)
		pdf.SetLineWidth(0.3)
		pdf.Line(14, rowY+rowH, pageW-14, rowY+rowH)
		receiptSetFill(pdf, "f2eee4")
		pdf.RoundedRect(20, rowY+3, 18, 19, 1, "1234", "F")
		var imageData []byte
		if strings.HasPrefix(item.Product.ImageURL, "/assets/") {
			imageData, _ = assets.FS.ReadFile(strings.TrimPrefix(item.Product.ImageURL, "/assets/"))
		} else if len(loaders) > 0 && loaders[0] != nil {
			imageData, _ = loaders[0](item.Product.ImageURL)
		}
		if thumbnail := receiptThumbnail(imageData); len(thumbnail) > 0 {
			name := fmt.Sprintf("receipt-item-%d", itemIndex)
			options := fpdf.ImageOptions{ImageType: "png"}
			pdf.RegisterImageOptionsReader(name, options, bytes.NewReader(thumbnail))
			pdf.ImageOptions(name, 20, rowY+3, 18, 18, false, options, 0, "")
		}
		receiptSetText(pdf, receiptInk)
		pdf.SetFont("DM", "", 10)
		pdf.SetXY(44, rowY+4)
		pdf.MultiCell(71, 5, item.Product.Name, "", "L", false)
		receiptSetText(pdf, receiptMuted)
		pdf.SetFont("DM", "", 8)
		pdf.SetXY(44, rowY+5+float64(len(nameLines))*5)
		pdf.MultiCell(71, 4, detail, "", "L", false)
		receiptValueRight(pdf, fmt.Sprintf("%d", item.Quantity), 120, rowY+8, 12)
		receiptValueRight(pdf, item.Product.Price, 136, rowY+8, 30)
		receiptValueRight(pdf, receiptMoney(item.LineTotalCents), 166, rowY+8, 30)
		rowY += rowH
	}
	if len(order.Items) == 0 {
		rowY += 12
	}
	// Totals and the two receipt-note lines occupy 83mm below the item table.
	// Keep a normal one-item receipt on one page without entering the footer.
	if rowY+83 > 270 {
		pdf.AddPage()
		receiptLabel(pdf, "Order "+order.Number+" · totals", 14, 16, 182)
		rowY = 30
	}
	totalY := rowY + 8
	receiptSetText(pdf, receiptMuted)
	pdf.SetFont("DM", "", 10)
	for _, line := range []struct{ label, value string }{{"Subtotal", order.Subtotal}, {"Shipping", order.Shipping}, {"Discount", order.Discount}} {
		pdf.SetFont("DM", "", 10)
		pdf.SetXY(112, totalY)
		pdf.CellFormat(30, 7, line.label, "", 0, "L", false, 0, "")
		receiptFitFont(pdf, "", 10, line.value, 54)
		pdf.SetXY(142, totalY)
		pdf.CellFormat(54, 7, line.value, "", 0, "R", false, 0, "")
		totalY += 8
		receiptSetDraw(pdf, receiptLine)
		pdf.Line(112, totalY-1, 196, totalY-1)
	}
	receiptSetFill(pdf, receiptForest)
	pdf.RoundedRect(110, totalY+2, 86, 15, 2, "1234", "F")
	receiptSetText(pdf, "ffffff")
	pdf.SetFont("Cormorant", "", 15)
	pdf.SetXY(116, totalY+5)
	totalLabel := "Total payable"
	if order.PaymentState == "captured" {
		totalLabel = "Total paid"
	} else if order.PaymentState == "refunded" {
		totalLabel = "Order total"
	}
	pdf.CellFormat(35, 7, totalLabel, "", 0, "L", false, 0, "")
	receiptFitFont(pdf, "B", 12, order.Total, 40)
	pdf.SetXY(151, totalY+5)
	pdf.CellFormat(40, 7, order.Total, "", 0, "R", false, 0, "")
	noteY := totalY + 27
	receiptSetDraw(pdf, receiptLine)
	pdf.Line(14, noteY, pageW-14, noteY)
	receiptLabel(pdf, "Receipt notes", 14, noteY+7, 70)
	receiptSetText(pdf, receiptMuted)
	pdf.SetFont("DM", "", 9)
	pdf.SetXY(14, noteY+14)
	pdf.MultiCell(contentW, 5, "This is an order receipt, not a GST tax invoice.\nPayment status reflects the recorded order.", "", "L", false)
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func receiptSeller(order viewmodels.Order) string {
	sellers := map[string]bool{}
	for _, item := range order.Items {
		if item.Product.Seller != "" {
			sellers[item.Product.Seller] = true
		}
	}
	if len(sellers) > 1 {
		return fmt.Sprintf("%d independent makers", len(sellers))
	}
	for _, item := range order.Items {
		if seller := strings.TrimSpace(item.Product.Seller); seller != "" {
			return seller
		}
	}
	return "Independent maker"
}

func receiptMoney(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	whole, paise := cents/100, cents%100
	text := fmt.Sprintf("%d", whole)
	for i := len(text) - 3; i > 0; i -= 3 {
		text = text[:i] + "," + text[i:]
	}
	return fmt.Sprintf("%s₹%s.%02d", sign, text, paise)
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func serveReceiptPDF(w http.ResponseWriter, order viewmodels.Order, loaders ...func(string) ([]byte, error)) {
	data, err := receiptPDF(order, loaders...)
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

func receiptThumbnail(data []byte) []byte {
	if len(data) == 0 || len(data) > 8<<20 {
		return nil
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 20000000 {
		return nil
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	width, height := 180, 180
	if config.Width > config.Height {
		height = config.Height * 180 / config.Width
	} else {
		width = config.Width * 180 / config.Height
	}
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	thumb := image.NewNRGBA(image.Rect(0, 0, 180, 180))
	draw.CatmullRom.Scale(thumb, image.Rect((180-width)/2, (180-height)/2, (180+width)/2, (180+height)/2), source, source.Bounds(), draw.Over, nil)
	var buffer bytes.Buffer
	if png.Encode(&buffer, thumb) != nil {
		return nil
	}
	return buffer.Bytes()
}
