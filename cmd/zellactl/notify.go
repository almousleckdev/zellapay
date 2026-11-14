package main

import (
	"context"
	"fmt"

	"github.com/zellapay/zellapay/internal/notify"
	"github.com/zellapay/zellapay/internal/platform/logging"
	"github.com/zellapay/zellapay/internal/wiring"
)

// notifyCommand sends a test email through the real notification pipeline,
// so a developer can see it arrive in MailDev.
func notifyCommand(ctx context.Context, args []string) error {
	if len(args) != 2 || args[0] != "test" {
		return fmt.Errorf("%s", usage)
	}
	settings, err := wiring.SettingsFromEnv()
	if err != nil {
		return err
	}
	log := logging.New("zellactl", "warn")
	c, err := wiring.Build(ctx, settings, log)
	if err != nil {
		return err
	}
	defer c.Close()
	return sendTest(ctx, c, args[1])
}

func sendTest(ctx context.Context, c *wiring.Container, to string) error {
	id, _, err := c.Notify.Enqueue(ctx, notify.Message{
		To: to, Template: "sign_in_alert", Locale: notify.LocaleFrench,
		Data: map[string]string{"when": "maintenant", "device": "zellactl (message de test)"},
	})
	if err != nil {
		return err
	}
	res, err := c.Notify.DeliverDue(ctx, 10)
	if err != nil {
		return err
	}
	fmt.Printf("queued %s; delivery: sent=%d retrying=%d dead=%d\n",
		id, res.Sent, res.Retrying, res.Dead)
	if res.Sent > 0 {
		fmt.Println("open the inbox at http://localhost:1081")
	}
	return nil
}
