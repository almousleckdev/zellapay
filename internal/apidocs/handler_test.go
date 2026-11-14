package apidocs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	Register(mux)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestServesSpecFiles(t *testing.T) {
	for _, p := range []string{
		"/openapi/openapi.yaml",
		"/openapi/paths/collections.yaml",
		"/openapi/components/schemas/common.yaml",
	} {
		rec := get(t, p)
		ct := rec.Header().Get("Content-Type")
		if rec.Code != http.StatusOK || !strings.Contains(ct, "yaml") {
			t.Errorf("%s: got %d %q", p, rec.Code, ct)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: missing nosniff", p)
		}
	}
}

func TestDoesNotListDirectoriesOrServeOtherFiles(t *testing.T) {
	for _, p := range []string{
		"/openapi/", "/openapi/paths/", "/openapi/paths", "/openapi/embed.go",
		"/openapi/../go.mod", "/openapi/nope.yaml", "/openapi/%2e%2e/go.mod",
	} {
		rec := get(t, p)
		redirected := rec.Code == http.StatusMovedPermanently ||
			rec.Code == http.StatusTemporaryRedirect
		if redirected && strings.HasPrefix(rec.Header().Get("Location"), "/openapi/") {
			t.Errorf("%s: redirects inside the spec tree", p)
			continue
		}
		if !redirected && rec.Code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404", p, rec.Code)
		}
	}
}

func TestSwaggerUIIsServed(t *testing.T) {
	rec := get(t, "/docs/")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "swagger") {
		t.Fatalf("got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "unpkg.com") || strings.Contains(body, "cdn.") {
		t.Fatal("swagger UI must not load assets from a CDN")
	}
}
