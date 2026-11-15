// Command zellactl bootstraps merchants and API keys for development and the
// sandbox. It refuses to run in production: live administration will go
// through the audited admin console.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/merchants"
	merchantstore "github.com/zellapay/zellapay/internal/merchants/pgstore"
	"github.com/zellapay/zellapay/internal/platform/clock"
)

const usage = `usage:
  zellactl merchant create <name> <currency>
  zellactl merchant activate <merchant-id>
  zellactl merchant fee <merchant-id> <basis-points>
  zellactl merchant payout-fee <merchant-id> <basis-points>
  zellactl key issue <merchant-id> <scope[,scope...]>
  zellactl notify test <email>     queue and send a test email (see MailDev)
  zellactl identity invite <merchant-id> <email> <role>   email the first user an invitation
  zellactl staff create-super-admin <email> <name>   password from ZP_STAFF_PASSWORD
  zellactl audit head              print the newest audit entry (copy it somewhere safe)
  zellactl audit verify [--full]   check the audit log chain

scopes: read, collections:write, payouts:write, webhooks:write
environment: ZP_DATABASE_URL (required), ZP_ENV (must not be production)`

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "zellactl:", err)
		os.Exit(1)
	}
}

// builtOnWiring are the commands that run the real services, so they need the
// full configuration rather than only a database.
var builtOnWiring = map[string]func(context.Context, []string) error{
	"notify":   notifyCommand,
	"staff":    staffCommand,
	"identity": identityCommand,
}

func run(ctx context.Context, args []string) error {
	if os.Getenv("ZP_ENV") == "production" {
		return fmt.Errorf("refusing to run in production")
	}
	if len(args) > 0 {
		if command, ok := builtOnWiring[args[0]]; ok {
			return command(ctx, args[1:])
		}
	}
	dsn := os.Getenv("ZP_DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("ZP_DATABASE_URL: required\n%s", usage)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()
	if len(args) > 0 && args[0] == "audit" {
		return auditCommand(ctx, pool, args[1:])
	}
	svc := merchants.NewService(merchantstore.New(pool), clock.System{}, merchants.ModeTest)
	return dispatch(ctx, svc, args)
}

func dispatch(ctx context.Context, svc *merchants.Service, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("%s", usage)
	}
	switch args[0] + " " + args[1] {
	case "merchant create":
		return merchantCreate(ctx, svc, args[2:])
	case "merchant activate":
		return merchantActivate(ctx, svc, args[2:])
	case "merchant fee":
		return merchantFee(ctx, svc, args[2:])
	case "merchant payout-fee":
		return merchantPayoutFee(ctx, svc, args[2:])
	case "key issue":
		return keyIssue(ctx, svc, args[2:])
	}
	return fmt.Errorf("%s", usage)
}
