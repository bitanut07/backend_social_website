package responses

import "testing"

func TestPageRoundsUpTotalPages(t *testing.T) {
	t.Parallel()

	page := Page(21, 2, 10)

	if page.TotalPages != 3 {
		t.Fatalf("got %d total pages, want 3", page.TotalPages)
	}
	if page.Page != 2 || page.PageSize != 10 || page.TotalItems != 21 {
		t.Fatalf("unexpected pagination: %#v", page)
	}
}

func TestPageHandlesEmptyResult(t *testing.T) {
	t.Parallel()

	page := Page(0, 1, 10)
	if page.TotalPages != 0 {
		t.Fatalf("got %d total pages, want 0", page.TotalPages)
	}
}
