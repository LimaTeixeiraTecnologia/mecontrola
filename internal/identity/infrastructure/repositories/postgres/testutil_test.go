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
		expect func(*sqlx.DB, string)
	}{
		{
			name:  "deve provisionar banco de teste com dsn",
			setup: func() {},
			expect: func(db *sqlx.DB, dsn string) {
				s.NotNil(db)
				s.NotEmpty(dsn)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			scenario.setup()

			db, dsn := setupTestDB(s.T())
			scenario.expect(db, dsn)
		})
	}
}

func setupTestDB(t *testing.T) (*sqlx.DB, string) {
	t.Helper()
	return testcontainer.Postgres(t)
}
