package identity

import (
	"context"
	"errors"
	"regexp"
)

var sixDigits = regexp.MustCompile(`^[0-9]{6}$`)

// Verification is the outcome of a correct code.
type Verification struct {
	// RecoveryUsed is true when a recovery code was spent. The user should
	// then be asked to set up their authenticator again.
	RecoveryUsed bool
	// CodesLeft is how many recovery codes remain, when one was spent.
	CodesLeft int
}

// Verify checks a six-digit authenticator code or a recovery code. A code
// that was already accepted never works again.
func (m *MFA) Verify(ctx context.Context, userID, code string) (Verification, error) {
	norm := normalizeCode(code)
	if sixDigits.MatchString(norm) {
		return Verification{}, m.verifyTOTP(ctx, userID, norm)
	}
	return m.verifyRecovery(ctx, userID, norm)
}

func (m *MFA) verifyTOTP(ctx context.Context, userID, code string) error {
	c, err := m.d.Repo.FindMFA(ctx, userID)
	if errors.Is(err, ErrNotFound) || (err == nil && c.ConfirmedAt == nil) {
		return ErrInvalidCode
	}
	if err != nil {
		return err
	}
	step, err := m.matchTOTP(c, code)
	if err != nil {
		return err
	}
	fresh, err := m.d.Repo.AdvanceStep(ctx, userID, step)
	if err != nil {
		return err
	}
	if !fresh {
		return ErrInvalidCode
	}
	return nil
}

func (m *MFA) verifyRecovery(ctx context.Context, userID, code string) (Verification, error) {
	if len(code) != 16 {
		return Verification{}, ErrInvalidCode
	}
	fx, err := m.effect("mfa.recovery_code_used", userID)
	if err != nil {
		return Verification{}, err
	}
	ok, left, err := m.d.Repo.UseRecoveryCode(ctx, RecoveryUse{
		UserID: userID, Hash: hashRecovery(code), At: m.d.Clock.Now(),
	}, fx)
	if err != nil {
		return Verification{}, err
	}
	if !ok {
		return Verification{}, ErrInvalidCode
	}
	return Verification{RecoveryUsed: true, CodesLeft: left}, nil
}
