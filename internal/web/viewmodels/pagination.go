package viewmodels

// PortalPagination describes a filtered PostgreSQL projection page.
type PortalPagination struct {
	Page, PageSize       int
	Total                int64
	Pages                int
	PreviousURL, NextURL string
}
