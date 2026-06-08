//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

type ReconciliationCheckpointRepositorySuite struct {
	suite.Suite
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
}

func TestReconciliationCheckpointRepositorySuite(t *testing.T) {
	suite.Run(t, new(ReconciliationCheckpointRepositorySuite))
}

func (s *ReconciliationCheckpointRepositorySuite) SetupTest() {}

func (s *ReconciliationCheckpointRepositorySuite) SetupSuite() {
	s.mgr = setupTestDB(s.T())
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *ReconciliationCheckpointRepositorySuite) newRepo() interfaces.ReconciliationCheckpointRepository {
	return s.factory.ReconciliationCheckpointRepository(s.mgr.DBTX(context.Background()))
}

func (s *ReconciliationCheckpointRepositorySuite) TestGet() {
	scenarios := []struct {
		name      string
		setup     func(context.Context, interfaces.ReconciliationCheckpointRepository) (string, time.Time)
		expectErr error
	}{
		{
			name: "deve retornar erro quando o checkpoint nao existir",
			setup: func(ctx context.Context, repo interfaces.ReconciliationCheckpointRepository) (string, time.Time) {
				return "nonexistent_checkpoint", time.Time{}
			},
			expectErr: application.ErrCheckpointNotFound,
		},
		{
			name: "deve retornar o checkpoint salvo",
			setup: func(ctx context.Context, repo interfaces.ReconciliationCheckpointRepository) (string, time.Time) {
				watermark := time.Now().UTC().Truncate(time.Millisecond)
				name := "kiwify_sales_test"
				s.Require().NoError(repo.Set(ctx, name, watermark))
				return name, watermark
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			repo := s.newRepo()
			name, watermark := scenario.setup(ctx, repo)

			got, err := repo.Get(ctx, name)

			if scenario.expectErr != nil {
				s.Require().Error(err)
				s.Assert().True(errors.Is(err, scenario.expectErr))
				return
			}

			s.Require().NoError(err)
			s.Assert().WithinDuration(watermark, got, time.Second)
		})
	}
}

func (s *ReconciliationCheckpointRepositorySuite) TestSet() {
	scenarios := []struct {
		name   string
		setup  func() (string, time.Time, time.Time)
		expect func(context.Context, interfaces.ReconciliationCheckpointRepository, string, time.Time, time.Time)
	}{
		{
			name: "deve fazer upsert do checkpoint",
			setup: func() (string, time.Time, time.Time) {
				first := time.Now().UTC().Truncate(time.Millisecond)
				return "kiwify_sales_upsert", first, first.Add(time.Hour)
			},
			expect: func(ctx context.Context, repo interfaces.ReconciliationCheckpointRepository, name string, first time.Time, second time.Time) {
				s.Require().NoError(repo.Set(ctx, name, first))
				s.Require().NoError(repo.Set(ctx, name, second))
				got, err := repo.Get(ctx, name)
				s.Require().NoError(err)
				s.Assert().WithinDuration(second, got, time.Second)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			repo := s.newRepo()
			name, first, second := scenario.setup()
			scenario.expect(ctx, repo, name, first, second)
		})
	}
}
