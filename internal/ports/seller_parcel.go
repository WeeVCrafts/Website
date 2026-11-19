package ports

type SellerParcelProfile struct {
	ID   int64
	Name string
	Parcel
}

type SellerParcelOption struct {
	ID                       int64
	Product, SKU, Source     string
	ProfileID                int64
	FollowsDefault, Measured bool
}

type SellerParcelSetup struct {
	Profiles        []SellerParcelProfile
	Options         []SellerParcelOption
	DefaultID, Next int64
}

type SellerParcelChange struct {
	Action        string
	ProfileID     int64
	OptionIDs     []int64
	FollowDefault bool
}

type SellerParcelPreview struct {
	Change  SellerParcelChange
	Options []SellerParcelOption
	Profile SellerParcelProfile
	Token   string
}
