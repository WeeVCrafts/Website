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
	receiptForest     = "0e3d30"
	receiptForestMid  = "165342"
	receiptForestTint = "edf5f1"
	receiptClay       = "b84e34"
	receiptInk        = "1b2823"
	receiptMuted      = "627168"
	receiptLine       = "e2dac9"
	receiptLineStrong = "c8beab"
	receiptCardBg     = "f8f6f0"
	receiptCardBdr    = "e6dfd1"
	receiptPaper      = "fcfbf8"
	receiptGold       = "c59436"
)

func receiptColor(value string) (int, int, int) {
	return pdfColor(value)
}

func receiptSetText(pdf *fpdf.Fpdf, color string) {
	pdfSetText(pdf, color)
}

func receiptSetFill(pdf *fpdf.Fpdf, color string) {
	pdfSetFill(pdf, color)
}

func receiptSetDraw(pdf *fpdf.Fpdf, color string) {
	pdfSetDraw(pdf, color)
}

func receiptLabel(pdf *fpdf.Fpdf, text string, x, y, width float64) {
	pdfSetText(pdf, receiptForest)
	pdf.SetFont("DM", "B", 7)
	pdf.SetXY(x, y)
	pdf.CellFormat(width, 4, strings.ToUpper(text), "", 0, "L", false, 0, "")
}

func receiptValueRight(pdf *fpdf.Fpdf, text string, x, y, width float64) {
	pdfSetText(pdf, receiptInk)
	receiptFitFont(pdf, "", 9, text, width)
	pdf.SetXY(x, y)
	pdf.CellFormat(width, 5, text, "", 0, "R", false, 0, "")
}

func receiptFitFont(pdf *fpdf.Fpdf, style string, size float64, text string, width float64) {
	pdf.SetFont("DM", style, size)
	for ; size > 5.5 && pdf.GetStringWidth(text) > width-2; size -= 0.5 {
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
	serif, err := assets.FS.ReadFile("fonts/playfair-display-600.ttf")
	if err != nil {
		serif, _ = assets.FS.ReadFile("fonts/cormorant-garamond-600.ttf")
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("DM", "", regular)
	pdf.AddUTF8FontFromBytes("DM", "B", medium)
	if len(serif) > 0 {
		pdf.AddUTF8FontFromBytes("Serif", "B", serif)
		pdf.AddUTF8FontFromBytes("Cormorant", "", serif)
	} else {
		pdf.AddUTF8FontFromBytes("Serif", "B", medium)
		pdf.AddUTF8FontFromBytes("Cormorant", "", medium)
	}

	pdf.SetMargins(14, 12, 14)
	pdf.SetAutoPageBreak(false, 18)
	pdf.AliasNbPages("")

	pdf.SetFooterFunc(func() {
		receiptSetDraw(pdf, receiptLine)
		pdf.SetLineWidth(0.3)
		pdf.Line(14, 281, 196, 281)

		receiptSetText(pdf, receiptForest)
		pdf.SetFont("DM", "B", 7.5)
		pdf.SetXY(14, 283)
		pdf.CellFormat(90, 5, "WeevCrafts Marketplace · Verified Artisan Network", "", 0, "L", false, 0, "")

		receiptSetText(pdf, receiptMuted)
		pdf.SetFont("DM", "", 7)
		pdf.SetXY(105, 283)
		pdf.CellFormat(91, 5, fmt.Sprintf("7-Day Replacement Guarantee · Page %d of {nb}", pdf.PageNo()), "", 0, "R", false, 0, "")
	})

	pdf.AddPage()
	pdf.SetCompression(true)
	pageW, _ := pdf.GetPageSize()
	contentW := pageW - 28

	// Background paper tint
	receiptSetFill(pdf, receiptPaper)
	pdf.Rect(0, 0, pageW, 297, "F")

	// ==========================================
	// BRAND HEADER
	// ==========================================
	if logo, readErr := assets.FS.ReadFile("images/brand/weevcrafts-logo.png"); readErr == nil {
		_ = pdf.RegisterImageOptionsReader("weevcrafts-logo", fpdf.ImageOptions{ImageType: "png", ReadDpi: true}, bytes.NewReader(logo))
		pdf.ImageOptions("weevcrafts-logo", 14, 11, 14, 14, false, fpdf.ImageOptions{ImageType: "png", ReadDpi: true}, 0, "")
	}

	receiptSetText(pdf, receiptForest)
	pdf.SetFont("Serif", "B", 18)
	pdf.SetXY(31, 13)
	pdf.CellFormat(70, 7, "WeevCrafts", "", 0, "L", false, 0, "")

	receiptSetText(pdf, receiptMuted)
	pdf.SetFont("DM", "B", 6.5)
	pdf.SetXY(31, 20)
	pdf.CellFormat(70, 4, "INDIAN ARTISAN MARKETPLACE", "", 0, "L", false, 0, "")

	// Right badge
	receiptSetFill(pdf, receiptForest)
	pdf.RoundedRect(pageW-76, 11, 62, 6.5, 1.5, "1234", "F")
	receiptSetText(pdf, "ffffff")
	pdf.SetFont("DM", "B", 7)
	pdf.SetXY(pageW-76, 11)
	pdf.CellFormat(62, 6.5, "CUSTOMER ORDER RECEIPT", "", 0, "C", false, 0, "")

	receiptSetText(pdf, receiptInk)
	pdf.SetFont("DM", "B", 9.5)
	pdf.SetXY(pageW-76, 18.5)
	pdf.CellFormat(62, 5, order.Number, "", 0, "R", false, 0, "")

	receiptSetText(pdf, receiptMuted)
	pdf.SetFont("DM", "", 7.5)
	pdf.SetXY(pageW-76, 23.5)
	orderDate := firstNonEmpty(order.Date, "Recorded Order")
	pdf.CellFormat(62, 4, orderDate, "", 0, "R", false, 0, "")

	// Divider line under brandbar
	receiptSetDraw(pdf, receiptForest)
	pdf.SetLineWidth(0.6)
	pdf.Line(14, 29, pageW-14, 29)

	// ==========================================
	// TITLE & SUBTITLE
	// ==========================================
	receiptSetText(pdf, receiptForest)
	pdf.SetFont("Serif", "B", 17)
	pdf.SetXY(14, 33)
	pdf.CellFormat(contentW, 8, "Thank you for your order.", "", 0, "L", false, 0, "")

	receiptSetText(pdf, receiptMuted)
	pdf.SetFont("DM", "", 8)
	pdf.SetXY(14, 41)
	pdf.CellFormat(contentW, 5, "A handmade piece. A little more meaning. This receipt confirms your payment capture and order placement.", "", 0, "L", false, 0, "")

	// ==========================================
	// STATUS BANNER
	// ==========================================
	receiptSetFill(pdf, receiptForestTint)
	receiptSetDraw(pdf, "cce2d6")
	pdf.SetLineWidth(0.3)
	pdf.RoundedRect(14, 48, contentW, 8.5, 2, "1234", "DF")

	// Status pill
	receiptSetFill(pdf, "ffffff")
	receiptSetDraw(pdf, "b6d8c5")
	pdf.RoundedRect(17, 50, 36, 4.5, 1, "1234", "DF")
	receiptSetText(pdf, receiptForestMid)
	pdf.SetFont("DM", "B", 6.5)
	pdf.SetXY(17, 50)
	pdf.CellFormat(36, 4.5, "● PAYMENT CAPTURED", "", 0, "C", false, 0, "")

	// Second pill
	receiptSetFill(pdf, "ffffff")
	receiptSetDraw(pdf, "d8cebc")
	pdf.RoundedRect(55, 50, 32, 4.5, 1, "1234", "DF")
	receiptSetText(pdf, "6d5b40")
	pdf.SetFont("DM", "B", 6.5)
	pdf.SetXY(55, 50)
	pdf.CellFormat(32, 4.5, "● ORDER CONFIRMED", "", 0, "C", false, 0, "")

	receiptSetText(pdf, receiptInk)
	pdf.SetFont("DM", "", 7.5)
	pdf.SetXY(91, 50)
	payText := "Paid via " + firstNonEmpty(order.Payment, "Razorpay / UPI")
	pdf.CellFormat(55, 4.5, payText, "", 0, "L", false, 0, "")

	receiptSetText(pdf, receiptMuted)
	pdf.SetFont("DM", "", 7)
	pdf.SetXY(contentW-38, 50)
	pdf.CellFormat(50, 4.5, "Package 1 of 1 · Standard Express", "", 0, "R", false, 0, "")

	// ==========================================
	// 3 SUMMARY CARDS
	// ==========================================
	cardW := (contentW - 6) / 3
	cardH := 23.0
	cardsY := 59.0

	// Card 1: Shipping Address
	receiptSetFill(pdf, receiptCardBg)
	receiptSetDraw(pdf, receiptCardBdr)
	pdf.RoundedRect(14, cardsY, cardW, cardH, 2, "1234", "DF")
	receiptLabel(pdf, "Shipping Address", 17, cardsY+3, cardW-6)
	receiptSetText(pdf, receiptInk)
	pdf.SetFont("DM", "B", 8)
	pdf.SetXY(17, cardsY+7.5)
	recipient := firstNonEmpty(order.ContactName, "Customer Delivery")
	pdf.CellFormat(cardW-6, 4, recipient, "", 0, "L", false, 0, "")
	receiptSetText(pdf, receiptMuted)
	pdf.SetFont("DM", "", 7)
	pdf.SetXY(17, cardsY+12)
	pdf.MultiCell(cardW-6, 3.2, firstNonEmpty(order.ContactPhone, "Home Delivery Address\nIndia"), "", "L", false)

	// Card 2: Artisan Guild
	receiptSetFill(pdf, receiptCardBg)
	receiptSetDraw(pdf, receiptCardBdr)
	pdf.RoundedRect(14+cardW+3, cardsY, cardW, cardH, 2, "1234", "DF")
	receiptLabel(pdf, "Artisan Guild / Sold By", 17+cardW+3, cardsY+3, cardW-6)
	receiptSetText(pdf, receiptInk)
	pdf.SetFont("DM", "B", 8)
	pdf.SetXY(17+cardW+3, cardsY+7.5)
	seller := receiptSeller(order)
	pdf.CellFormat(cardW-6, 4, seller, "", 0, "L", false, 0, "")
	receiptSetText(pdf, receiptMuted)
	pdf.SetFont("DM", "", 7)
	pdf.SetXY(17+cardW+3, cardsY+12)
	pdf.MultiCell(cardW-6, 3.2, "Handcrafted Heritage Workshop\nVerified Maker Community", "", "L", false)

	// Card 3: Fulfilment
	receiptSetFill(pdf, receiptCardBg)
	receiptSetDraw(pdf, receiptCardBdr)
	pdf.RoundedRect(14+(cardW+3)*2, cardsY, cardW, cardH, 2, "1234", "DF")
	receiptLabel(pdf, "Delivery & Fulfilment", 17+(cardW+3)*2, cardsY+3, cardW-6)
	receiptSetText(pdf, receiptInk)
	pdf.SetFont("DM", "B", 8)
	pdf.SetXY(17+(cardW+3)*2, cardsY+7.5)
	pdf.CellFormat(cardW-6, 4, "Artisan Express Delivery", "", 0, "L", false, 0, "")
	receiptSetText(pdf, receiptMuted)
	pdf.SetFont("DM", "", 7)
	pdf.SetXY(17+(cardW+3)*2, cardsY+12)
	pdf.MultiCell(cardW-6, 3.2, "Insured Handcrafted Transit\nCarrier: Shiprocket / Logistics", "", "L", false)

	// ==========================================
	// ITEMS TABLE
	// ==========================================
	tableY := cardsY + cardH + 5
	receiptSetFill(pdf, receiptCardBg)
	receiptSetDraw(pdf, receiptCardBdr)
	pdf.RoundedRect(14, tableY, contentW, 7, 1.5, "1234", "DF")

	receiptLabel(pdf, "Ordered Item Description", 18, tableY+2, 85)
	pdfSetText(pdf, receiptForest)
	pdf.SetFont("DM", "B", 7)
	pdf.SetXY(110, tableY+2)
	pdf.CellFormat(16, 4, "QTY", "", 0, "C", false, 0, "")
	pdf.SetXY(128, tableY+2)
	pdf.CellFormat(30, 4, "UNIT PRICE", "", 0, "R", false, 0, "")
	pdf.SetXY(160, tableY+2)
	pdf.CellFormat(34, 4, "TOTAL AMOUNT", "", 0, "R", false, 0, "")

	rowY := tableY + 7
	for itemIndex, item := range order.Items {
		pdf.SetFont("DM", "B", 8.5)
		nameLines := pdf.SplitText(item.Product.Name, 74)
		detail := strings.TrimSpace(item.VariantName + " " + item.Product.SKU)
		if detail == "" {
			detail = firstNonEmpty(item.Product.Category, "Authentic Handcrafted Piece")
		}
		if item.Product.Seller != "" {
			detail += " · " + item.Product.Seller
		}
		pdf.SetFont("DM", "", 7)
		detailLines := pdf.SplitText(detail, 74)

		rowH := 8 + float64(len(nameLines))*4.2 + float64(len(detailLines))*3.5
		if rowH < 18 {
			rowH = 18
		}

		if rowY+rowH > 268 {
			pdf.AddPage()
			receiptLabel(pdf, "Order "+order.Number+" · continued", 14, 16, 182)
			rowY = 24
		}

		receiptSetDraw(pdf, receiptLine)
		pdf.SetLineWidth(0.2)
		pdf.Line(14, rowY+rowH, pageW-14, rowY+rowH)

		// Thumbnail box
		receiptSetFill(pdf, receiptCardBg)
		pdf.RoundedRect(17, rowY+2, 14, 14, 1, "1234", "F")
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
			pdf.ImageOptions(name, 17, rowY+2, 14, 14, false, options, 0, "")
		}

		// Text
		receiptSetText(pdf, receiptInk)
		pdf.SetFont("DM", "B", 8.5)
		pdf.SetXY(34, rowY+2.5)
		pdf.MultiCell(74, 4.2, item.Product.Name, "", "L", false)

		receiptSetText(pdf, receiptMuted)
		pdf.SetFont("DM", "", 7)
		pdf.SetXY(34, rowY+3+float64(len(nameLines))*4.2)
		pdf.MultiCell(74, 3.5, detail, "", "L", false)

		// Values
		receiptSetText(pdf, receiptInk)
		pdf.SetFont("DM", "", 8.5)
		pdf.SetXY(110, rowY+5)
		pdf.CellFormat(16, 5, fmt.Sprintf("%d", item.Quantity), "", 0, "C", false, 0, "")

		receiptValueRight(pdf, item.Product.Price, 128, rowY+5, 30)

		pdf.SetFont("DM", "B", 8.5)
		pdf.SetXY(160, rowY+5)
		pdf.CellFormat(34, 5, receiptMoney(item.LineTotalCents), "", 0, "R", false, 0, "")

		rowY += rowH
	}

	if len(order.Items) == 0 {
		rowY += 10
	}

	// ==========================================
	// TOTALS & TRUST / QR SECTION
	// ==========================================
	neededSpace := 56.0
	if order.CreditCents > 0 {
		neededSpace += 12
	}
	if rowY+neededSpace > 270 {
		pdf.AddPage()
		receiptLabel(pdf, "Order "+order.Number+" · totals", 14, 16, 182)
		rowY = 24
	}

	blockY := rowY + 5

	// Left: QR Code Card
	receiptSetFill(pdf, receiptCardBg)
	receiptSetDraw(pdf, receiptCardBdr)
	pdf.RoundedRect(14, blockY, 96, 26, 2, "1234", "DF")
	pdfDrawQR(pdf, 17, blockY+3, 20)

	receiptSetText(pdf, receiptInk)
	pdf.SetFont("DM", "B", 8)
	pdf.SetXY(40, blockY+4)
	pdf.CellFormat(68, 4, "Track Order & Support", "", 0, "L", false, 0, "")

	receiptSetText(pdf, receiptMuted)
	pdf.SetFont("DM", "", 7)
	pdf.SetXY(40, blockY+9)
	pdf.MultiCell(68, 3.4, "Scan for live courier dispatch updates.\nEmail: support@weevcrafts.com\nHelpline: +91 80 4910 8800", "", "L", false)

	// Left: Notice / Callout
	receiptSetFill(pdf, receiptCardBg)
	receiptSetDraw(pdf, receiptCardBdr)
	pdf.RoundedRect(14, blockY+29, 96, 20, 2, "1234", "DF")
	receiptSetDraw(pdf, receiptGold)
	pdf.SetLineWidth(0.8)
	pdf.Line(14, blockY+29, 14, blockY+49)

	receiptSetText(pdf, receiptForest)
	pdf.SetFont("DM", "B", 7)
	pdf.SetXY(17, blockY+31)
	pdf.CellFormat(90, 4, "A NOTE ON THIS DOCUMENT", "", 0, "L", false, 0, "")

	receiptSetText(pdf, receiptMuted)
	pdf.SetFont("DM", "", 6.8)
	pdf.SetXY(17, blockY+36)
	pdf.MultiCell(91, 3.2, "This receipt confirms customer payment capture. Your official GST Tax Invoice is issued directly by the registered seller upon parcel dispatch and can be downloaded from your account.", "", "L", false)

	// Right: Ledger Box
	ledgerX := 116.0
	ledgerW := contentW - 102.0
	ledgerY := blockY

	receiptSetFill(pdf, receiptCardBg)
	receiptSetDraw(pdf, receiptCardBdr)
	pdf.RoundedRect(ledgerX, ledgerY, ledgerW, 33, 2, "1234", "DF")

	trows := []struct{ label, value string }{
		{"Items Subtotal", order.Subtotal},
		{"Shipping & Delivery", order.Shipping},
		{"Artisan Promo / Discount", order.Discount},
	}
	if order.CreditCents > 0 {
		trows = append(trows, struct{ label, value string }{"Store credit used", order.CreditUsed}, struct{ label, value string }{"Net cash paid", order.CashPaid})
	}

	curY := ledgerY + 3
	for _, tr := range trows {
		receiptSetText(pdf, receiptMuted)
		pdf.SetFont("DM", "", 7.5)
		pdf.SetXY(ledgerX+4, curY)
		pdf.CellFormat(40, 4.5, tr.label, "", 0, "L", false, 0, "")

		receiptSetText(pdf, receiptInk)
		pdf.SetFont("DM", "", 7.5)
		pdf.SetXY(ledgerX+ledgerW-38, curY)
		pdf.CellFormat(34, 4.5, tr.value, "", 0, "R", false, 0, "")

		curY += 5.2
	}

	// Total Paid Container
	totBoxY := blockY + 36
	receiptSetFill(pdf, receiptForest)
	pdf.RoundedRect(ledgerX, totBoxY, ledgerW, 13, 2, "1234", "F")

	receiptSetText(pdf, "ffffff")
	pdf.SetFont("DM", "B", 8)
	pdf.SetXY(ledgerX+5, totBoxY+4)
	totalLabel := "TOTAL PAID"
	if !order.Confirmed() && order.PaymentState != "refunded" {
		totalLabel = "TOTAL PAYABLE"
	} else if order.PaymentState == "refunded" {
		totalLabel = "ORDER TOTAL"
	}
	pdf.CellFormat(35, 5, totalLabel, "", 0, "L", false, 0, "")

	pdf.SetFont("DM", "B", 12)
	pdf.SetXY(ledgerX+ledgerW-46, totBoxY+4)
	pdf.CellFormat(42, 5, order.Total, "", 0, "R", false, 0, "")

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
	return pdfMoney(cents)
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
