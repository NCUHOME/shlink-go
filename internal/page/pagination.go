package page

type Pagination struct {
	CurrentPage        int `json:"currentPage"`
	PagesCount         int `json:"pagesCount"`
	ItemsPerPage       int `json:"itemsPerPage"`
	ItemsInCurrentPage int `json:"itemsInCurrentPage"`
	TotalItems         int `json:"totalItems"`
}

func New(current, perPage, total, inPage int) Pagination {
	pages := 0
	if total > 0 && perPage > 0 {
		pages = (total + perPage - 1) / perPage
	}
	return Pagination{
		CurrentPage:        current,
		PagesCount:         pages,
		ItemsPerPage:       perPage,
		ItemsInCurrentPage: inPage,
		TotalItems:         total,
	}
}
