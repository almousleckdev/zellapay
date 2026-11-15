package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/platform/logging"
	"github.com/zellapay/zellapay/internal/staff"
	"github.com/zellapay/zellapay/internal/wiring"
)

// staffPasswordVar names the variable the password is read from. A password
// given as an argument would stay in shell history and the process list.
const staffPasswordVar = "ZP_STAFF_PASSWORD"

// staffCommand creates the first administrator. Later staff are added from
// the admin console, by an administrator, with an audit trail.
func staffCommand(ctx context.Context, args []string) error {
	if len(args) != 3 || args[0] != "create-super-admin" {
		return fmt.Errorf("%s", usage)
	}
	password := os.Getenv(staffPasswordVar)
	if password == "" {
		return errors.New(staffPasswordVar + ": set it to the password for the new account")
	}
	settings, err := wiring.SettingsFromEnv()
	if err != nil {
		return err
	}
	c, err := wiring.Build(ctx, settings, logging.New("zellactl", "warn"))
	if err != nil {
		return err
	}
	defer c.Close()
	u, err := c.Staff.Create(ctx, staff.CreateInput{
		Email: args[1], Name: args[2], Role: staff.RoleSuperAdmin, Password: password,
		By: audit.Actor{Kind: audit.ActorSystem, ID: "zellactl"},
	})
	if err != nil {
		return err
	}
	fmt.Printf("created %s as %s (%s)\n", u.Email, u.Role, u.ID)
	fmt.Println("they sign in at the admin console and register a passkey on first use")
	return nil
}
