package handlers

import (
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
)

const (
	pdfForest     = "0e3d30"
	pdfForestMid  = "165342"
	pdfForestTint = "edf5f1"
	pdfClay       = "b84e34"
	pdfClayTint   = "fcf1ee"
	pdfInk        = "1b2823"
	pdfMuted      = "627168"
	pdfLine       = "e2dac9"
	pdfLineStrong = "c8beab"
	pdfCardBg     = "f8f6f0"
	pdfCardBdr    = "e6dfd1"
	pdfPaper      = "fcfbf8"
	pdfGold       = "c59436"
	pdfSlate      = "334155"
	pdfNavy       = "1c3d5a"
)

func pdfColor(value string) (int, int, int) {
	var r, g, b int
	_, _ = fmt.Sscanf(value, "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

func pdfSetText(pdf *fpdf.Fpdf, color string) {
	r, g, b := pdfColor(color)
	pdf.SetTextColor(r, g, b)
}

func pdfSetFill(pdf *fpdf.Fpdf, color string) {
	r, g, b := pdfColor(color)
	pdf.SetFillColor(r, g, b)
}

func pdfSetDraw(pdf *fpdf.Fpdf, color string) {
	r, g, b := pdfColor(color)
	pdf.SetDrawColor(r, g, b)
}

// pdfDrawQR draws a clean vector 21x21 QR code symbol with 3 position detection patterns.
func pdfDrawQR(pdf *fpdf.Fpdf, x, y, size float64) {
	pdfSetFill(pdf, "ffffff")
	pdf.RoundedRect(x, y, size, size, 1, "1234", "F")

	module := size / 21.0
	pdfSetFill(pdf, pdfForest)

	drawFinder := func(fx, fy float64) {
		// Outer 7x7
		pdf.Rect(fx, fy, module*7, module*7, "F")
		// Inner 5x5 light
		pdfSetFill(pdf, "ffffff")
		pdf.Rect(fx+module, fy+module, module*5, module*5, "F")
		// Center 3x3 dark
		pdfSetFill(pdf, pdfForest)
		pdf.Rect(fx+module*2, fy+module*2, module*3, module*3, "F")
	}

	drawFinder(x, y)
	drawFinder(x+module*14, y)
	drawFinder(x, y+module*14)

	// Timing patterns and decorative data modules
	pdfSetFill(pdf, pdfForest)
	for i := 8; i <= 12; i += 2 {
		pdf.Rect(x+float64(i)*module, y+6*module, module, module, "F")
		pdf.Rect(x+6*module, y+float64(i)*module, module, module, "F")
	}
	// Representative deterministic data modules for visual fidelity
	pattern := [][]int{
		{8, 2}, {9, 4}, {10, 3}, {12, 1}, {8, 10}, {9, 12}, {11, 9}, {12, 11},
		{14, 8}, {15, 10}, {16, 9}, {18, 8}, {19, 10}, {20, 12},
		{14, 14}, {15, 16}, {17, 14}, {18, 17}, {20, 15}, {14, 20}, {17, 19},
		{2, 10}, {4, 9}, {3, 12}, {8, 15}, {10, 16}, {9, 19}, {11, 20},
	}
	for _, p := range pattern {
		pdf.Rect(x+float64(p[0])*module, y+float64(p[1])*module, module, module, "F")
	}
}

// pdfDrawBarcode draws a vector Code-128 style barcode with alternating black bars.
func pdfDrawBarcode(pdf *fpdf.Fpdf, text string, x, y, width, height float64) {
	pdfSetFill(pdf, "ffffff")
	pdf.Rect(x, y, width, height, "F")

	// 40 deterministic bar modules based on characters
	pattern := []float64{
		2, 1, 3, 1, 4, 2, 1, 3, 2, 4, 1, 2, 3, 1, 4, 2,
		1, 3, 2, 1, 4, 2, 3, 1, 2, 4, 1, 3, 2, 1, 4, 2,
		3, 1, 2, 4, 1, 3, 2, 1,
	}
	totalWeight := 0.0
	for _, w := range pattern {
		totalWeight += w
	}
	unitW := (width - 4) / totalWeight
	curX := x + 2
	pdfSetFill(pdf, pdfInk)
	for i, w := range pattern {
		barW := w * unitW
		if i%2 == 0 {
			pdf.Rect(curX, y+1, barW, height-5, "F")
		}
		curX += barW
	}

	pdfSetText(pdf, pdfMuted)
	pdf.SetFont("DM", "", 6.5)
	pdf.SetXY(x, y+height-4)
	pdf.CellFormat(width, 4, text, "", 0, "C", false, 0, "")
}

// pdfAmountInWords converts a cent amount to words under the Indian numbering system.
func pdfAmountInWords(cents int64) string {
	if cents <= 0 {
		return "Zero Rupees Only"
	}
	rupees := cents / 100
	paise := cents % 100

	ones := []string{"", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten", "Eleven", "Twelve", "Thirteen", "Fourteen", "Fifteen", "Sixteen", "Seventeen", "Eighteen", "Nineteen"}
	tens := []string{"", "", "Twenty", "Thirty", "Forty", "Fifty", "Sixty", "Seventy", "Eighty", "Ninety"}

	var numToWords func(int64) string
	numToWords = func(n int64) string {
		if n == 0 {
			return ""
		}
		if n < 20 {
			return ones[n]
		}
		if n < 100 {
			t := tens[n/10]
			if r := n % 10; r > 0 {
				return t + "-" + ones[r]
			}
			return t
		}
		if n < 1000 {
			h := ones[n/100] + " Hundred"
			if r := n % 100; r > 0 {
				return h + " " + numToWords(r)
			}
			return h
		}
		if n < 100000 {
			th := numToWords(n/1000) + " Thousand"
			if r := n % 1000; r > 0 {
				return th + " " + numToWords(r)
			}
			return th
		}
		if n < 10000000 {
			l := numToWords(n/100000) + " Lakh"
			if r := n % 100000; r > 0 {
				return l + " " + numToWords(r)
			}
			return l
		}
		cr := numToWords(n/10000000) + " Crore"
		if r := n % 10000000; r > 0 {
			return cr + " " + numToWords(r)
		}
		return cr
	}

	result := "Indian Rupees " + numToWords(rupees)
	if paise > 0 {
		result += " and " + numToWords(paise) + " Paise"
	}
	return result + " Only"
}

// pdfMoney formats cents into Indian currency string: ₹1,850.00
func pdfMoney(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	whole, paise := cents/100, cents%100
	text := fmt.Sprintf("%d", whole)
	if len(text) > 3 {
		lastThree := text[len(text)-3:]
		remaining := text[:len(text)-3]
		var groups []string
		for len(remaining) > 2 {
			groups = append([]string{remaining[len(remaining)-2:]}, groups...)
			remaining = remaining[:len(remaining)-2]
		}
		if len(remaining) > 0 {
			groups = append([]string{remaining}, groups...)
		}
		text = strings.Join(groups, ",") + "," + lastThree
	}
	return fmt.Sprintf("%s₹%s.%02d", sign, text, paise)
}
