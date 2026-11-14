package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/apiserver"
	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/operators/simulator"
	"github.com/zellapay/zellapay/internal/platform/ids"
	"github.com/zellapay/zellapay/internal/webhooks"
	"github.com/zellapay/zellapay/internal/wiring"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// account is a merchant with an API key.
type account struct {
	merchant merchants.Merchant
	key      string
}

// env is the production wiring running against real PostgreSQL and Redis.
type env struct {
	t      *testing.T
	srv    *httptest.Server
	c      *wiring.Container
	clock  *testClock
	main   account // read and collections:write
	other  account // a second merchant
	reader account // main merchant, read-only key
}

func newEnv(t *testing.T) *env {
	t.Helper()
	clk := &testClock{now: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	c, err := wiring.BuildWithClock(context.Background(), settings(), clk, log)
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, c: c, clock: clk}
	e.main = e.newAccount(150, merchants.ScopeRead, merchants.ScopeCollectionsWrite,
		merchants.ScopeWebhooksWrite, merchants.ScopePayoutsWrite)
	e.reader = account{
		merchant: e.main.merchant,
		key:      e.issue(e.main.merchant, merchants.ScopeRead),
	}
	e.other = e.newAccount(0, merchants.ScopeRead, merchants.ScopeCollectionsWrite)
	e.srv = httptest.NewServer(apiserver.New(c, apiserver.Options{}, log))
	t.Cleanup(func() { e.srv.Close(); c.Close() })
	return e
}

func settings() wiring.Settings {
	return wiring.Settings{
		Environment: "local", Mode: merchants.ModeTest,
		DatabaseURL: testDSN, RedisURL: testRedis,
		DataKeyID: "k1", DataKey: secret(7), BlindIndexKey: secret(9),
		SimulatorEnabled: true, SimulatorSecret: secret(5), SimulatorPrefix: "+241",
		WebhookPolicy:   webhooks.Policy{AllowPrivate: true, AllowHTTP: true},
		SMTPAddr:        "127.0.0.1:1026",
		MailFrom:        "ZellaPay <no-reply@zellapay.test>",
		SMSLogOnly:      true,
		SettlementDelay: 24 * time.Hour,
		PayoutMaxAmount: 5_000_000,
	}
}

func (e *env) newAccount(bps int, scopes ...merchants.Scope) account {
	e.t.Helper()
	ctx := context.Background()
	m, err := e.c.Merchants.CreateMerchant(ctx, "Acme "+ids.New(""), "XAF")
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.c.Merchants.SetStatus(ctx, m.ID, merchants.StatusActive); err != nil {
		e.t.Fatal(err)
	}
	if err := e.c.Merchants.SetFeeBPS(ctx, m.ID, bps); err != nil {
		e.t.Fatal(err)
	}
	return account{merchant: m, key: e.issue(m, scopes...)}
}

func (e *env) issue(m merchants.Merchant, scopes ...merchants.Scope) string {
	e.t.Helper()
	k, err := e.c.Merchants.IssueKey(context.Background(), merchants.KeySpec{
		MerchantID: m.ID, Mode: merchants.ModeTest, Scopes: scopes,
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return k.Secret
}

// response is a decoded HTTP response.
type response struct {
	status int
	header http.Header
	raw    []byte
}

func (r response) json(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.raw, &m); err != nil {
		t.Fatalf("not JSON (%d): %s", r.status, r.raw)
	}
	return m
}

func (r response) errorCode(t *testing.T) string {
	t.Helper()
	inner, _ := r.json(t)["error"].(map[string]any)
	code, _ := inner["code"].(string)
	return code
}

type request struct {
	method, path, key, idemKey, body string
}

func (e *env) do(r request) response {
	e.t.Helper()
	req, err := http.NewRequestWithContext(context.Background(),
		r.method, e.srv.URL+r.path, bytes.NewBufferString(r.body))
	if err != nil {
		e.t.Fatal(err)
	}
	if r.key != "" {
		req.Header.Set("Authorization", "Bearer "+r.key)
	}
	if r.idemKey != "" {
		req.Header.Set("Idempotency-Key", r.idemKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return response{status: resp.StatusCode, header: resp.Header, raw: raw}
}

// deliverCallbacks posts every due simulator callback to the callback
// endpoint over HTTP, as the operator would.
func (e *env) deliverCallbacks() int {
	e.t.Helper()
	due := e.c.Simulator.DueCallbacks()
	for _, cb := range due {
		req, err := http.NewRequestWithContext(context.Background(),
			http.MethodPost, e.srv.URL+"/callbacks/simulator", bytes.NewReader(cb.Body))
		if err != nil {
			e.t.Fatal(err)
		}
		req.Header.Set(simulator.SignatureHeader, cb.Signature)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			e.t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			e.t.Fatalf("callback returned %d", resp.StatusCode)
		}
	}
	return len(due)
}

func contextBackground() context.Context { return context.Background() }

func secret(fill byte) []byte { return bytes.Repeat([]byte{fill}, 32) }

func (e *env) get(path, key string) response {
	return e.do(request{method: "GET", path: path, key: key})
}

func (e *env) post(path, key, idemKey, body string) response {
	return e.do(request{method: "POST", path: path, key: key, idemKey: idemKey, body: body})
}

type jsonRaw = json.RawMessage

func unmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
