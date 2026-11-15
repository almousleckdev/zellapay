package apiserver_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
)

// dashboardOperations reads method and path pairs from the dashboard contract.
func dashboardOperations(t *testing.T) [][2]string {
	t.Helper()
	abs, err := filepath.Abs("../../api/dashboard.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := fs.ReadFile(os.DirFS(filepath.Dir(abs)), filepath.Base(abs))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := libopenapi.NewDocumentWithConfiguration(raw, &datamodel.DocumentConfiguration{
		BasePath: filepath.Dir(abs), SpecFilePath: abs, AllowFileReferences: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		t.Fatal(err)
	}
	var ops [][2]string
	for path, item := range model.Model.Paths.PathItems.FromOldest() {
		for method := range item.GetOperations().FromOldest() {
			ops = append(ops, [2]string{strings.ToUpper(method), path})
		}
	}
	return ops
}

func TestEveryDocumentedDashboardOperationIsServed(t *testing.T) {
	e := newEnv(t)
	ops := dashboardOperations(t)
	if len(ops) != 16 {
		t.Fatalf("contract has %d operations, the server mounts 16", len(ops))
	}
	for _, op := range ops {
		path := strings.NewReplacer("{session_id}", "x").Replace(op[1])
		r := e.dash(op[0], path, "", "{}")
		if r.status == 404 && r.errorCode(t) == "" || r.status == 405 {
			t.Errorf("%s %s is documented but not served (%d)", op[0], op[1], r.status)
		}
	}
}
