package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/zellapay/zellapay/internal/merchants"
)

func merchantCreate(ctx context.Context, svc *merchants.Service, a []string) error {
	if len(a) != 2 {
		return fmt.Errorf("%s", usage)
	}
	m, err := svc.CreateMerchant(ctx, a[0], a[1])
	if err != nil {
		return err
	}
	fmt.Printf("merchant %s created (status: %s)\n", m.ID, m.Status)
	return nil
}

func merchantActivate(ctx context.Context, svc *merchants.Service, a []string) error {
	if len(a) != 1 {
		return fmt.Errorf("%s", usage)
	}
	if err := svc.SetStatus(ctx, a[0], merchants.StatusActive); err != nil {
		return err
	}
	fmt.Printf("merchant %s is active\n", a[0])
	return nil
}

func merchantFee(ctx context.Context, svc *merchants.Service, a []string) error {
	if len(a) != 2 {
		return fmt.Errorf("%s", usage)
	}
	bps, err := strconv.Atoi(a[1])
	if err != nil {
		return fmt.Errorf("basis points must be a whole number")
	}
	if err := svc.SetFeeBPS(ctx, a[0], bps); err != nil {
		return err
	}
	fmt.Printf("merchant %s fee set to %d basis points\n", a[0], bps)
	return nil
}

func merchantPayoutFee(ctx context.Context, svc *merchants.Service, a []string) error {
	if len(a) != 2 {
		return fmt.Errorf("%s", usage)
	}
	bps, err := strconv.Atoi(a[1])
	if err != nil {
		return fmt.Errorf("basis points must be a whole number")
	}
	if err := svc.SetPayoutFeeBPS(ctx, a[0], bps); err != nil {
		return err
	}
	fmt.Printf("merchant %s payout fee set to %d basis points\n", a[0], bps)
	return nil
}

// keyIssue prints the secret once. It is not stored and cannot be shown again.
func keyIssue(ctx context.Context, svc *merchants.Service, a []string) error {
	if len(a) != 2 {
		return fmt.Errorf("%s", usage)
	}
	var scopes []merchants.Scope
	for _, s := range strings.Split(a[1], ",") {
		scopes = append(scopes, merchants.Scope(strings.TrimSpace(s)))
	}
	k, err := svc.IssueKey(ctx, merchants.KeySpec{
		MerchantID: a[0], Mode: merchants.ModeTest, Scopes: scopes,
	})
	if err != nil {
		return err
	}
	fmt.Printf("key %s issued. Copy the secret now; it is shown only once:\n%s\n",
		k.Key.ID, k.Secret)
	return nil
}
