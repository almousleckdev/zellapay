package api_test

import (
	"strings"
	"testing"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// sessionSpec describes a document whose routes are called by an app's own
// server with a session token: the dashboard and the admin console.
type sessionSpec struct {
	file  string
	paths int
	// public lists the operations that need no session.
	public map[string]bool
}

var sessionSpecs = []sessionSpec{
	{file: "dashboard.openapi.yaml", paths: 17, public: map[string]bool{
		"post /dashboard/invitations/accept":     true,
		"post /dashboard/email/verify":           true,
		"post /dashboard/password/reset-request": true,
		"post /dashboard/password/reset":         true,
		"post /dashboard/session":                true,
	}},
	{file: "admin.openapi.yaml", paths: 10, public: map[string]bool{
		"post /admin/session": true,
	}},
}

type route struct {
	name string
	op   *v3.Operation
}

func routesOf(t *testing.T, file string) []route {
	t.Helper()
	var out []route
	paths := loadFile(t, file).Paths.PathItems
	for path, item := range paths.FromOldest() {
		for method, op := range item.GetOperations().FromOldest() {
			out = append(out, route{name: method + " " + path, op: op})
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s has no operations", file)
	}
	return out
}

func eachSpec(t *testing.T, check func(t *testing.T, s sessionSpec, routes []route)) {
	t.Helper()
	for _, s := range sessionSpecs {
		t.Run(s.file, func(t *testing.T) { check(t, s, routesOf(t, s.file)) })
	}
}

func TestSessionSpecsBuildWithAllReferencesResolved(t *testing.T) {
	for _, s := range sessionSpecs {
		if n := loadFile(t, s.file).Paths.PathItems.Len(); n != s.paths {
			t.Errorf("%s: paths = %d, want %d", s.file, n, s.paths)
		}
	}
}

func TestSessionSpecOperationsAreDocumentedAndUnique(t *testing.T) {
	eachSpec(t, func(t *testing.T, _ sessionSpec, routes []route) {
		seen := map[string]string{}
		for _, r := range routes {
			if r.op.OperationId == "" || len(r.op.Tags) == 0 || r.op.Summary == "" {
				t.Errorf("%s: needs operationId, tags and summary", r.name)
			}
			if prev, dup := seen[r.op.OperationId]; dup {
				t.Errorf("%s duplicates operationId of %s", r.name, prev)
			}
			seen[r.op.OperationId] = r.name
		}
	})
}

func isSecured(r route) bool {
	return len(r.op.Security) > 0 && r.op.Security[0].Requirements.Len() > 0
}

func TestSessionSpecRoutesDeclareAuthAndRefusal(t *testing.T) {
	eachSpec(t, func(t *testing.T, s sessionSpec, routes []route) {
		for _, r := range routes {
			if s.public[r.name] == isSecured(r) {
				t.Errorf("%s: public = %v but secured = %v", r.name, s.public[r.name], isSecured(r))
			}
			if !s.public[r.name] && r.op.Responses.Codes.GetOrZero("401") == nil {
				t.Errorf("%s: missing 401 response", r.name)
			}
		}
	})
}

func TestSessionSpecsNeverAskForAnIdempotencyKey(t *testing.T) {
	eachSpec(t, func(t *testing.T, _ sessionSpec, routes []route) {
		for _, r := range routes {
			for _, p := range r.op.Parameters {
				if strings.EqualFold(p.Name, "Idempotency-Key") {
					t.Errorf("%s: responses with secrets must not be cached for replay", r.name)
				}
			}
		}
	})
}

func TestSessionSpecErrorsUseTheSharedEnvelope(t *testing.T) {
	eachSpec(t, func(t *testing.T, _ sessionSpec, routes []route) {
		for _, r := range routes {
			for code, resp := range r.op.Responses.Codes.FromOldest() {
				if code[0] != '4' {
					continue
				}
				media := resp.Content.GetOrZero("application/json")
				if media == nil || media.Schema == nil {
					t.Errorf("%s %s: error without the shared envelope", r.name, code)
				}
			}
		}
	})
}
