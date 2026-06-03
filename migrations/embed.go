// Package migrations embeds the SQL migration files so the binary carries
// them at compile time without requiring filesystem access at runtime.
// Reference: ADR-007 (migrations via //go:embed).
package migrations

import "embed"

// FS is the embedded filesystem containing all *.sql migration files.
// Imported by internal/platform/database to run migrations via golang-migrate.
//
//go:embed *.sql
var FS embed.FS
