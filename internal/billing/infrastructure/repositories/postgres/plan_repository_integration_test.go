//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories"
	billingpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories/postgres"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type PlanRepositorySuite struct {
	suite.Suite
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
}

func TestPlanRepositorySuite(t *testing.T) {
	suite.Run(t, new(PlanRepositorySuite))
}

func (s *PlanRepositorySuite) SetupSuite() {
	s.mgr = setupTestDB(s.T())
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *PlanRepositorySuite) newRepo() interfaces.PlanRepository {
	return s.factory.PlanRepository(s.mgr.DBTX(context.Background()))
}

func (s *PlanRepositorySuite) TestFindByCode_Monthly() {
	ctx := context.Background()
	repo := s.newRepo()

	plan, err := repo.FindByCode(ctx, valueobjects.PlanCodeMonthly)
	s.Require().NoError(err)
	s.Assert().Equal(valueobjects.PlanCodeMonthly, plan.Code())
	s.Assert().Equal(30, plan.DurationDays())
}

func (s *PlanRepositorySuite) TestFindByCode_Quarterly() {
	ctx := context.Background()
	repo := s.newRepo()

	plan, err := repo.FindByCode(ctx, valueobjects.PlanCodeQuarterly)
	s.Require().NoError(err)
	s.Assert().Equal(valueobjects.PlanCodeQuarterly, plan.Code())
	s.Assert().Equal(90, plan.DurationDays())
}

func (s *PlanRepositorySuite) TestFindByCode_Annual() {
	ctx := context.Background()
	repo := s.newRepo()

	plan, err := repo.FindByCode(ctx, valueobjects.PlanCodeAnnual)
	s.Require().NoError(err)
	s.Assert().Equal(valueobjects.PlanCodeAnnual, plan.Code())
	s.Assert().Equal(365, plan.DurationDays())
}

func (s *PlanRepositorySuite) TestFindByCode_NotFound() {
	ctx := context.Background()
	repo := s.newRepo()

	_, err := repo.FindByCode(ctx, "NONEXISTENT")
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, billingpostgres.ErrPlanNotFound))
}

func (s *PlanRepositorySuite) TestFindByKiwifyProductID_Monthly() {
	ctx := context.Background()
	repo := s.newRepo()

	plan, err := repo.FindByKiwifyProductID(ctx, "<id-mensal>")
	s.Require().NoError(err)
	s.Assert().Equal(valueobjects.PlanCodeMonthly, plan.Code())
}

func (s *PlanRepositorySuite) TestFindByKiwifyProductID_NotFound() {
	ctx := context.Background()
	repo := s.newRepo()

	_, err := repo.FindByKiwifyProductID(ctx, "unknown-product-id")
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, billingpostgres.ErrPlanNotFound))
}
