package main

import (
	"context"
	"strings"
	"testing"
)

func TestDevSeedRefusesOutsideLocal(t *testing.T) {
	for _, env := range []string{"staging", "test", ""} {
		t.Setenv("ZP_ENV", env)
		t.Setenv("ZP_DEV_STAFF_PASSWORD", "some long local password")
		err := devCommand(context.Background(), []string{"seed"})
		if err == nil || !strings.Contains(err.Error(), "ZP_ENV=local") {
			t.Fatalf("ZP_ENV=%q: err = %v", env, err)
		}
	}
}

func TestDevSeedNeedsTheDevVariables(t *testing.T) {
	t.Setenv("ZP_ENV", "local")
	t.Setenv("ZP_DEV_STAFF_EMAIL", "admin@zellapay.test")
	t.Setenv("ZP_DEV_STAFF_NAME", "DevAdmin")
	t.Setenv("ZP_DEV_STAFF_PASSWORD", "")
	err := devCommand(context.Background(), []string{"seed"})
	if err == nil || !strings.Contains(err.Error(), "make env") {
		t.Fatalf("err = %v", err)
	}
}

func TestDevCommandRejectsOtherArguments(t *testing.T) {
	if err := devCommand(context.Background(), []string{"wipe"}); err == nil {
		t.Fatal("an unknown dev command was accepted")
	}
}
