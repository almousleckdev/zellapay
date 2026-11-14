// Package apidocs serves the OpenAPI contract and Swagger UI.
package apidocs

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/swaggest/swgui/v5emb"

	"github.com/zellapay/zellapay/api"
)

const (
	specPrefix = "/openapi/"
	docsPrefix = "/docs/"
)

// Register mounts Swagger UI at /docs/ and the spec files at /openapi/.
// Both are read-only and served from the embedded filesystem.
func Register(mux *http.ServeMux) {
	mux.Handle("GET "+docsPrefix, v5emb.New("ZellaPay API", specPrefix+"openapi.yaml", docsPrefix))
	mux.Handle("GET "+specPrefix, specHandler(api.FS))
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, docsPrefix, http.StatusMovedPermanently)
	})
}

// specHandler serves only the YAML files of the contract, from an
// allowlist read once at startup. Nothing in a response is derived from the
// request, directories are never listed, and unknown paths are 404.
func specHandler(files fs.FS) http.Handler {
	specs := loadSpecs(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := specs[strings.TrimPrefix(r.URL.Path, specPrefix)]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(data)
	})
}

func loadSpecs(files fs.FS) map[string][]byte {
	specs := map[string][]byte{}
	_ = fs.WalkDir(files, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(name, ".yaml") {
			return err
		}
		data, readErr := fs.ReadFile(files, name)
		if readErr == nil {
			specs[name] = data
		}
		return readErr
	})
	return specs
}
