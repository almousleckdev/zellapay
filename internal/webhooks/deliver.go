package webhooks

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"
)

// DeliverResult counts what one delivery pass did.
type DeliverResult struct {
	Attempted int
	Succeeded int
	Retrying  int
	Dead      int
}

// DeliverDue sends the deliveries that are due, several at a time.
func (s *Service) DeliverDue(ctx context.Context, limit int) (DeliverResult, error) {
	work, err := s.d.Repo.ClaimDue(ctx, s.now(), limit, s.cfg.Lease)
	if err != nil || len(work) == 0 {
		return DeliverResult{}, err
	}
	var (
		mu  sync.Mutex
		res DeliverResult
		wg  sync.WaitGroup
		sem = make(chan struct{}, max(s.cfg.Concurrency, 1))
	)
	for _, w := range work {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			status := s.attempt(ctx, w)
			mu.Lock()
			defer mu.Unlock()
			res.add(status)
		}()
	}
	wg.Wait()
	return res, nil
}

func (r *DeliverResult) add(s DeliveryStatus) {
	r.Attempted++
	switch s {
	case DeliverySucceeded:
		r.Succeeded++
	case DeliveryDead:
		r.Dead++
	default:
		r.Retrying++
	}
}

// attempt sends one delivery and records the result. It returns the
// delivery's new status.
func (s *Service) attempt(ctx context.Context, w Work) DeliveryStatus {
	start, began := s.now(), time.Now()
	status, code := s.send(ctx, w, start)
	a := Attempt{
		Number: w.Delivery.Attempts + 1, At: start, StatusCode: status,
		Error: failureText(code, status), Duration: time.Since(began),
	}
	out := s.outcome(w, a, start)
	// Record even if shutdown was requested: an unrecorded attempt would be
	// repeated, and the merchant would receive the event twice.
	if err := s.d.Repo.Record(context.WithoutCancel(ctx), w.Delivery.ID, a, out); err != nil {
		s.d.Log.Error("recording a webhook attempt failed", "delivery", w.Delivery.ID, "err", err)
	}
	return out.Status
}

// send signs and posts the payload. A failure to read the signing secret is
// recorded as a failed attempt rather than lost.
func (s *Service) send(ctx context.Context, w Work, at time.Time) (int, string) {
	secret, err := s.d.Box.Open(w.SecretSealed, secretContext(w.Delivery.EndpointID))
	if err != nil {
		s.d.Log.Error("webhook secret unreadable", "endpoint", w.Delivery.EndpointID)
		return 0, "secret_unreadable"
	}
	status, err := s.d.Sender.Send(ctx, SendRequest{
		URL: w.URL, Body: w.Event.Payload, Headers: headers(w, string(secret), at),
	})
	var se *SendError
	switch {
	case errors.As(err, &se):
		return 0, se.Code
	case err != nil:
		return 0, CodeUnknown
	}
	return status, ""
}

func headers(w Work, secret string, at time.Time) map[string]string {
	return map[string]string{
		"Content-Type":              "application/json",
		"User-Agent":                "ZellaPay-Webhooks/1.0",
		SignatureHeader:             Sign(secret, at, w.Event.Payload),
		"ZellaPay-Event-Id":         w.Event.ID,
		"ZellaPay-Event-Type":       w.Event.Type,
		"ZellaPay-Delivery-Attempt": strconv.Itoa(w.Delivery.Attempts + 1),
	}
}

// outcome decides what happens after an attempt: done, retry later, or give
// up when the next retry would fall beyond the retry window.
func (s *Service) outcome(w Work, a Attempt, now time.Time) Outcome {
	if a.Error == "" && isHTTPSuccess(a.StatusCode) {
		return Outcome{Status: DeliverySucceeded, CompletedAt: &now}
	}
	next := now.Add(Backoff(a.Number))
	if next.After(w.Delivery.RetryUntil) {
		return Outcome{Status: DeliveryDead, CompletedAt: &now}
	}
	return Outcome{Status: DeliveryPending, NextAttemptAt: &next}
}

func isHTTPSuccess(status int) bool { return status >= 200 && status < 300 }

// failureText is the short, safe description stored with a failed attempt.
func failureText(code string, status int) string {
	if code != "" {
		return code
	}
	if status != 0 && !isHTTPSuccess(status) {
		return "http_" + strconv.Itoa(status)
	}
	return ""
}
