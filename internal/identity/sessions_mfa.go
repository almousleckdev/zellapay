package identity

import (
	"context"
	"fmt"
)

// MFAInput is a second-factor answer for a session.
type MFAInput struct {
	Token string
	Code  string
	IP    string
}

// CompleteMFA proves the second factor for a session, at sign-in or later.
// Wrong answers are counted per user, so guessing six digits is slowed down.
func (s *Sessions) CompleteMFA(ctx context.Context, in MFAInput) (Verification, error) {
	p, err := s.Resolve(ctx, in.Token)
	if err != nil {
		return Verification{}, err
	}
	// A session that already passed MFA may answer again: that is how it
	// proves a recent second factor before a sensitive action.
	if p.MFA != MFARequired && p.MFA != MFADone {
		return Verification{}, invalid("mfa cannot be answered for this session")
	}
	a := Attempt{Realm: RealmMFA, Account: p.User.ID, IP: in.IP}
	if err := s.guard(ctx, a); err != nil {
		return Verification{}, err
	}
	v, err := s.d.Factor.Verify(ctx, p.User.ID, in.Code)
	if err != nil {
		return Verification{}, s.failed(ctx, a, err)
	}
	s.succeeded(ctx, a)
	if err := s.d.Repo.SetMFA(ctx, p.Session.ID, s.d.Clock.Now()); err != nil {
		return Verification{}, fmt.Errorf("record mfa: %w", err)
	}
	return v, nil
}

// BeginMFAEnrolment starts setting up the authenticator app for the user
// behind a session that has not enrolled yet.
func (s *Sessions) BeginMFAEnrolment(ctx context.Context, token string) (Enrolment, error) {
	p, err := s.Resolve(ctx, token)
	if err != nil {
		return Enrolment{}, err
	}
	if p.MFA != MFAEnrolRequired {
		return Enrolment{}, ErrMFAAlreadyEnrolled
	}
	return s.d.Factor.BeginEnrolment(ctx, p.User.ID)
}

// ConfirmMFAEnrolment finishes enrolment with the first code. The session
// counts as having proven the second factor, and the recovery codes are
// returned for the user to keep.
func (s *Sessions) ConfirmMFAEnrolment(
	ctx context.Context, in MFAInput,
) ([]string, error) {
	p, err := s.Resolve(ctx, in.Token)
	if err != nil {
		return nil, err
	}
	a := Attempt{Realm: RealmMFA, Account: p.User.ID, IP: in.IP}
	if err := s.guard(ctx, a); err != nil {
		return nil, err
	}
	codes, err := s.d.Factor.ConfirmEnrolment(ctx, p.User.ID, in.Code)
	if err != nil {
		return nil, s.failed(ctx, a, err)
	}
	s.succeeded(ctx, a)
	if err := s.d.Repo.SetMFA(ctx, p.Session.ID, s.d.Clock.Now()); err != nil {
		return nil, fmt.Errorf("record mfa: %w", err)
	}
	return codes, nil
}
