package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/zellapay/zellapay/internal/identity"
)

// devCommand holds the commands that exist only for local development.
func devCommand(ctx context.Context, args []string) error {
	if len(args) != 1 || args[0] != "seed" {
		return fmt.Errorf("%s", usage)
	}
	if os.Getenv("ZP_ENV") != "local" {
		return errors.New("dev seed: runs only when ZP_ENV=local")
	}
	email, name := os.Getenv("ZP_DEV_STAFF_EMAIL"), os.Getenv("ZP_DEV_STAFF_NAME")
	password := os.Getenv("ZP_DEV_STAFF_PASSWORD")
	if email == "" || name == "" || password == "" {
		return errors.New("ZP_DEV_STAFF_EMAIL, _NAME and _PASSWORD: run `make env` to add them")
	}
	_, err := createSuperAdmin(ctx, email, name, password)
	if errors.Is(err, identity.ErrEmailTaken) {
		fmt.Printf("dev staff account %s already exists\n", email)
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("created dev staff account %s\n", email)
	fmt.Println("its password is ZP_DEV_STAFF_PASSWORD in .env.local")
	return nil
}
