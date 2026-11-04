package viewmodels

// AdminInsights contains read-only aggregates, independent of queue pagination.
type AdminInsights struct {
	Available    bool
	Days         int
	Trend        []AdminChartPoint
	RefundTrend  []AdminChartPoint
	Distribution []AdminChartSlice
	Total        string
	RecentStock  []AdminStockChange
}

type AdminChartPoint struct {
	Label string
	Value int64
}

type AdminChartSlice struct {
	Label, Color string
	Value        int64
}

type AdminStockChange struct {
	Product, SKU, Reason, Date string
	Delta                      int
}
