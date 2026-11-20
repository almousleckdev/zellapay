package merchants

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"strings"

	"github.com/zellapay/zellapay/internal/platform/clock"
	"github.com/zellapay/zellapay/internal/platform/ids"
)

const maxNameLen = 100

// Service implements merchant and API key use cases.
type Service struct {
	repo  Repository
	clock clock.Clock
	rand  io.Reader
	mode  Mode // the environment this process serves
	// auditor is optional: only changes that name a person are audited.
	auditor Auditor
}

// NewService returns a Service. mode is the environment the process serves:
// a sandbox deployment accepts only test keys, production only live keys.
func NewService(repo Repository, c clock.Clock, mode Mode) *Service {
	return &Service{repo: repo, clock: c, rand: rand.Reader, mode: mode}
}

// CreateMerchant registers a merchant in the pending state.
func (s *Service) CreateMerchant(ctx context.Context, name, currency string) (Merchant, error) {
	m, err := s.newMerchant(name, currency)
	if err != nil {
		return Merchant{}, err
	}
	return s.repo.CreateMerchant(ctx, m, Effects{})
}

func (s *Service) newMerchant(name, currency string) (Merchant, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxNameLen || len(currency) != 3 ||
		currency != strings.ToUpper(currency) {
		return Merchant{}, ErrInvalidInput
	}
	return Merchant{
		ID:              ids.New("mer_"),
		Name:            name,
		Status:          StatusPending,
		DefaultCurrency: currency,
		CreatedAt:       s.clock.Now(),
	}, nil
}

// GetMerchant returns a merchant by ID, or ErrNotFound.
func (s *Service) GetMerchant(ctx context.Context, id string) (Merchant, error) {
	return s.repo.GetMerchant(ctx, id)
}

// SetStatus activates or suspends a merchant.
func (s *Service) SetStatus(ctx context.Context, id string, st Status) error {
	switch st {
	case StatusPending, StatusActive, StatusSuspended:
		return s.repo.SetStatus(ctx, id, st, s.clock.Now())
	}
	return ErrInvalidInput
}

// IssueKey creates an API key and returns its secret exactly once.
func (s *Service) IssueKey(ctx context.Context, spec KeySpec) (IssuedKey, error) {
	if err := validateSpec(spec); err != nil {
		return IssuedKey{}, err
	}
	parts, err := newKeyParts(s.rand, spec.Mode)
	if err != nil {
		return IssuedKey{}, err
	}
	key := s.buildKey(spec, parts)
	fx, err := s.keyEffects("api_key.created", key, spec.By, nil)
	if err != nil {
		return IssuedKey{}, err
	}
	if err := s.repo.InsertKey(ctx, key, fx); err != nil {
		return IssuedKey{}, fmt.Errorf("store key: %w", err)
	}
	return IssuedKey{Key: key, Secret: parts.raw()}, nil
}

func (s *Service) buildKey(spec KeySpec, p keyParts) APIKey {
	return APIKey{
		ID:          ids.New("key_"),
		MerchantID:  spec.MerchantID,
		LookupID:    p.lookup,
		SecretHash:  p.hash(),
		Mode:        spec.Mode,
		Scopes:      spec.Scopes,
		IPAllowlist: spec.IPAllowlist,
		CreatedAt:   s.clock.Now(),
		ExpiresAt:   spec.ExpiresAt,
		Label:       spec.Label,
	}
}

func validateSpec(spec KeySpec) error {
	if spec.MerchantID == "" || !spec.Mode.Valid() || len(spec.Scopes) == 0 ||
		len(spec.Label) > 64 {
		return ErrInvalidInput
	}
	for _, sc := range spec.Scopes {
		if !sc.Valid() {
			return ErrInvalidInput
		}
	}
	return nil
}

// RevokeKey permanently disables a key.
func (s *Service) RevokeKey(ctx context.Context, merchantID, keyID string) error {
	return s.RevokeKeyRef(ctx, KeyRef{MerchantID: merchantID, KeyID: keyID})
}

// RevokeKeyRef disables a key. When ref names who is acting, it is audited.
func (s *Service) RevokeKeyRef(ctx context.Context, ref KeyRef) error {
	var fx Effects
	if ref.By != nil {
		key, err := s.findKey(ctx, ref)
		if err != nil {
			return err
		}
		if fx, err = s.keyEffects("api_key.revoked", key, ref.By, nil); err != nil {
			return err
		}
	}
	return s.repo.RevokeKey(ctx, ref, s.clock.Now(), fx)
}
