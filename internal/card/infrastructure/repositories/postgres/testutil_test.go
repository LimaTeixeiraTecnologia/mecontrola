//go:build integration

package postgres_test

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type TestutilSuite struct {
	suite.Suite
}

func TestTestutilSuite(t *testing.T) {
	suite.Run(t, new(TestutilSuite))
}

func (s *TestutilSuite) SetupTest() {}

func (s *TestutilSuite) TestSetupTestDB() {
	scenarios := []struct {
		name   string
		setup  func()
		expect func(*sqlx.DB)
	}{
		{
			name:  "deve provisionar banco de teste",
			setup: func() {},
			expect: func(db *sqlx.DB) {
				s.NotNil(db)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()
			db := setupTestDB(s.T())
			scenario.expect(db)
		})
	}
}

func setupTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, _ := testcontainer.Postgres(t)
	return db
}
