package collections

import (
	"context"
	"fmt"
)

const (
	defaultPageSize = 25
	maxPageSize     = 100
)

// ListQuery selects a page of a merchant's collections, newest first.
type ListQuery struct {
	MerchantID string
	Limit      int      // 1 to 100; zero means 25
	After      string   // ID of the last item of the previous page
	Statuses   []Status // empty means every status
	Reference  string   // empty means any merchant reference
}

// Page is one page of collections.
type Page struct {
	Items      []Collection
	HasMore    bool
	NextCursor string // pass as After to read the next page
}

// List returns one page of a merchant's collections.
func (s *Service) List(ctx context.Context, q ListQuery) (Page, error) {
	if q.MerchantID == "" || q.Limit < 0 || q.Limit > maxPageSize {
		return Page{}, fmt.Errorf("%w: page size", ErrInvalidInput)
	}
	if q.Limit == 0 {
		q.Limit = defaultPageSize
	}
	if err := s.checkCursor(ctx, q); err != nil {
		return Page{}, err
	}
	// Ask for one extra item to learn whether another page exists.
	asked := q
	asked.Limit = q.Limit + 1
	items, err := s.d.Repo.List(ctx, asked)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: items}
	if len(items) > q.Limit {
		page.Items = items[:q.Limit]
		page.HasMore = true
		page.NextCursor = page.Items[q.Limit-1].ID
	}
	return page, nil
}

// checkCursor makes sure a cursor names one of the merchant's own items.
func (s *Service) checkCursor(ctx context.Context, q ListQuery) error {
	if q.After == "" {
		return nil
	}
	c, err := s.d.Repo.Get(ctx, q.After)
	if err != nil || c.MerchantID != q.MerchantID {
		return fmt.Errorf("%w: starting_after", ErrInvalidInput)
	}
	return nil
}
