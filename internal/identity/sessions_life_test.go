package identity

import (
	"context"
	"errors"
	"testing"
	"time"
)

func resolve(r *rig, token string) error {
	_, err := r.sessions.Resolve(context.Background(), token)
	return err
}

func TestSessionsExpireWhenIdleAndAtTheAbsoluteLimit(t *testing.T) {
	r := newRig(t)
	r.signUp(t, "jane@example.com")
	s, _ := r.login("jane@example.com", goodPhrase)
	r.clock.advance(20 * time.Minute)
	if err := resolve(r, s.Token); err != nil {
		t.Fatalf("active session refused: %v", err)
	}
	r.clock.advance(31 * time.Minute)
	if err := resolve(r, s.Token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("idle session: %v", err)
	}
	busy, _ := r.login("jane@example.com", goodPhrase)
	for range 26 {
		r.clock.advance(29 * time.Minute)
		_ = resolve(r, busy.Token)
	}
	if err := resolve(r, busy.Token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("a session outlived its absolute limit: %v", err)
	}
}

func TestUnknownAndEndedTokensLookTheSame(t *testing.T) {
	r := newRig(t)
	r.signUp(t, "jane@example.com")
	s, _ := r.login("jane@example.com", goodPhrase)
	ctx := context.Background()
	if err := r.sessions.Logout(ctx, s.Token); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{s.Token, "nonsense"} {
		if err := resolve(r, token); !errors.Is(err, ErrInvalidSession) {
			t.Errorf("token %q: %v", token, err)
		}
	}
	if err := r.sessions.Logout(ctx, "nonsense"); err != nil {
		t.Fatalf("logout of an unknown token: %v", err)
	}
}

func TestPeopleManageOnlyTheirOwnSessions(t *testing.T) {
	r := newRig(t)
	jane := r.signUp(t, "jane@example.com")
	r.signUp(t, "omar@example.com")
	a, _ := r.login("jane@example.com", goodPhrase)
	b, _ := r.login("jane@example.com", goodPhrase)
	o, _ := r.login("omar@example.com", goodPhrase)
	ctx := context.Background()
	if err := r.sessions.End(ctx, jane.ID, o.Session.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ended someone else's session: %v", err)
	}
	n, err := r.sessions.EndOthers(ctx, jane.ID, a.Session.ID)
	if err != nil || n != 1 {
		t.Fatalf("ended %d, %v", n, err)
	}
	if resolve(r, a.Token) != nil || resolve(r, b.Token) == nil || resolve(r, o.Token) != nil {
		t.Fatal("wrong sessions were ended")
	}
	list, _ := r.sessions.List(ctx, jane.ID)
	if len(list) != 1 || list[0].ID != a.Session.ID {
		t.Fatalf("list = %+v", list)
	}
}

func TestResettingThePasswordSignsEveryoneOut(t *testing.T) {
	r := newRig(t)
	r.signUp(t, "jane@example.com")
	s, _ := r.login("jane@example.com", goodPhrase)
	ctx := context.Background()
	_ = r.svc.RequestPasswordReset(ctx, "jane@example.com")
	token := tokenIn(t, r.mail.last(t).Data["link"])
	err := r.svc.CompletePasswordReset(ctx, ResetInput{Token: token, NewPassword: next})
	if err != nil {
		t.Fatal(err)
	}
	if err := resolve(r, s.Token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("a session survived a password reset: %v", err)
	}
}

func TestADisabledUsersSessionStopsWorking(t *testing.T) {
	r := newRig(t)
	u := r.signUp(t, "jane@example.com")
	s, _ := r.login("jane@example.com", goodPhrase)
	u.Status = StatusDisabled
	r.repo.users[u.ID] = u
	if err := resolve(r, s.Token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("err = %v", err)
	}
}
