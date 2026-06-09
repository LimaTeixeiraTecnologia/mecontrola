//go:build integration

package postgres_test

import (
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
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
		expect func(manager.Manager, string)
	}{
		{
			name:  "deve provisionar banco de teste com dsn",
			setup: func() {},
			expect: func(mgr manager.Manager, dsn string) {
				s.NotNil(mgr)
				s.NotEmpty(dsn)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			scenario.setup()

			mgr, dsn := setupTestDB(s.T())
			scenario.expect(mgr, dsn)
		})
	}
}

func setupTestDB(t *testing.T) (manager.Manager, string) {
	t.Helper()
	return testcontainer.Postgres(t)
}
