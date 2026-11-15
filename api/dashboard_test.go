package api_test

import (
	"strings"
	"testing"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

const dashboardSpec = "dashboard.openapi.yaml"

// public routes need no session: they are reached by a link or a password.
var publicRoutes = map[string]bool{
	"post /dashboard/invitations/accept":     true,
	"post /dashboard/email/verify":           true,
	"post /dashboard/password/reset-request": true,
	"post /dashboard/password/reset":         true,
	"post /dashboard/session":                true,
}

type route struct {
	name string
	op   *v3.Operation
}

func dashboardRoutes(t *testing.T) []route {
	t.Helper()
	var out []route
	paths := loadFile(t, dashboardSpec).Paths.PathItems
	for path, item := range paths.FromOldest() {
		for method, op := range item.GetOperations().FromOldest() {
			out = append(out, route{name: method + " " + path, op: op})
		}
	}
	if len(out) == 0 {
		t.Fatal("no dashboard operations")
	}
	return out
}

func TestDashboardSpecBuildsWithAllReferencesResolved(t *testing.T) {
	if n := loadFile(t, dashboardSpec).Paths.PathItems.Len(); n != 14 {
		t.Fatalf("paths = %d, want 14", n)
	}
}

func TestDashboardOperationsAreDocumentedAndUnique(t *testing.T) {
	seen := map[string]string{}
	for _, r := range dashboardRoutes(t) {
		if r.op.OperationId == "" || len(r.op.Tags) == 0 || r.op.Summary == "" {
			t.Errorf("%s: needs operationId, tags and summary", r.name)
		}
		if prev, dup := seen[r.op.OperationId]; dup {
			t.Errorf("%s duplicates operationId of %s", r.name, prev)
		}
		seen[r.op.OperationId] = r.name
	}
}

func TestDashboardSessionRoutesDeclareAuthAndRefusal(t *testing.T) {
	for _, r := range dashboardRoutes(t) {
		secured := len(r.op.Security) > 0 && r.op.Security[0].Requirements.Len() > 0
		if publicRoutes[r.name] {
			if secured {
				t.Errorf("%s is public but declares a session", r.name)
			}
			continue
		}
		if !secured {
			t.Errorf("%s needs sessionAuth", r.name)
		}
		if r.op.Responses.Codes.GetOrZero("401") == nil {
			t.Errorf("%s: missing 401 response", r.name)
		}
	}
}

func TestDashboardNeverAsksForAnIdempotencyKey(t *testing.T) {
	for _, r := range dashboardRoutes(t) {
		for _, p := range r.op.Parameters {
			if strings.EqualFold(p.Name, "Idempotency-Key") {
				t.Errorf("%s: responses with secrets must not be cached for replay", r.name)
			}
		}
	}
}

func TestDashboardErrorsUseTheSharedEnvelope(t *testing.T) {
	for _, r := range dashboardRoutes(t) {
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
}
