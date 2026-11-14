package collections

import (
	"log/slog"
	"time"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/platform/clock"
)

// Deps are the collaborators a Service needs.
type Deps struct {
	Repo     Repository
	Terms    MerchantTerms
	Router   OperatorRouter
	Ledger   ledger.Poster
	Accounts AccountEnsurer
	Clock    clock.Clock
	Log      *slog.Logger
}

// Config tunes when open collections are re-checked at the operator.
type Config struct {
	PendingCheckAfter   time.Duration // between checks of a pending collection
	AmbiguousCheckAfter time.Duration // between checks of an unclear outcome
	CreatedGrace        time.Duration // how long a stored request may await the operator call
	StaleAfter          time.Duration // age at which a pending one needs attention
	StaleCheckAfter     time.Duration // between checks once stale
}

// DefaultConfig returns conservative intervals.
func DefaultConfig() Config {
	return Config{
		PendingCheckAfter:   30 * time.Second,
		AmbiguousCheckAfter: 10 * time.Second,
		CreatedGrace:        2 * time.Minute,
		StaleAfter:          24 * time.Hour,
		StaleCheckAfter:     time.Hour,
	}
}

// Service implements the collections use cases.
type Service struct {
	d   Deps
	cfg Config
}

// NewService returns a Service.
func NewService(d Deps, cfg Config) *Service {
	return &Service{d: d, cfg: cfg}
}

func (s *Service) now() time.Time { return s.d.Clock.Now() }
