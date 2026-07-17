package migrations

import "embed"

// FS contains the SQL migrations used by the application migrator.
//
//go:embed *.sql
var FS embed.FS
