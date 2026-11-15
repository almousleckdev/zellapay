package identity

import (
	"context"
	"errors"
	"testing"
)

func TestAuthenticate(t *testing.T) {
	r := newRig(t)
	r.signUp(t, "jane@example.com")
	ctx := context.Background()
	tests := []struct {
		name, email, password string
		ok                    bool
	}{
		{"right", "Jane@Example.com", goodPhrase, true},
		{"wrong password", "jane@example.com", "not the password", false},
		{"unknown account", "nobody@example.com", goodPhrase, false},
		{"malformed email", "nobody", goodPhrase, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := r.svc.Authenticate(ctx, tc.email, tc.password)
			if tc.ok && (err != nil || u.Email != "jane@example.com") {
				t.Fatalf("u=%+v err=%v", u, err)
			}
			if !tc.ok && !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("err = %v, want the one generic error", err)
			}
		})
	}
}

func TestAuthenticate_RefusesADisabledAccount(t *testing.T) {
	r := newRig(t)
	u := r.signUp(t, "jane@example.com")
	u.Status = StatusDisabled
	r.repo.users[u.ID] = u
	_, err := r.svc.Authenticate(context.Background(), u.Email, goodPhrase)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v", err)
	}
}

func TestAuthenticate_UpgradesAnOlderHash(t *testing.T) {
	r := newRig(t)
	u := r.signUp(t, "jane@example.com")
	stronger := Params{Memory: 16, Passes: 2, Lanes: 1, Version: 1}
	r.svc.d.Hasher = testHasher(t, stronger, 1, testPepper)
	if _, err := r.svc.Authenticate(context.Background(), u.Email, goodPhrase); err != nil {
		t.Fatal(err)
	}
	if got := r.repo.users[u.ID].PasswordHash; got == u.PasswordHash {
		t.Fatal("hash was not upgraded")
	}
	if _, err := r.svc.Authenticate(context.Background(), u.Email, goodPhrase); err != nil {
		t.Fatalf("sign-in after upgrade: %v", err)
	}
}
