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

func (s *PlanRepositorySuite) SetupTest() {}

func (s *PlanRepositorySuite) SetupSuite() {
	s.mgr = setupTestDB(s.T())
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *PlanRepositorySuite) newRepo() interfaces.PlanRepository {
	return s.factory.PlanRepository(s.mgr.DBTX(context.Background()))
}

func (s *PlanRepositorySuite) TestFindByCode() {
	scenarios := []struct {
		name       string
		code       valueobjects.PlanCode
		expectCode valueobjects.PlanCode
		expectDays int
		expectErr  error
	}{
		{name: "deve encontrar plano mensal", code: valueobjects.PlanCodeMonthly, expectCode: valueobjects.PlanCodeMonthly, expectDays: 30},
		{name: "deve encontrar plano trimestral", code: valueobjects.PlanCodeQuarterly, expectCode: valueobjects.PlanCodeQuarterly, expectDays: 90},
		{name: "deve encontrar plano anual", code: valueobjects.PlanCodeAnnual, expectCode: valueobjects.PlanCodeAnnual, expectDays: 365},
		{name: "deve retornar erro quando o plano nao existir", code: "NONEXISTENT", expectErr: billingpostgres.ErrPlanNotFound},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			repo := s.newRepo()

			plan, err := repo.FindByCode(ctx, scenario.code)

			if scenario.expectErr != nil {
				s.Require().Error(err)
				s.Assert().True(errors.Is(err, scenario.expectErr))
				return
			}

			s.Require().NoError(err)
			s.Assert().Equal(scenario.expectCode, plan.Code())
			s.Assert().Equal(scenario.expectDays, plan.DurationDays())
		})
	}
}

func (s *PlanRepositorySuite) TestFindByKiwifyProductID() {
	scenarios := []struct {
		name       string
		productID  string
		expectCode valueobjects.PlanCode
		expectErr  error
	}{
		{name: "deve encontrar plano pelo produto mensal", productID: "<id-mensal>", expectCode: valueobjects.PlanCodeMonthly},
		{name: "deve retornar erro quando o produto nao existir", productID: "unknown-product-id", expectErr: billingpostgres.ErrPlanNotFound},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			repo := s.newRepo()

			plan, err := repo.FindByKiwifyProductID(ctx, scenario.productID)

			if scenario.expectErr != nil {
				s.Require().Error(err)
				s.Assert().True(errors.Is(err, scenario.expectErr))
				return
			}

			s.Require().NoError(err)
			s.Assert().Equal(scenario.expectCode, plan.Code())
		})
	}
}

func (s *PlanRepositorySuite) TestConfigureProductIDs() {
	scenarios := []struct {
		name   string
		setup  func(context.Context, interfaces.PlanRepository)
		expect func(context.Context, interfaces.PlanRepository)
	}{
		{
			name: "deve reconfigurar os ids de produto",
			setup: func(ctx context.Context, repo interfaces.PlanRepository) {
				err := repo.ConfigureProductIDs(ctx, map[valueobjects.PlanCode]string{
					valueobjects.PlanCodeMonthly:   "real-monthly",
					valueobjects.PlanCodeQuarterly: "real-quarterly",
					valueobjects.PlanCodeAnnual:    "real-annual",
				})
				s.Require().NoError(err)
			},
			expect: func(ctx context.Context, repo interfaces.PlanRepository) {
				plan, err := repo.FindByKiwifyProductID(ctx, "real-monthly")
				s.Require().NoError(err)
				s.Equal(valueobjects.PlanCodeMonthly, plan.Code())
				s.Require().NoError(repo.ConfigureProductIDs(ctx, map[valueobjects.PlanCode]string{
					valueobjects.PlanCodeMonthly:   "<id-mensal>",
					valueobjects.PlanCodeQuarterly: "<id-trimestral>",
					valueobjects.PlanCodeAnnual:    "<id-anual>",
				}))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			repo := s.newRepo()
			scenario.setup(ctx, repo)
			scenario.expect(ctx, repo)
		})
	}
}
