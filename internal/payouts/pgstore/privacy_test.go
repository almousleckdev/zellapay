package pgstore_test

import (
	"context"
	"strings"
	"testing"

	"github.com/zellapay/zellapay/internal/payouts"
	"github.com/zellapay/zellapay/internal/platform/pgtest"
)

func TestRecipientPhoneIsNeverStoredInTheClear(t *testing.T) {
	w := newWorld(t, 0, 20_000)
	p := w.create(t, "00", 1_000)
	var enc, masked string
	var idx []byte
	err := pgtest.NewPool(t, testDSN).QueryRow(context.Background(),
		`SELECT recipient_phone_enc, recipient_phone_masked, recipient_phone_idx
		   FROM payouts WHERE id = $1`, p.ID).Scan(&enc, &masked, &idx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(enc, "2410700") || !strings.HasPrefix(enc, "v1:k1:") || len(idx) != 32 {
		t.Fatalf("sealed value %q, index %d bytes", enc, len(idx))
	}
	if masked != "+241 07 ** ** 00" {
		t.Fatalf("masked = %q", masked)
	}
}

func TestFinalPayoutsAndTheirHistoryCannotBeChanged(t *testing.T) {
	w := newWorld(t, 0, 20_000)
	p := w.create(t, "08", 1_000) // refused by the operator: already final
	if got := w.reload(t, p.ID).Status; got != payouts.StatusFailed {
		t.Fatalf("setup: %s", got)
	}
	pool := pgtest.NewPool(t, testDSN)
	for _, stmt := range []string{
		"UPDATE payouts SET status = 'succeeded' WHERE id = $1",
		"UPDATE payout_events SET type = 'x' WHERE payout_id = $1",
		"DELETE FROM payout_events WHERE payout_id = $1",
		"DELETE FROM payout_outbox WHERE payout_id = $1",
		"UPDATE payout_outbox SET event_type = 'payout.succeeded' WHERE payout_id = $1",
	} {
		if _, err := pool.Exec(context.Background(), stmt, p.ID); err == nil {
			t.Errorf("%q must be rejected", stmt)
		}
	}
}

func TestOnlySucceededAndFailedPayoutsAreAnnounced(t *testing.T) {
	w := newWorld(t, 0, 5_000)
	drain := func() []payouts.OutboxEntry {
		var got []payouts.OutboxEntry
		_, err := w.store.ClaimOutbox(context.Background(), w.clock.Now(), 1000,
			func(_ context.Context, e []payouts.OutboxEntry) error {
				got = append(got, e...)
				return nil
			})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	drain()
	// Too poor to pay: rejected, and rejections are not announced.
	refused, _ := w.svc.Create(context.Background(), w.input("00", "key-out-1", 9_000))
	failed := w.create(t, "08", 1_000)
	got := drain()
	types := map[string]string{}
	for _, e := range got {
		types[e.PayoutID] = e.EventType
	}
	if _, announced := types[refused.ID]; announced {
		t.Fatal("a rejected payout must not be announced")
	}
	if types[failed.ID] != payouts.OutboxFailed {
		t.Fatalf("events: %v", types)
	}
}
