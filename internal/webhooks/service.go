package webhooks

import (
	"crypto/rand"
	"io"
	"log/slog"
	"time"

	"github.com/zellapay/zellapay/internal/platform/clock"
)

// Deps are the collaborators a Service needs.
type Deps struct {
	Repo   Repository
	Sender Sender
	// Streams are the sources of events. Each has its own outbox and its own
	// way to describe the objects it announces.
	Streams []Stream
	Box     SecretBox
	Clock   clock.Clock
	Log     *slog.Logger
}

// Stream pairs an outbox with the source that describes its objects.
type Stream struct {
	Feed   Feed
	Source PayloadSource
}

// Config tunes the service.
type Config struct {
	Policy       Policy        // which destinations are allowed
	MaxEndpoints int           // per merchant
	Lease        time.Duration // how long a worker owns a delivery it is sending
	Concurrency  int           // deliveries sent in parallel
}

// DefaultConfig returns production-safe settings: no private destinations.
func DefaultConfig() Config {
	return Config{MaxEndpoints: 10, Lease: time.Minute, Concurrency: 8}
}

// Service implements webhook use cases.
type Service struct {
	d    Deps
	cfg  Config
	rand io.Reader
}

// NewService returns a Service.
func NewService(d Deps, cfg Config) *Service {
	return &Service{d: d, cfg: cfg, rand: rand.Reader}
}

func (s *Service) now() time.Time { return s.d.Clock.Now() }
