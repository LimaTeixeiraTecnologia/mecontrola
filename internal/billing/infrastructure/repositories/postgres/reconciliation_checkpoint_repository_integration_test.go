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
	billingpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories/postgres"

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

func (s *ReconciliationCheckpointRepositorySuite) SetupSuite() {
	s.mgr = setupTestDB(s.T())
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *ReconciliationCheckpointRepositorySuite) newRepo() interfaces.ReconciliationCheckpointRepository {
	return s.factory.ReconciliationCheckpointRepository(s.mgr.DBTX(context.Background()))
}

func (s *ReconciliationCheckpointRepositorySuite) TestGet_NotFound() {
	ctx := context.Background()
	repo := s.newRepo()

	_, err := repo.Get(ctx, "nonexistent_checkpoint")
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, billingpostgres.ErrCheckpointNotFound))
}

func (s *ReconciliationCheckpointRepositorySuite) TestSet_ThenGet() {
	ctx := context.Background()
	repo := s.newRepo()

	watermark := time.Now().UTC().Truncate(time.Millisecond)
	name := "kiwify_sales_test"

	err := repo.Set(ctx, name, watermark)
	s.Require().NoError(err)

	got, err := repo.Get(ctx, name)
	s.Require().NoError(err)
	s.Assert().WithinDuration(watermark, got, time.Second)
}

func (s *ReconciliationCheckpointRepositorySuite) TestSet_Upsert() {
	ctx := context.Background()
	repo := s.newRepo()

	name := "kiwify_sales_upsert"
	first := time.Now().UTC().Truncate(time.Millisecond)
	s.Require().NoError(repo.Set(ctx, name, first))

	second := first.Add(time.Hour)
	s.Require().NoError(repo.Set(ctx, name, second))

	got, err := repo.Get(ctx, name)
	s.Require().NoError(err)
	s.Assert().WithinDuration(second, got, time.Second)
}
