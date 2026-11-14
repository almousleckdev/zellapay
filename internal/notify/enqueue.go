package notify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zellapay/zellapay/internal/platform/ids"
)

const idPrefix = "ntf_"

// payload is what is sealed in the outbox.
type payload struct {
	To   string            `json:"to"`
	Data map[string]string `json:"data"`
}

func payloadContext(id string) []byte { return []byte("notification:" + id + ":payload") }

// Prepare validates m and returns it ready to store. Callers that must store
// a message in the same database transaction as another change use Prepare
// and the store's transactional insert; everyone else uses Enqueue.
func (s *Service) Prepare(m Message) (Prepared, error) {
	ch, ok := s.catalog.Channel(m.Template)
	if !ok {
		return Prepared{}, fmt.Errorf("%w: unknown template %q", ErrInvalidInput, m.Template)
	}
	if _, ok := s.d.Senders[ch]; !ok {
		return Prepared{}, fmt.Errorf("%w: %s", ErrChannelUnavailable, ch)
	}
	if m.Locale == "" {
		m.Locale = LocaleFrench
	}
	if err := validRecipient(ch, m.To); err != nil {
		return Prepared{}, err
	}
	if err := cleanData(m.Data); err != nil {
		return Prepared{}, err
	}
	// Render now so a message that could never be sent is refused up front.
	if _, _, err := s.catalog.Render(m.Template, m.Locale, m.Data); err != nil {
		return Prepared{}, err
	}
	id := ids.New(idPrefix)
	raw, err := json.Marshal(payload{To: m.To, Data: m.Data})
	if err != nil {
		return Prepared{}, err
	}
	sealed, err := s.d.Box.Seal(raw, payloadContext(id))
	if err != nil {
		return Prepared{}, fmt.Errorf("seal payload: %w", err)
	}
	now := s.now()
	return Prepared{
		ID: id, Channel: ch, Template: m.Template, Locale: m.Locale,
		RecipientMasked: maskRecipient(ch, m.To), PayloadSealed: sealed,
		DedupeKey: m.DedupeKey, CreatedAt: now, RetryUntil: now.Add(RetryHorizon),
	}, nil
}

// Enqueue stores a message for delivery. It returns the message ID and
// whether anything was stored: a repeat of a message with the same dedupe key
// is ignored.
func (s *Service) Enqueue(ctx context.Context, m Message) (string, bool, error) {
	p, err := s.Prepare(m)
	if err != nil {
		return "", false, err
	}
	stored, err := s.d.Repo.Insert(ctx, p)
	if err != nil {
		return "", false, err
	}
	return p.ID, stored, nil
}
