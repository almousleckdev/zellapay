package main

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zellapay/zellapay/internal/audit"
	auditstore "github.com/zellapay/zellapay/internal/audit/pgstore"
	"github.com/zellapay/zellapay/internal/platform/clock"
)

// auditCommand inspects the audit log. The head hash is a checkpoint: copy
// it somewhere that cannot be rewritten, and any later trimming of the log
// can be proved.
func auditCommand(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	svc := audit.NewService(auditstore.New(pool), clock.System{})
	switch {
	case len(args) == 1 && args[0] == "head":
		return printHead(ctx, svc)
	case len(args) >= 1 && args[0] == "verify":
		return runVerify(ctx, svc, len(args) == 2 && args[1] == "--full")
	}
	return fmt.Errorf("%s", usage)
}

func printHead(ctx context.Context, svc *audit.Service) error {
	h, err := svc.Head(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("seq=%d hash=%s\n", h.Seq, hex.EncodeToString(h.Hash[:]))
	return nil
}

func runVerify(ctx context.Context, svc *audit.Service, full bool) error {
	verify := svc.Verify
	if full {
		verify = svc.VerifyFull
	}
	res, err := verify(ctx, 1000)
	if err != nil {
		return err
	}
	if res.Broken != nil {
		return fmt.Errorf("THE CHAIN IS BROKEN at entry %d: %s (verified through %d)",
			res.Broken.Seq, res.Broken.Reason, res.VerifiedThrough)
	}
	fmt.Printf("ok: checked %d entries, intact through %d\n", res.Checked, res.VerifiedThrough)
	return nil
}
