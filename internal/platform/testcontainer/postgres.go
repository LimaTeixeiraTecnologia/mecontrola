//go:build integration || e2e

package testcontainer

import (
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
)

func Postgres(t *testing.T) (*sqlx.DB, string) {
	t.Helper()
	return postgres.NewTestDatabase(t)
}
