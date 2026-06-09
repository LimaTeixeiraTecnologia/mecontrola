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
		expect func(manager.Manager)
	}{
		{
			name:  "deve provisionar banco de teste",
			setup: func() {},
			expect: func(mgr manager.Manager) {
				s.NotNil(mgr)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()

			mgr := setupTestDB(s.T())
			scenario.expect(mgr)
		})
	}
}

func setupTestDB(t *testing.T) manager.Manager {
	t.Helper()
	mgr, _ := testcontainer.Postgres(t)
	return mgr
}
