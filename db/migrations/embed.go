// Package migrations embeds the SQL migrations so binaries and tests can
// apply them without reading the filesystem.
package migrations

import "embed"

// FS holds every migration file.
//
//go:embed *.sql
var FS embed.FS
