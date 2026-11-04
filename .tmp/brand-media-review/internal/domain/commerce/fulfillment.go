package commerce

// FulfillmentStage is the seller-scoped delivery state. It deliberately sits
// beside Order.Status, which continues to describe the financial lifecycle.
type FulfillmentStage string

const (
	FulfillmentPending        FulfillmentStage = "pending"
	FulfillmentProcessing     FulfillmentStage = "processing"
	FulfillmentPacking        FulfillmentStage = "packing"
	FulfillmentShipped        FulfillmentStage = "shipped"
	FulfillmentOutForDelivery FulfillmentStage = "out_for_delivery"
	FulfillmentDelivered      FulfillmentStage = "delivered"
)

var FulfillmentStages = []FulfillmentStage{
	FulfillmentPending,
	FulfillmentProcessing,
	FulfillmentPacking,
	FulfillmentShipped,
	FulfillmentOutForDelivery,
	FulfillmentDelivered,
}

func (s FulfillmentStage) Label() string {
	switch s {
	case FulfillmentPending:
		return "New"
	case FulfillmentProcessing:
		return "Processing"
	case FulfillmentPacking:
		return "Packing"
	case FulfillmentShipped:
		return "Shipped"
	case FulfillmentOutForDelivery:
		return "Ready to deliver"
	case FulfillmentDelivered:
		return "Delivered"
	default:
		return string(s)
	}
}

func (s FulfillmentStage) Index() int {
	for i, stage := range FulfillmentStages {
		if s == stage {
			return i
		}
	}
	return -1
}

func (s FulfillmentStage) Next() FulfillmentStage {
	i := s.Index()
	if i < 0 || i+1 >= len(FulfillmentStages) {
		return ""
	}
	return FulfillmentStages[i+1]
}

func (s FulfillmentStage) CanAdvanceTo(next FulfillmentStage) bool {
	return next == s || next == s.Next()
}

func ParseFulfillmentStage(value string) (FulfillmentStage, bool) {
	stage := FulfillmentStage(value)
	for _, allowed := range FulfillmentStages {
		if stage == allowed {
			return stage, true
		}
	}
	return "", false
}

// AggregateFulfillmentStage projects seller parcel stages onto the order. A
// parcel that has not been started keeps the order in its current financial
// state, while the finer seller stage remains visible in shipment details.
func AggregateFulfillmentStage(fulfillments []OrderFulfillment) FulfillmentStage {
	if len(fulfillments) == 0 {
		return FulfillmentPending
	}
	allAtLeast := func(min FulfillmentStage) bool {
		for _, fulfillment := range fulfillments {
			stage, ok := ParseFulfillmentStage(fulfillment.Status)
			if !ok || stage.Index() < min.Index() {
				return false
			}
		}
		return true
	}
	if allAtLeast(FulfillmentDelivered) {
		return FulfillmentDelivered
	}
	if allAtLeast(FulfillmentOutForDelivery) {
		return FulfillmentOutForDelivery
	}
	if allAtLeast(FulfillmentShipped) {
		return FulfillmentShipped
	}
	for _, fulfillment := range fulfillments {
		stage, ok := ParseFulfillmentStage(fulfillment.Status)
		if ok && stage == FulfillmentPacking {
			return FulfillmentPacking
		}
	}
	for _, fulfillment := range fulfillments {
		stage, ok := ParseFulfillmentStage(fulfillment.Status)
		if ok && stage.Index() >= FulfillmentProcessing.Index() {
			return FulfillmentProcessing
		}
	}
	return FulfillmentPending
}

// FulfillmentProgress preserves the six editable parcel stages and describes
// mixed shipment/delivery progress without promoting the financial lifecycle.
func FulfillmentProgress(parcels []OrderFulfillment) string {
	stage := AggregateFulfillmentStage(parcels)
	shipped, delivered := 0, 0
	for _, parcel := range parcels {
		value, ok := ParseFulfillmentStage(parcel.Status)
		if ok && value.Index() >= FulfillmentShipped.Index() {
			shipped++
		}
		if value == FulfillmentDelivered {
			delivered++
		}
	}
	if delivered > 0 && delivered < len(parcels) {
		return "partially_delivered"
	}
	if shipped > 0 && shipped < len(parcels) {
		return "partially_shipped"
	}
	return string(stage)
}
