package api

// Page sizes for every list.
const (
	DefaultPerPage = 20
	MaxPerPage     = 100
)

// NormalizePage clamps page (from 1) and perPage (1..MaxPerPage).
func NormalizePage(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}

	if perPage < 1 {
		perPage = DefaultPerPage
	}

	return page, min(perPage, MaxPerPage)
}

// PageMeta describes one page of a list.
type PageMeta struct {
	Page     int   `json:"page"`
	PerPage  int   `json:"per_page"`
	Total    int64 `json:"total"`
	LastPage int   `json:"last_page"`
}

// NewPageMeta builds the meta of a page.
func NewPageMeta(page, perPage int, total int64) PageMeta {
	page, perPage = NormalizePage(page, perPage)
	last := int((total + int64(perPage) - 1) / int64(perPage))

	return PageMeta{Page: page, PerPage: perPage, Total: total, LastPage: max(last, 1)}
}
