package handlers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/jobs/handlers"
)

type GraceExpirationJobSuite struct {
	suite.Suite
	factoryMock   *ucmocks.RepositoryFactory
	uowMock       *ucmocks.UnitOfWorkSubscription
	publisherMock *ucmocks.SubscriptionEventPublisher
	subRepoMock   *ucmocks.SubscriptionRepository
}

func TestGraceExpirationJob(t *testing.T) {
	suite.Run(t, new(GraceExpirationJobSuite))
}

func (s *GraceExpirationJobSuite) SetupTest() {
	s.factoryMock = ucmocks.NewRepositoryFactory(s.T())
	s.uowMock = ucmocks.NewUnitOfWorkSubscription(s.T())
	s.publisherMock = ucmocks.NewSubscriptionEventPublisher(s.T())
	s.subRepoMock = ucmocks.NewSubscriptionRepository(s.T())
}

func (s *GraceExpirationJobSuite) newJob(cfg configs.BillingConfig) *handlers.GraceExpirationJob {
	uc := usecases.NewProcessSubscriptionGraceExpired(s.uowMock, nil, s.factoryMock, s.publisherMock, noop.NewProvider())
	return handlers.NewGraceExpirationJob(uc, cfg)
}

func (s *GraceExpirationJobSuite) TestMetadata() {
	scenarios := []struct {
		name             string
		cfg              configs.BillingConfig
		expectedName     string
		expectedSchedule string
	}{
		{
			name:             "deve expor nome e schedule configurado",
			cfg:              configs.BillingConfig{GraceExpirationSchedule: "@every 15m"},
			expectedName:     "billing-grace-expiration",
			expectedSchedule: "@every 15m",
		},
		{
			name:             "deve usar schedule padrao quando vazio",
			cfg:              configs.BillingConfig{},
			expectedName:     "billing-grace-expiration",
			expectedSchedule: "@every 30m",
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			job := s.newJob(scenario.cfg)
			s.Equal(scenario.expectedName, job.Name())
			s.Equal(scenario.expectedSchedule, job.Schedule())
		})
	}
}

func (s *GraceExpirationJobSuite) TestRun() {
	scenarios := []struct {
		name      string
		setup     func(ctx context.Context)
		expectErr string
	}{
		{
			name: "execucao normal sem candidatos retorna nil",
			setup: func(ctx context.Context) {
				s.factoryMock.EXPECT().SubscriptionRepository(nil).Return(s.subRepoMock).Once()
				s.subRepoMock.EXPECT().ListPastDueGraceExpired(ctx, mock.Anything, 100).Return(nil, nil).Once()
			},
		},
		{
			name: "use case retorna erro propaga o erro",
			setup: func(ctx context.Context) {
				s.factoryMock.EXPECT().SubscriptionRepository(nil).Return(s.subRepoMock).Once()
				s.subRepoMock.EXPECT().ListPastDueGraceExpired(ctx, mock.Anything, 100).
					Return(nil, errors.New("db unavailable")).Once()
			},
			expectErr: "db unavailable",
		},
		{
			name: "contexto cancelado e propagado pelo use case",
			setup: func(ctx context.Context) {
				s.factoryMock.EXPECT().SubscriptionRepository(nil).Return(s.subRepoMock).Once()
				s.subRepoMock.EXPECT().ListPastDueGraceExpired(mock.Anything, mock.Anything, 100).
					Return(nil, ctx.Err()).Once()
			},
			expectErr: "context canceled",
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			ctx := context.Background()
			if scenario.expectErr == "context canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(context.Background())
				cancel()
			}
			scenario.setup(ctx)
			job := s.newJob(configs.BillingConfig{GraceExpirationSchedule: "@every 30m"})
			err := job.Run(ctx)

			if scenario.expectErr == "" {
				s.Require().NoError(err)
				return
			}
			s.Require().Error(err)
			s.Contains(err.Error(), scenario.expectErr)
		})
	}
}
