package catalogimport

import (
	"encoding/csv"
	"strings"
	"testing"
)

func inventoryCSV(rows ...[]string) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write(Headers)
	_ = w.WriteAll(rows)
	return b.String()
}
func inventoryRow() []string {
	return []string{"WC-001", "ceramic-cup", "Ceramic cup", "Merchant supplied description", "pottery-ceramics", "gifts;home", "599.50", "12", "cup.jpg", ""}
}

func TestInventoryRequiresActualPricesStockAndUniqueIdentities(t *testing.T) {
	records, err := Read(strings.NewReader(inventoryCSV(inventoryRow())))
	if err != nil || len(records) != 1 || records[0].Input.PriceCents != 59950 || records[0].Input.InitialStock != 12 || len(records[0].Input.CategorySlugs) != 3 {
		t.Fatalf("parse: %+v %v", records, err)
	}
	for _, example := range []struct {
		field int
		value string
	}{{6, ""}, {6, "599.999"}, {6, "1e3"}, {6, "0"}, {7, ""}, {7, "-1"}, {8, ""}, {4, "Bad Category"}, {5, "gifts;;home"}} {
		row := inventoryRow()
		row[example.field] = example.value
		if _, err := Read(strings.NewReader(inventoryCSV(row))); err == nil {
			t.Fatalf("accepted invalid field %d=%q", example.field, example.value)
		}
	}
	if _, err := Read(strings.NewReader(inventoryCSV(inventoryRow(), inventoryRow()))); err == nil {
		t.Fatal("duplicate inventory accepted")
	}
	if _, err := Read(strings.NewReader(inventoryCSV())); err == nil {
		t.Fatal("empty template treated as inventory")
	}
	if err := ValidateLaunchCounts(records); err == nil {
		t.Fatal("cross listing must not inflate primary counts")
	}
}

func TestInventoryRejectsOversizedManifestInsteadOfImportingAPrefix(t *testing.T) {
	manifest := inventoryCSV(inventoryRow())
	manifest += strings.Repeat("\n", (8<<20)-len(manifest))
	manifest += inventoryCSV(inventoryRow())
	if _, err := Read(strings.NewReader(manifest)); err == nil || !strings.Contains(err.Error(), "8 MiB") {
		t.Fatalf("oversized manifest was not rejected: %v", err)
	}
}
