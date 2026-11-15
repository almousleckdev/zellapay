package access

import (
	"errors"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/identity"
)

var now = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

func principal(role identity.Role, mfa identity.MFAState, mfaAge time.Duration) identity.Principal {
	s := identity.Session{}
	if mfaAge >= 0 {
		at := now.Add(-mfaAge)
		s.MFAAt = &at
	}
	u := identity.User{ID: "usr_1", MerchantID: "mer_1", Role: role}
	return identity.Principal{Session: s, User: u, MFA: mfa}
}

type checkCase struct {
	name string
	pr   identity.Principal
	perm Permission
	res  Resource
	want error
}

func checkCases() []checkCase {
	done, hour := identity.MFADone, time.Hour
	owner := principal(identity.RoleOwner, done, hour)
	freshOwner := principal(identity.RoleOwner, done, 0)
	pending := principal(identity.RoleOwner, identity.MFARequired, -1)
	enrol := principal(identity.RoleSupport, identity.MFAEnrolRequired, -1)
	support := principal(identity.RoleSupport, done, hour)
	staleDev := principal(identity.RoleDeveloper, done, 11*time.Minute)
	freshDev := principal(identity.RoleDeveloper, done, 9*time.Minute)
	staleFin := principal(identity.RoleFinance, done, hour)
	freshFin := principal(identity.RoleFinance, done, time.Minute)
	mine, theirs := Resource{MerchantID: "mer_1"}, Resource{MerchantID: "mer_2"}
	large := Resource{Amount: 100_000}
	return []checkCase{
		{"owner reads", owner, CollectionsRead, Resource{}, nil},
		{"support cannot pay out", support, PayoutsCreate, Resource{}, ErrForbidden},
		{"other merchant", freshOwner, CollectionsRead, theirs, ErrForbidden},
		{"own merchant", freshOwner, CollectionsRead, mine, nil},
		{"mfa pending", pending, CollectionsRead, Resource{}, ErrMFARequired},
		{"enrolment pending", enrol, BalanceRead, Resource{}, ErrMFARequired},
		{"profile before mfa", enrol, ProfileUpdate, Resource{}, nil},
		{"keys, stale factor", staleDev, APIKeysManage, Resource{}, ErrStepUpRequired},
		{"keys, fresh factor", freshDev, APIKeysManage, Resource{}, nil},
		{"small payout", staleFin, PayoutsCreate, Resource{Amount: 99_999}, nil},
		{"large payout, stale", staleFin, PayoutsCreate, large, ErrStepUpRequired},
		{"large payout, fresh", freshFin, PayoutsCreate, Resource{Amount: 5_000_000}, nil},
		{"role before step-up", support, APIKeysManage, Resource{}, ErrForbidden},
	}
}

func TestCheck(t *testing.T) {
	pol := NewPolicy(DefaultConfig())
	for _, tc := range checkCases() {
		t.Run(tc.name, func(t *testing.T) {
			if err := pol.Check(tc.pr, tc.perm, tc.res, now); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestEveryPayoutNeedsStepUpWhenTheThresholdIsZero(t *testing.T) {
	pol := NewPolicy(Config{StepUpWindow: time.Minute})
	pr := principal(identity.RoleFinance, identity.MFADone, time.Hour)
	err := pol.Check(pr, PayoutsCreate, Resource{Amount: 1}, now)
	if !errors.Is(err, ErrStepUpRequired) {
		t.Fatalf("err = %v", err)
	}
}
