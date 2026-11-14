package pgstore_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/zellapay/zellapay/internal/platform/pgtest"
	"github.com/zellapay/zellapay/internal/webhooks"
	"github.com/zellapay/zellapay/internal/webhooks/webhookstest"
)

func TestSecretIsShownOnceAndStoredEncrypted(t *testing.T) {
	w := newWorld(t)
	r := webhookstest.New(200)
	defer r.Close()
	ep := w.endpoint(t, r)
	if !strings.HasPrefix(ep.Secret, "whsec_") || len(ep.Secret) < 40 {
		t.Fatalf("unexpected secret %q", ep.Secret)
	}
	var stored string
	err := pgtest.NewPool(t, testDSN).QueryRow(context.Background(),
		"SELECT secret_enc FROM webhook_endpoints WHERE id = $1", ep.ID).Scan(&stored)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, ep.Secret) || strings.Contains(stored, ep.Secret[6:]) ||
		!strings.HasPrefix(stored, "v1:k1:") {
		t.Fatalf("the secret must be stored sealed, got %q", stored)
	}
	list, _ := w.svc.ListEndpoints(context.Background(), w.merchant.ID)
	if len(list) != 1 || list[0].ID != ep.ID || !list[0].Active {
		t.Fatalf("list: %+v", list)
	}
}

const okURL = "https://example.com/h"

func TestEndpointValidation(t *testing.T) {
	w := newWorld(t)
	prod := newProductionWorld(t, w)
	bad := map[string]webhooks.CreateEndpointInput{
		"no events":     {MerchantID: w.merchant.ID, URL: "https://example.com/h"},
		"unknown event": {MerchantID: w.merchant.ID, URL: okURL, Events: []string{"nope"}},
		"repeated": {
			MerchantID: w.merchant.ID, URL: okURL,
			Events: []string{"collection.failed", "collection.failed"},
		},
		"no merchant": {URL: "https://example.com/h", Events: []string{"collection.failed"}},
	}
	for name, in := range bad {
		_, err := prod.CreateEndpoint(context.Background(), in)
		if !errors.Is(err, webhooks.ErrInvalidInput) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	unsafe := []string{
		"http://example.com/h", "https://127.0.0.1/h",
		"https://169.254.169.254/x", "https://localhost/h",
	}
	for _, u := range unsafe {
		_, err := prod.CreateEndpoint(context.Background(), webhooks.CreateEndpointInput{
			MerchantID: w.merchant.ID, URL: u, Events: []string{"collection.failed"},
		})
		if !errors.Is(err, webhooks.ErrUnsafeURL) {
			t.Errorf("%s: got %v", u, err)
		}
	}
}

func TestTheEndpointLimitHoldsUnderConcurrency(t *testing.T) {
	w := newWorld(t)
	var created, refused atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := w.svc.CreateEndpoint(context.Background(), webhooks.CreateEndpointInput{
				MerchantID: w.merchant.ID, URL: "http://127.0.0.1:9/h",
				Events: []string{"collection.failed"},
			})
			switch {
			case err == nil:
				created.Add(1)
			case errors.Is(err, webhooks.ErrTooManyEndpoints):
				refused.Add(1)
			default:
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	if created.Load() != 10 || refused.Load() != 15 {
		t.Fatalf("created %d, refused %d; want 10 and 15", created.Load(), refused.Load())
	}
}
