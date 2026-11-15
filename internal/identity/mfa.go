package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/platform/clock"
	"github.com/zellapay/zellapay/internal/platform/totp"
)

// UserLookup finds a user by ID.
type UserLookup interface {
	FindUser(ctx context.Context, id string) (User, error)
}

// MFADeps are the collaborators of the MFA service.
type MFADeps struct {
	Repo    MFARepository
	Users   UserLookup
	Box     SecretBox
	Auditor Auditor
	Clock   clock.Clock
	// Issuer names the service in the authenticator app.
	Issuer string
}

// MFA enrols and checks the TOTP second factor.
type MFA struct{ d MFADeps }

// NewMFA returns an MFA service.
func NewMFA(d MFADeps) (*MFA, error) {
	if d.Repo == nil || d.Users == nil || d.Box == nil || d.Auditor == nil ||
		d.Clock == nil || d.Issuer == "" {
		return nil, errors.New("identity: missing MFA dependency")
	}
	return &MFA{d: d}, nil
}

// Enrolment is what the user scans or types into their authenticator app.
type Enrolment struct {
	ManualKey string
	URI       string
}

func secretContext(userID string) []byte { return []byte("mfa:" + userID + ":secret") }

// Enrolled reports whether the user has an active second factor.
func (m *MFA) Enrolled(ctx context.Context, userID string) (bool, error) {
	c, err := m.d.Repo.FindMFA(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil && c.ConfirmedAt != nil, err
}

// BeginEnrolment creates a secret for the user's authenticator app. It is not
// active until ConfirmEnrolment proves the app produces matching codes.
func (m *MFA) BeginEnrolment(ctx context.Context, userID string) (Enrolment, error) {
	u, err := m.d.Users.FindUser(ctx, userID)
	if err != nil {
		return Enrolment{}, err
	}
	if on, err := m.Enrolled(ctx, userID); err != nil || on {
		if err == nil {
			err = ErrMFAAlreadyEnrolled
		}
		return Enrolment{}, err
	}
	secret, err := totp.NewSecret()
	if err != nil {
		return Enrolment{}, err
	}
	sealed, err := m.d.Box.Seal(secret, secretContext(userID))
	if err != nil {
		return Enrolment{}, fmt.Errorf("seal secret: %w", err)
	}
	err = m.d.Repo.SaveEnrolment(ctx, MFACredential{
		UserID: userID, SecretEnc: sealed, CreatedAt: m.d.Clock.Now(),
	})
	if err != nil {
		return Enrolment{}, err
	}
	return Enrolment{
		ManualKey: totp.Encode(secret),
		URI:       totp.URI(m.d.Issuer, u.Email, secret),
	}, nil
}

// ConfirmEnrolment activates the second factor once the user types a valid
// code. It returns the recovery codes, which are shown this once.
func (m *MFA) ConfirmEnrolment(ctx context.Context, userID, code string) ([]string, error) {
	c, err := m.d.Repo.FindMFA(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNoEnrolment
	}
	if err != nil {
		return nil, err
	}
	if c.ConfirmedAt != nil {
		return nil, ErrMFAAlreadyEnrolled
	}
	step, err := m.matchTOTP(c, code)
	if err != nil {
		return nil, err
	}
	shown, hashes, err := newRecoveryCodes()
	if err != nil {
		return nil, err
	}
	fx, err := m.effect("mfa.enrolled", userID)
	if err != nil {
		return nil, err
	}
	err = m.d.Repo.Confirm(ctx, Confirmation{
		UserID: userID, Step: step, CodeHashes: hashes, At: m.d.Clock.Now(),
	}, fx)
	if err != nil {
		return nil, err
	}
	return shown, nil
}

func (m *MFA) matchTOTP(c MFACredential, code string) (int64, error) {
	secret, err := m.d.Box.Open(c.SecretEnc, secretContext(c.UserID))
	if err != nil {
		return 0, fmt.Errorf("open secret: %w", err)
	}
	step, ok := totp.Verify(secret, normalizeCode(code), m.d.Clock.Now(), c.LastStep)
	if !ok {
		return 0, ErrInvalidCode
	}
	return step, nil
}

func (m *MFA) effect(action, userID string) (Effects, error) {
	e, err := m.d.Auditor.Prepare(audit.Entry{
		Actor:  audit.Actor{Kind: audit.ActorMerchantUser, ID: userID},
		Action: action, Target: audit.Target{Kind: "merchant_user", ID: userID},
	})
	if err != nil {
		return Effects{}, fmt.Errorf("prepare audit entry: %w", err)
	}
	return Effects{Audit: []audit.Pending{e}}, nil
}
