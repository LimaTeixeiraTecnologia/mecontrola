// Package migrations embeds the SQL migration files so the binary carries
// them at compile time without requiring filesystem access at runtime.
// Reference: ADR-007 (migrations via //go:embed).
package migrations

import "embed"

// FS is the embedded filesystem containing the single V0 baseline.
// Imported by internal/platform/database to run migrations via golang-migrate.
//
//go:embed 000001_initial_baseline.up.sql 000001_initial_baseline.down.sql
var FS embed.FS
