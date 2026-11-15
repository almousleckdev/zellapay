package main

import (
	"context"
	"fmt"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/platform/logging"
	"github.com/zellapay/zellapay/internal/wiring"
)

// identityCommand bootstraps the first user of a merchant. In production this
// is done by the approval flow; here it lets a developer sign in locally.
func identityCommand(ctx context.Context, args []string) error {
	if len(args) != 4 || args[0] != "invite" {
		return fmt.Errorf("%s", usage)
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
	inv, err := c.Identity.Invite(ctx, identity.InviteInput{
		MerchantID: args[1], Email: args[2], Role: identity.Role(args[3]),
		InvitedBy: audit.Actor{Kind: audit.ActorSystem, ID: "zellactl"},
	})
	if err != nil {
		return err
	}
	res, err := c.Notify.DeliverDue(ctx, 10)
	if err != nil {
		return err
	}
	fmt.Printf("invited %s as %s (%s); emails sent=%d\n", args[2], args[3], inv.ID, res.Sent)
	fmt.Println("open the invitation in MailDev at http://localhost:1081")
	return nil
}
