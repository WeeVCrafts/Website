// Package catalogimport validates merchant-supplied inventory without creating
// sample products or inferring price, stock, material or provenance claims.
package catalogimport

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/wecratfs/commerce/internal/domain/commerce"
)

var Headers = []string{"sku", "slug", "name", "description", "primary_category", "additional_categories", "price_inr", "stock", "image_file", "delivery_note"}
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var moneyPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]{1,2})?$`)

type Record struct {
	Input     commerce.ProductDraftInput
	ImageFile string
	Line      int
}

func Read(source io.Reader) ([]Record, error) {
	const maxCSVBytes = 8 << 20
	data, err := io.ReadAll(io.LimitReader(source, maxCSVBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read inventory CSV: %w", err)
	}
	if len(data) > maxCSVBytes {
		return nil, errors.New("inventory CSV must be no larger than 8 MiB; split it into smaller imports")
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.TrimLeadingSpace = true
	header, err := reader.Read()
	if err != nil {
		return nil, errors.New("inventory CSV needs a header row")
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	if len(header) != len(Headers) {
		return nil, fmt.Errorf("CSV must have columns: %s", strings.Join(Headers, ","))
	}
	for i, h := range header {
		if strings.TrimSpace(h) != Headers[i] {
			return nil, fmt.Errorf("column %d must be %s", i+1, Headers[i])
		}
	}
	records := []Record{}
	seen := map[string]bool{}
	slugs := map[string]bool{}
	for line := 2; ; line++ {
		cells, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", line, err)
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		sku := strings.ToUpper(cells[0])
		slug := cells[1]
		price, err := parseRupees(cells[6])
		if err != nil {
			return nil, fmt.Errorf("row %d: enter a positive INR price with at most two decimal places", line)
		}
		stock, err := strconv.Atoi(cells[7])
		if err != nil || stock < 0 || stock > 100000 {
			return nil, fmt.Errorf("row %d: stock must be a confirmed quantity from 0 to 100000", line)
		}
		if sku == "" || len(sku) > 120 || !validSlug(slug) || !validSlug(cells[4]) || utf8.RuneCountInString(cells[2]) < 1 || utf8.RuneCountInString(cells[2]) > 180 || utf8.RuneCountInString(cells[3]) < 1 || utf8.RuneCountInString(cells[3]) > 10000 || cells[8] == "" {
			return nil, fmt.Errorf("row %d: SKU, valid slugs, name, description and matching photo are required", line)
		}
		if seen[sku] || slugs[slug] {
			return nil, fmt.Errorf("row %d: duplicate SKU or product slug", line)
		}
		seen[sku] = true
		slugs[slug] = true
		categories := []string{cells[4]}
		if cells[5] != "" {
			for _, category := range strings.Split(cells[5], ";") {
				category = strings.TrimSpace(category)
				if !validSlug(category) {
					return nil, fmt.Errorf("row %d: invalid additional category", line)
				}
				categories = append(categories, category)
			}
		}
		if len(categories) > 100 || utf8.RuneCountInString(cells[9]) > 180 {
			return nil, fmt.Errorf("row %d: too many categories or delivery note too long", line)
		}
		note := cells[9]
		if note == "" {
			note = "Delivery details at checkout"
		}
		records = append(records, Record{Input: commerce.ProductDraftInput{SKU: sku, Slug: slug, Name: cells[2], Description: cells[3], CategorySlug: cells[4], CategorySlugs: categories, PriceCents: price, InitialStock: stock, DeliveryLabel: note, BrandSlug: "weevcrafts", BrandName: "WeevCrafts"}, ImageFile: cells[8], Line: line})
		if len(records) > 10000 {
			return nil, errors.New("import at most 10000 products per CSV")
		}
	}
	if len(records) == 0 {
		return nil, errors.New("the CSV contains no inventory; fill the template with your real products first")
	}
	return records, nil
}

func validSlug(s string) bool { return len(s) >= 2 && len(s) <= 120 && slugPattern.MatchString(s) }
func parseRupees(raw string) (int64, error) {
	if !moneyPattern.MatchString(raw) {
		return 0, errors.New("invalid price")
	}
	parts := strings.SplitN(raw, ".", 2)
	rupees, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || rupees > 999999999 {
		return 0, errors.New("price out of range")
	}
	var paise int64
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) == 1 {
			fraction += "0"
		}
		paise, _ = strconv.ParseInt(fraction, 10, 64)
	}
	value := rupees*100 + paise
	if value <= 0 {
		return 0, errors.New("invalid price")
	}
	return value, nil
}

func PrimaryCounts(records []Record) map[string]int {
	counts := map[string]int{}
	for _, record := range records {
		counts[record.Input.CategorySlug]++
	}
	return counts
}

var LaunchCategories = []string{"arts", "home", "textiles", "gifts", "pottery-ceramics", "handloom-sarees", "brass-metalwork"}

func ValidateLaunchCounts(records []Record) error {
	counts := PrimaryCounts(records)
	for _, category := range LaunchCategories {
		if counts[category] < 10 {
			return fmt.Errorf("%s needs at least 10 primary products; CSV contains %d", category, counts[category])
		}
	}
	return nil
}
