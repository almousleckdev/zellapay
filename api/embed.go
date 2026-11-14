// Package api embeds the OpenAPI contract so the binary can serve it.
package api

import "embed"

// FS holds openapi.yaml and every file it references.
//
//go:embed openapi.yaml paths components
var FS embed.FS
