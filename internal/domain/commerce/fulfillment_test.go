package commerce

import "testing"

func TestAggregateFulfillmentStage(t *testing.T) {
	tests := []struct {
		name string
		got  FulfillmentStage
		want FulfillmentStage
		rows []OrderFulfillment
	}{
		{name: "empty", want: FulfillmentPending},
		{name: "processing", rows: []OrderFulfillment{{Status: "processing"}}, want: FulfillmentProcessing},
		{name: "packing takes precedence over processing", rows: []OrderFulfillment{{Status: "processing"}, {Status: "packing"}}, want: FulfillmentPacking},
		{name: "mixed sellers stay shipped until all are ready", rows: []OrderFulfillment{{Status: "shipped"}, {Status: "out_for_delivery"}}, want: FulfillmentShipped},
		{name: "all ready", rows: []OrderFulfillment{{Status: "out_for_delivery"}, {Status: "delivered"}}, want: FulfillmentOutForDelivery},
		{name: "all delivered", rows: []OrderFulfillment{{Status: "delivered"}, {Status: "delivered"}}, want: FulfillmentDelivered},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got = AggregateFulfillmentStage(test.rows); test.got != test.want {
				t.Fatalf("AggregateFulfillmentStage() = %q, want %q", test.got, test.want)
			}
		})
	}
}

func TestFulfillmentStageOnlyAdvancesOneStep(t *testing.T) {
	if FulfillmentProcessing.CanAdvanceTo(FulfillmentShipped) {
		t.Fatal("processing must not skip packing")
	}
	if !FulfillmentPacking.CanAdvanceTo(FulfillmentShipped) {
		t.Fatal("packing should advance to shipped")
	}
	if FulfillmentDelivered.Next() != "" {
		t.Fatal("delivered must be terminal")
	}
}

func TestMixedParcelProgress(t *testing.T) {
	for _, test := range []struct {
		stages []string
		want   string
	}{
		{[]string{"shipped", "pending"}, "partially_shipped"},
		{[]string{"delivered", "packing"}, "partially_delivered"},
		{[]string{"delivered", "out_for_delivery"}, "partially_delivered"},
		{[]string{"shipped", "out_for_delivery"}, "shipped"},
		{[]string{"delivered", "delivered"}, "delivered"},
	} {
		parcels := make([]OrderFulfillment, len(test.stages))
		for i, stage := range test.stages {
			parcels[i].Status = stage
		}
		if got := FulfillmentProgress(parcels); got != test.want {
			t.Fatalf("%v = %s want %s", test.stages, got, test.want)
		}
		if len(parcels) == 2 && parcels[1].Status == "pending" && AggregateFulfillmentStage(parcels) != FulfillmentProcessing {
			t.Fatal("partly shipped must remain processing")
		}
	}
}
