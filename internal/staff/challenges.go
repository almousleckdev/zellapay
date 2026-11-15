package staff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/zellapay/zellapay/internal/identity"
)

// keep stores a ceremony's state so the browser's answer can be checked.
func (s *Service) keep(
	ctx context.Context, sessionID string, kind ChallengeKind, data *webauthn.SessionData,
) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.d.Repo.SaveChallenge(ctx, Challenge{
		SessionID: sessionID, Kind: kind, Data: raw,
		ExpiresAt: s.d.Clock.Now().Add(s.cfg.Challenge),
	})
}

// take returns the ceremony's state and deletes it: an answer is checked
// against a challenge once, and a replay finds nothing.
func (s *Service) take(
	ctx context.Context, sessionID string, kind ChallengeKind,
) (webauthn.SessionData, error) {
	raw, err := s.d.Repo.TakeChallenge(ctx, sessionID, kind, s.d.Clock.Now())
	if errors.Is(err, identity.ErrNotFound) {
		return webauthn.SessionData{}, identity.ErrInvalidCode
	}
	if err != nil {
		return webauthn.SessionData{}, err
	}
	var session webauthn.SessionData
	if err := json.Unmarshal(raw, &session); err != nil {
		return webauthn.SessionData{}, fmt.Errorf("read challenge: %w", err)
	}
	return session, nil
}
