package webhooks

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/zellapay/zellapay/internal/platform/ids"
)

const (
	secretPrefix   = "whsec_"
	secretBytes    = 32
	maxDescription = 200
)

// CreateEndpointInput describes an endpoint to register.
type CreateEndpointInput struct {
	MerchantID  string
	URL         string
	Description string
	Events      []string
}

func (s *Service) validate(in CreateEndpointInput) error {
	if in.MerchantID == "" || len(in.Description) > maxDescription {
		return fmt.Errorf("%w: merchant or description", ErrInvalidInput)
	}
	if len(in.Events) == 0 {
		return fmt.Errorf("%w: at least one event is required", ErrInvalidInput)
	}
	seen := map[string]bool{}
	for _, e := range in.Events {
		if !KnownEvent(e) || seen[e] {
			return fmt.Errorf("%w: unknown or repeated event %q", ErrInvalidInput, e)
		}
		seen[e] = true
	}
	return s.cfg.Policy.ValidateURL(in.URL)
}

// CreateEndpoint registers an endpoint. The returned secret is the only copy
// the merchant will ever see; ZellaPay keeps it encrypted.
func (s *Service) CreateEndpoint(
	ctx context.Context, in CreateEndpointInput,
) (CreatedEndpoint, error) {
	if err := s.validate(in); err != nil {
		return CreatedEndpoint{}, err
	}
	secret, err := newSecret(s.rand)
	if err != nil {
		return CreatedEndpoint{}, err
	}
	id := ids.New("we_")
	sealed, err := s.d.Box.Seal([]byte(secret), secretContext(id))
	if err != nil {
		return CreatedEndpoint{}, fmt.Errorf("seal secret: %w", err)
	}
	ep := Endpoint{
		ID: id, MerchantID: in.MerchantID, URL: in.URL, Events: in.Events,
		Description: in.Description, Active: true, CreatedAt: s.now(),
	}
	stored := SealedEndpoint{Endpoint: ep, SecretSealed: sealed}
	if err := s.d.Repo.CreateEndpoint(ctx, stored, s.cfg.MaxEndpoints); err != nil {
		return CreatedEndpoint{}, err
	}
	return CreatedEndpoint{Endpoint: ep, Secret: secret}, nil
}

// ListEndpoints returns a merchant's endpoints without their secrets.
func (s *Service) ListEndpoints(ctx context.Context, merchantID string) ([]Endpoint, error) {
	return s.d.Repo.ListEndpoints(ctx, merchantID)
}

func newSecret(r io.Reader) (string, error) {
	b := make([]byte, secretBytes)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", fmt.Errorf("read entropy: %w", err)
	}
	return secretPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

// secretContext binds a sealed secret to its endpoint so it cannot be moved.
func secretContext(endpointID string) []byte {
	return []byte("webhook_endpoint:" + endpointID + ":secret")
}
