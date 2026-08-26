// Package migrations embeds the versioned SQL migration files so the binary can run
// them on startup without needing the source tree present at runtime (important for
// the Docker image, which only ships the compiled binary).
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
