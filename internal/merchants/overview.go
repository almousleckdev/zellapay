package merchants

import (
	"context"
	"time"
)

// DayCount is how many merchants were created on one local day.
type DayCount struct {
	Day   time.Time // midnight of the day, in Gabon's time zone
	Count int
}

// Overview summarises the merchant base for the admin console.
type Overview struct {
	Total     int
	ByStatus  map[Status]int
	CreatedBy []DayCount // one entry per day, oldest first, zeros included
}

// gabon is West Africa Time. Gabon has no daylight saving time.
var gabon = time.FixedZone("WAT", 3600)

const maxOverviewDays = 90

// Overview counts merchants by status and by day of creation over days days.
func (s *Service) Overview(ctx context.Context, days int) (Overview, error) {
	if days < 1 || days > maxOverviewDays {
		return Overview{}, ErrInvalidInput
	}
	today := s.clock.Now().In(gabon)
	first := time.Date(today.Year(), today.Month(), today.Day()-days+1, 0, 0, 0, 0, gabon)
	counts, err := s.repo.MerchantCounts(ctx)
	if err != nil {
		return Overview{}, err
	}
	rows, err := s.repo.CreatedPerDay(ctx, first)
	if err != nil {
		return Overview{}, err
	}
	o := Overview{ByStatus: map[Status]int{}, CreatedBy: fillDays(first, days, rows)}
	for _, st := range []Status{StatusPending, StatusActive, StatusSuspended} {
		o.ByStatus[st] = counts[st]
		o.Total += counts[st]
	}
	return o, nil
}

// fillDays returns one entry per day from first, with zero for quiet days.
func fillDays(first time.Time, days int, rows []DayCount) []DayCount {
	have := map[string]int{}
	for _, r := range rows {
		have[r.Day.Format(time.DateOnly)] = r.Count
	}
	out := make([]DayCount, days)
	for i := range out {
		d := first.AddDate(0, 0, i)
		out[i] = DayCount{Day: d, Count: have[d.Format(time.DateOnly)]}
	}
	return out
}
