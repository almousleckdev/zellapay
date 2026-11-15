package merchants

import (
	"context"
	"strings"
	"time"
)

// Paging limits for ListMerchants.
const (
	DefaultPageSize = 25
	MaxPageSize     = 100
	maxSearchLen    = 64
)

// Cursor is the position after the last merchant of a page.
type Cursor struct {
	CreatedAt time.Time
	ID        string
}

// ListQuery selects a page of merchants, newest first.
type ListQuery struct {
	Status Status // empty means every status
	Search string // part of the name, case-insensitive
	After  *Cursor
	Limit  int
}

// Page is one page of merchants. Next is nil on the last page.
type Page struct {
	Merchants []Merchant
	Next      *Cursor
}

// Detail is a merchant with the settings staff read when investigating.
type Detail struct {
	Merchant
	FeeBPS       int
	PayoutFeeBPS int
}

// List returns one page of merchants matching q.
func (s *Service) List(ctx context.Context, q ListQuery) (Page, error) {
	q, err := cleanQuery(q)
	if err != nil {
		return Page{}, err
	}
	// One extra row tells whether another page follows.
	rows, err := s.repo.ListMerchants(ctx, ListQuery{
		Status: q.Status, Search: q.Search, After: q.After, Limit: q.Limit + 1,
	})
	if err != nil {
		return Page{}, err
	}
	if len(rows) <= q.Limit {
		return Page{Merchants: rows}, nil
	}
	rows = rows[:q.Limit]
	last := rows[len(rows)-1]
	return Page{Merchants: rows, Next: &Cursor{CreatedAt: last.CreatedAt, ID: last.ID}}, nil
}

func cleanQuery(q ListQuery) (ListQuery, error) {
	if q.Status != "" && !q.Status.Valid() {
		return q, ErrInvalidInput
	}
	q.Search = strings.TrimSpace(q.Search)
	if len(q.Search) > maxSearchLen {
		return q, ErrInvalidInput
	}
	switch {
	case q.Limit == 0:
		q.Limit = DefaultPageSize
	case q.Limit < 0 || q.Limit > MaxPageSize:
		return q, ErrInvalidInput
	}
	return q, nil
}

// Detail returns a merchant with its fee settings, or ErrNotFound.
func (s *Service) Detail(ctx context.Context, id string) (Detail, error) {
	m, err := s.repo.GetMerchant(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	fee, err := s.repo.FeeBPS(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	payout, err := s.repo.PayoutFeeBPS(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	return Detail{Merchant: m, FeeBPS: fee, PayoutFeeBPS: payout}, nil
}
