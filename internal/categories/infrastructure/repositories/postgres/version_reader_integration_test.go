//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/repositories/postgres"
)

type VersionReaderIntegrationSuite struct {
	suite.Suite
	mgr    manager.Manager
	reader interfaces.VersionReader
}

func TestVersionReaderIntegrationSuite(t *testing.T) {
	suite.Run(t, new(VersionReaderIntegrationSuite))
}

func (s *VersionReaderIntegrationSuite) SetupSuite() {
	s.mgr = setupTestDB(s.T())
	s.reader = postgres.NewVersionReader(noop.NewProvider(), s.mgr.DBTX(context.Background()))
}

func (s *VersionReaderIntegrationSuite) SetupTest() {}

func (s *VersionReaderIntegrationSuite) TestCurrent() {
	scenarios := []struct {
		name             string
		expectVersion    int64
		expectMinVersion int64
		expectErr        error
	}{
		{
			name:             "deve retornar versao atual",
			expectMinVersion: 1,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()

			version, err := s.reader.Current(ctx)

			if scenario.expectErr != nil {
				s.Require().Error(err)
				s.Assert().True(errors.Is(err, scenario.expectErr))
				return
			}

			s.Require().NoError(err)
			s.Assert().GreaterOrEqual(version, scenario.expectMinVersion)
		})
	}
}

func (s *VersionReaderIntegrationSuite) TestCurrentConsistency() {
	ctx := context.Background()

	version1, err := s.reader.Current(ctx)
	s.Require().NoError(err)

	version2, err := s.reader.Current(ctx)
	s.Require().NoError(err)

	s.Assert().Equal(version1, version2, "versao deve ser consistente entre chamadas")
}
