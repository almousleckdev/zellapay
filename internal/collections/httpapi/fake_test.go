package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/zellapay/zellapay/internal/collections"
)

// fakeService records calls and returns canned results.
type fakeService struct {
	created collections.CreateInput
	listed  collections.ListQuery

	collection collections.Collection
	page       collections.Page
	err        error
}

func (f *fakeService) Create(
	_ context.Context, in collections.CreateInput,
) (collections.Collection, error) {
	f.created = in
	return f.collection, f.err
}

func (f *fakeService) Get(context.Context, string) (collections.Collection, error) {
	return f.collection, f.err
}

func (f *fakeService) List(_ context.Context, q collections.ListQuery) (collections.Page, error) {
	f.listed = q
	return f.page, f.err
}

var created = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

func sampleCollection() collections.Collection {
	return collections.Collection{
		ID: "col_1", MerchantID: "mer_1", Status: collections.StatusPending,
		Amount: 10000, Currency: "XAF", Fee: 150, PhoneMasked: "+241 07 ** ** 56",
		MerchantReference: "ORDER-1", Operator: "simulator", CreatedAt: created,
	}
}

func newHandler(f *fakeService, merchant string) *Handler {
	who := func(*http.Request) (string, bool) { return merchant, merchant != "" }
	return New(f, who, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// call describes a request to a handler.
type call struct {
	method, target, body string
	header               map[string]string
}

func do(h http.HandlerFunc, c call) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(
		context.Background(), c.method, c.target, strings.NewReader(c.body))
	for k, v := range c.header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func get(h http.HandlerFunc, target string) *httptest.ResponseRecorder {
	return do(h, call{method: "GET", target: target})
}

func post(h http.HandlerFunc, target, body string) *httptest.ResponseRecorder {
	return do(h, call{method: "POST", target: target, body: body})
}
