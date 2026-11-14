package payouts

import (
	"log/slog"
	"time"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/platform/clock"
)

const idPrefix = "pay_"

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

// Config tunes limits and when open payouts are re-checked at the operator.
type Config struct {
	MaxAmount           int64         // largest single payout, in the smallest unit
	PendingCheckAfter   time.Duration // between checks of a pending payout
	AmbiguousCheckAfter time.Duration // between checks of an unclear outcome
	CreatedGrace        time.Duration // how long a stored or held payout may await the next step
	StaleAfter          time.Duration // age at which a pending one needs attention
	StaleCheckAfter     time.Duration // between checks once stale
}

// DefaultConfig returns conservative settings.
func DefaultConfig() Config {
	return Config{
		MaxAmount:           5_000_000,
		PendingCheckAfter:   30 * time.Second,
		AmbiguousCheckAfter: 10 * time.Second,
		CreatedGrace:        2 * time.Minute,
		StaleAfter:          24 * time.Hour,
		StaleCheckAfter:     time.Hour,
	}
}

// Service implements the payouts use cases.
type Service struct {
	d   Deps
	cfg Config
}

// NewService returns a Service.
func NewService(d Deps, cfg Config) *Service { return &Service{d: d, cfg: cfg} }

func (s *Service) now() time.Time { return s.d.Clock.Now() }
