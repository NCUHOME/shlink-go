package migrations

import "embed"

// FS contains all SQL migrations. Keeping them embedded makes the binary
// self-contained while still using bun/migrate for version tracking.
//
//go:embed *.sql
var FS embed.FS
