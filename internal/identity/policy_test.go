package identity

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		hints    []string
		wantErr  bool
	}{
		{"long passphrase", "tournesol bleu sur la table", nil, false},
		{"exactly twelve", "abcdefgh1234", nil, false},
		{"eleven is short", "abcdefgh123", nil, true},
		{"too long", strings.Repeat("a1", 65), nil, true},
		{"one repeated character", strings.Repeat("a", 20), nil, true},
		{"well known", "Password1234", nil, true},
		{"known in other case", "QWERTYUIOP123", nil, true},
		{"is the email", "jane@example.com", []string{"jane@example.com"}, true},
		{"unicode counts characters", "éééééééééééé", nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckPassword(tc.password, tc.hints...)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && !errors.Is(err, ErrWeakPassword) {
				t.Fatalf("err = %v, want ErrWeakPassword", err)
			}
		})
	}
}
