package api_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

func loadModel(t *testing.T) *v3.Document {
	t.Helper()
	abs, err := filepath.Abs("openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := libopenapi.NewDocumentWithConfiguration(raw, &datamodel.DocumentConfiguration{
		BasePath:            filepath.Dir(abs),
		SpecFilePath:        abs,
		AllowFileReferences: true,
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		t.Fatalf("build model: %v", err)
	}
	return &model.Model
}

func TestSpecBuildsWithAllReferencesResolved(t *testing.T) {
	m := loadModel(t)
	if m.Paths == nil || m.Paths.PathItems.Len() == 0 {
		t.Fatal("no paths resolved")
	}
	if m.Webhooks == nil || m.Webhooks.Len() != 5 {
		t.Fatalf("want 5 webhooks, got %v", m.Webhooks)
	}
}
