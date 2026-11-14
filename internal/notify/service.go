package notify

import (
	"log/slog"
	"time"

	"github.com/zellapay/zellapay/internal/platform/clock"
)

// Deps are the collaborators a Service needs.
type Deps struct {
	Repo    Repository
	Senders map[Channel]Sender // a channel with no sender cannot be used
	Box     SecretBox
	Clock   clock.Clock
	Log     *slog.Logger
}

// Config tunes delivery.
type Config struct {
	Lease       time.Duration // how long a worker owns a message it is sending
	SendTimeout time.Duration
	Concurrency int
}

// DefaultConfig returns sensible settings.
func DefaultConfig() Config {
	return Config{Lease: time.Minute, SendTimeout: 20 * time.Second, Concurrency: 4}
}

// Service queues and delivers messages.
type Service struct {
	d       Deps
	cfg     Config
	catalog *Catalog
}

// NewService returns a Service, failing if a template is missing or broken.
func NewService(d Deps, cfg Config) (*Service, error) {
	c, err := NewCatalog()
	if err != nil {
		return nil, err
	}
	return &Service{d: d, cfg: cfg, catalog: c}, nil
}

func (s *Service) now() time.Time { return s.d.Clock.Now() }
