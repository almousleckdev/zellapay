package pgstore_test

import (
	"context"
	"strings"
	"testing"

	"github.com/zellapay/zellapay/internal/platform/pgtest"
)

func TestPhoneNumberIsNeverStoredInTheClear(t *testing.T) {
	w := newWorld(t, 0)
	c := w.create(t, "00")
	pool := pgtest.NewPool(t, testDSN)
	var enc, masked string
	var idx []byte
	err := pool.QueryRow(context.Background(),
		`SELECT customer_phone_enc, customer_phone_masked, customer_phone_idx
		   FROM collections WHERE id = $1`, c.ID).Scan(&enc, &masked, &idx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(enc, "2410700") || strings.Contains(enc, phone("00")) {
		t.Fatalf("sealed value reveals the number: %s", enc)
	}
	if !strings.HasPrefix(enc, "v1:k1:") || len(idx) != 32 {
		t.Fatalf("unexpected sealed value or index: %q (%d bytes)", enc, len(idx))
	}
	if masked != "+241 07 ** ** 00" || strings.Contains(masked, "0000") {
		t.Fatalf("masked value = %q", masked)
	}
}

func TestFinalCollectionsCannotBeChangedEvenByRawSQL(t *testing.T) {
	w := newWorld(t, 0)
	c := w.create(t, "08") // rejected at initiation, so already failed
	pool := pgtest.NewPool(t, testDSN)
	_, err := pool.Exec(context.Background(),
		"UPDATE collections SET status = 'succeeded' WHERE id = $1", c.ID)
	if err == nil {
		t.Fatal("the database must refuse to change a final status")
	}
}

func TestEventsCannotBeAltered(t *testing.T) {
	w := newWorld(t, 0)
	c := w.create(t, "00")
	pool := pgtest.NewPool(t, testDSN)
	for _, stmt := range []string{
		"UPDATE collection_events SET type = 'x' WHERE collection_id = $1",
		"DELETE FROM collection_events WHERE collection_id = $1",
	} {
		if _, err := pool.Exec(context.Background(), stmt, c.ID); err == nil {
			t.Errorf("%q must be rejected", stmt)
		}
	}
}
