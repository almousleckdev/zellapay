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

scopes: read, collections:write, payouts:write, webhooks:write
environment: ZP_DATABASE_URL (required), ZP_ENV (must not be production)`

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "zellactl:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if os.Getenv("ZP_ENV") == "production" {
		return fmt.Errorf("refusing to run in production")
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
