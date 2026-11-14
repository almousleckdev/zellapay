package apiserver_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/ledger"
	"github.com/zellapay/zellapay/internal/ledger/ledgertest"
	ledgerstore "github.com/zellapay/zellapay/internal/ledger/pgstore"
)

func payoutBody(suffix string, amount int) string {
	return `{"amount":` + itoa(amount) + `,"currency":"XAF","recipient_phone":"+241070000` +
		suffix + `","merchant_reference":"PAYOUT-` + suffix + `"}`
}

func (e *env) pay(suffix, idem string, amount int) response {
	return e.post("/v1/payouts", e.main.key, idem, payoutBody(suffix, amount))
}

// fund credits the main merchant's available balance directly.
func (e *env) fund(t *testing.T, amount int64) {
	t.Helper()
	ledgertest.FundMerchant(t, ledgerstore.New(e.c.Pool), e.main.merchant.ID, amount)
}

func (e *env) availableBalance(t *testing.T) int64 {
	t.Helper()
	got, err := ledgerstore.New(e.c.Pool).Balance(context.Background(),
		ledger.MerchantAvailable(e.main.merchant.ID))
	if errors.Is(err, ledger.ErrUnknownAccount) {
		return 0 // nothing has ever been released to this merchant
	}
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// The whole journey: a customer pays, the funds settle after the delay, and
// the merchant sends part of them out.
func TestFromCustomerPaymentToMerchantPayout(t *testing.T) {
	e := newEnv(t)
	e.create("00", "idem-journey-1") // 10000 collected, fee 150
	e.clock.advance(3 * time.Second)
	e.deliverCallbacks()
	if _, err := e.c.Collections.Settle(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	if got := e.availableBalance(t); got != 0 {
		t.Fatalf("nothing is withdrawable before the settlement delay, got %d", got)
	}
	e.clock.advance(25 * time.Hour)
	if _, err := e.c.Collections.Settle(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	if got := e.availableBalance(t); got != 9850 {
		t.Fatalf("available after settlement = %d, want 9850", got)
	}

	r := e.pay("00", "idem-journey-2", 5000)
	if r.status != http.StatusCreated || r.json(t)["status"] != "pending" {
		t.Fatalf("payout: %d %s", r.status, r.raw)
	}
	id, _ := r.json(t)["id"].(string)
	e.clock.advance(3 * time.Second)
	e.deliverCallbacks()
	got := e.get("/v1/payouts/"+id, e.main.key).json(t)
	if got["status"] != "succeeded" || got["completed_at"] == nil {
		t.Fatalf("got %v", got)
	}
	if bal := e.availableBalance(t); bal != 4850 {
		t.Fatalf("available after payout = %d, want 4850", bal)
	}
}

func TestPayoutsNeedTheirOwnScopeAndAreMerchantScoped(t *testing.T) {
	e := newEnv(t)
	e.fund(t, 50_000)
	denied := e.post("/v1/payouts", e.reader.key, "idem-p-scope", payoutBody("00", 1000))
	if denied.status != http.StatusForbidden {
		t.Fatalf("a read key must not pay out: %d", denied.status)
	}
	id, _ := e.pay("00", "idem-p-own", 1000).json(t)["id"].(string)
	if r := e.get("/v1/payouts/"+id, e.other.key); r.status != http.StatusNotFound {
		t.Fatalf("another merchant must not see it: %d", r.status)
	}
	if r := e.get("/v1/payouts", ""); r.status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d", r.status)
	}
}

func TestInsufficientFundsIsRefusedAndTheRefusalIsReplayed(t *testing.T) {
	e := newEnv(t)
	e.fund(t, 2_000)
	first := e.pay("00", "idem-poor-1", 5_000)
	refused := first.status == http.StatusUnprocessableEntity
	if !refused || first.errorCode(t) != "insufficient_funds" {
		t.Fatalf("got %d %s", first.status, first.raw)
	}
	again := e.pay("00", "idem-poor-1", 5_000)
	if again.status != first.status || again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("the refusal must replay: %d", again.status)
	}
	if got := e.availableBalance(t); got != 2_000 {
		t.Fatalf("no money may move, available = %d", got)
	}
}

// withPhone returns a valid payout body addressed to phone.
func withPhone(phone string) string {
	return strings.Replace(payoutBody("00", 100), "+24107000000", phone, 1)
}

func TestPayoutValidation(t *testing.T) {
	e := newEnv(t)
	e.fund(t, 50_000)
	cases := map[string]struct {
		body string
		code string
		http int
	}{
		"not json":     {`nope`, "invalid_request", 400},
		"unknown":      {`{"amount":1,"metadata":{}}`, "invalid_request", 400},
		"zero amount":  {payoutBody("00", 0), "invalid_request", 400},
		"bad phone":    {withPhone("0707"), "invalid_request", 400},
		"unserved":     {withPhone("+33612345678"), "unsupported_number", 422},
		"over the cap": {payoutBody("00", 5_000_001), "amount_too_large", 422},
	}
	n := 0
	for name, c := range cases {
		n++
		r := e.post("/v1/payouts", e.main.key, "idem-pv-"+itoa(n), c.body)
		if r.status != c.http || r.errorCode(t) != c.code {
			t.Errorf("%s: got %d %s", name, r.status, r.raw)
		}
	}
	if got := e.availableBalance(t); got != 50_000 {
		t.Fatalf("invalid requests must not move money, available = %d", got)
	}
}

func TestPublicPayoutNeverLeaksThePhoneOrInternals(t *testing.T) {
	e := newEnv(t)
	e.fund(t, 50_000)
	r := e.pay("01", "idem-leak-1", 1_000)
	e.clock.advance(3 * time.Second)
	e.deliverCallbacks()
	id, _ := r.json(t)["id"].(string)
	got := e.get("/v1/payouts/"+id, e.main.key)
	body := string(got.raw)
	if strings.Contains(body, "+24107000001") || strings.Contains(body, "CUSTOMER_DECLINED") ||
		!strings.Contains(body, "+241 07 ** ** 01") {
		t.Fatalf("unexpected body: %s", body)
	}
	if m := got.json(t); m["status"] != "failed" || m["failure_code"] != "operator_error" {
		t.Fatalf("got %v", m)
	}
	if bal := e.availableBalance(t); bal != 50_000 {
		t.Fatalf("a failed payout must return the funds, available = %d", bal)
	}
}
