package notify

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// DeliverResult counts what one delivery pass did.
type DeliverResult struct {
	Attempted int
	Sent      int
	Retrying  int
	Dead      int
}

// DeliverDue sends the messages that are due, several at a time.
func (s *Service) DeliverDue(ctx context.Context, limit int) (DeliverResult, error) {
	queued, err := s.d.Repo.ClaimDue(ctx, s.now(), limit, s.cfg.Lease)
	if err != nil || len(queued) == 0 {
		return DeliverResult{}, err
	}
	var (
		mu  sync.Mutex
		res DeliverResult
		wg  sync.WaitGroup
		sem = make(chan struct{}, max(s.cfg.Concurrency, 1))
	)
	for _, q := range queued {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			status := s.attempt(ctx, q)
			mu.Lock()
			defer mu.Unlock()
			res.add(status)
		}()
	}
	wg.Wait()
	return res, nil
}

func (r *DeliverResult) add(s Status) {
	r.Attempted++
	switch s {
	case StatusSent:
		r.Sent++
	case StatusDead:
		r.Dead++
	default:
		r.Retrying++
	}
}

// attempt sends one message and records the result.
func (s *Service) attempt(ctx context.Context, q Queued) Status {
	now := s.now()
	code, permanent := s.deliver(ctx, q)
	res := s.outcome(q, code, permanent, now)
	// Record even if shutdown was requested: an unrecorded send would be
	// repeated and the person would get the message twice.
	if err := s.d.Repo.Record(context.WithoutCancel(ctx), q.ID, res); err != nil {
		s.d.Log.Error("recording a notification attempt failed", "id", q.ID, "err", err)
	}
	return res.Status
}

// deliver opens, renders and sends. It returns an error code, empty on
// success, and whether the failure is permanent.
func (s *Service) deliver(ctx context.Context, q Queued) (string, bool) {
	raw, err := s.d.Box.Open(q.PayloadSealed, payloadContext(q.ID))
	var p payload
	if err != nil || json.Unmarshal(raw, &p) != nil {
		return CodeUnreadable, true
	}
	subject, body, err := s.catalog.Render(q.Template, q.Locale, p.Data)
	if err != nil {
		return CodeRejected, true
	}
	sender, ok := s.d.Senders[q.Channel]
	if !ok {
		return CodeUnavailable, false
	}
	sctx, cancel := context.WithTimeout(ctx, s.cfg.SendTimeout)
	defer cancel()
	err = sender.Send(sctx, Delivery{ID: q.ID, To: p.To, Subject: subject, Body: body})
	var se *SendError
	switch {
	case err == nil:
		return "", false
	case errors.As(err, &se):
		return se.Code, se.Permanent
	}
	return CodeUnknown, false
}

// outcome decides what happens after an attempt: done, retry later, or give
// up when the failure is permanent or the retry window has passed.
func (s *Service) outcome(q Queued, code string, permanent bool, now time.Time) Result {
	attempts := q.Attempts + 1
	if code == "" {
		return Result{Status: StatusSent, Attempts: attempts, SentAt: &now}
	}
	next := now.Add(Backoff(attempts))
	if permanent || next.After(q.RetryUntil) {
		return Result{Status: StatusDead, Attempts: attempts, LastError: code}
	}
	return Result{Status: StatusPending, Attempts: attempts, NextAttemptAt: &next, LastError: code}
}
