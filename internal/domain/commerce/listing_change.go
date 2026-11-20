package commerce

type ListingChange struct {
	ProductID, Revision          int64
	Product, Seller, State, Note string
	CurrentCover, ProposedCover  string
	Prices                       []ListingPriceChange
	Media                        []ListingMediaChange
}

type ListingPriceChange struct {
	Option            string
	Current, Proposed int64
	CurrentCompareAt  *int64
	ProposedCompareAt *int64
	NewOption         bool
}

type ListingMediaChange struct {
	Path, ContentType string
	Publish           bool
}
