package merchants

import (
	"context"
	"errors"
	"time"

	"github.com/zellapay/zellapay/internal/audit"
)

// ChangeKind is what a change request asks for.
type ChangeKind string

// Kinds of change that a second staff member must approve.
const (
	ChangeActivate ChangeKind = "activate"
	ChangeFees     ChangeKind = "fees"
)

// RequestStatus is where a change request stands.
type RequestStatus string

// Request states. Only a pending request that has not expired can be decided.
const (
	RequestPending  RequestStatus = "pending"
	RequestApproved RequestStatus = "approved"
	RequestRejected RequestStatus = "rejected"
	RequestExpired  RequestStatus = "expired"
)

// requestTTL is how long a request waits for its second approver.
const requestTTL = 24 * time.Hour

// Errors for change requests.
var (
	// ErrSelfApproval means the person deciding is the one who asked.
	ErrSelfApproval = errors.New("merchants: a request cannot be decided by its requester")
	// ErrRequestClosed means the request was already decided or has expired.
	ErrRequestClosed = errors.New("merchants: request is no longer open")
	// ErrRequestOpen means the same kind of request is already waiting.
	ErrRequestOpen = errors.New("merchants: a request of this kind is already open")
)

// ChangeRequest is a change waiting for a second person's decision.
type ChangeRequest struct {
	ID             string
	MerchantID     string
	MerchantName   string // set when listing
	Kind           ChangeKind
	FeeBPS         *int // fees only
	PayoutFeeBPS   *int // fees only
	Reason         string
	RequestedBy    string
	RequestedAt    time.Time
	ExpiresAt      time.Time
	Status         RequestStatus
	DecidedBy      *string
	DecidedAt      *time.Time
	DecisionReason string
}

// Outcome is what deciding a request produces: the new state and the audit
// entries to write in the same transaction that applies the change.
type Outcome struct {
	Status         RequestStatus
	DecidedBy      string
	DecidedAt      time.Time
	DecisionReason string
	Effects        Effects
}

// DecideFunc looks at the locked request and returns its outcome, or an error
// that leaves the request untouched.
type DecideFunc func(r ChangeRequest) (Outcome, error)

// RequestFilter selects change requests, newest first.
type RequestFilter struct {
	Status     RequestStatus // empty means every status
	MerchantID string        // empty means every merchant
	Limit      int
}

// Origin is who asks for a change, why, and where the request came from.
type Origin struct {
	By     audit.Actor
	Reason string
	Source audit.Source
}

// effective reports the status a request has at now: an open request past its
// expiry reads as expired.
func (r ChangeRequest) effective(now time.Time) ChangeRequest {
	if r.Status == RequestPending && !r.ExpiresAt.After(now) {
		r.Status = RequestExpired
	}
	return r
}

// ListRequests returns change requests, newest first, with expiry applied.
func (s *Service) ListRequests(ctx context.Context, f RequestFilter) ([]ChangeRequest, error) {
	if f.Limit < 1 || f.Limit > MaxPageSize {
		f.Limit = DefaultPageSize
	}
	rows, err := s.repo.ListChangeRequests(ctx, f)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	for i := range rows {
		rows[i] = rows[i].effective(now)
	}
	return rows, nil
}

// PendingRequestCount counts requests that still wait for a decision.
func (s *Service) PendingRequestCount(ctx context.Context) (int, error) {
	return s.repo.CountPendingChangeRequests(ctx, s.clock.Now())
}
