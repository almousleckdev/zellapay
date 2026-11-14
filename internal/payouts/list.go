package payouts

import (
	"context"
	"fmt"
)

const (
	defaultPageSize = 25
	maxPageSize     = 100
)

// ListQuery selects a page of a merchant's payouts, newest first.
type ListQuery struct {
	MerchantID string
	Limit      int      // 1 to 100; zero means 25
	After      string   // ID of the last item of the previous page
	Statuses   []Status // empty means every status
	Reference  string   // empty means any merchant reference
}

// Page is one page of payouts.
type Page struct {
	Items      []Payout
	HasMore    bool
	NextCursor string // pass as After to read the next page
}

// List returns one page of a merchant's payouts.
func (s *Service) List(ctx context.Context, q ListQuery) (Page, error) {
	if q.MerchantID == "" || q.Limit < 0 || q.Limit > maxPageSize {
		return Page{}, fmt.Errorf("%w: page size", ErrInvalidInput)
	}
	if q.Limit == 0 {
		q.Limit = defaultPageSize
	}
	if q.After != "" {
		p, err := s.d.Repo.Get(ctx, q.After)
		if err != nil || p.MerchantID != q.MerchantID {
			return Page{}, fmt.Errorf("%w: starting_after", ErrInvalidInput)
		}
	}
	asked := q
	asked.Limit = q.Limit + 1 // one extra tells us whether another page exists
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

// Get returns a payout by ID.
func (s *Service) Get(ctx context.Context, id string) (Payout, error) {
	return s.d.Repo.Get(ctx, id)
}
