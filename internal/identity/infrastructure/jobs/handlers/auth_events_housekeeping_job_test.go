package handlers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	usecasemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/jobs/handlers"
)

type AuthEventsHousekeepingJobSuite struct {
	suite.Suite
	factoryMock *imocks.RepositoryFactory
	repoMock    *imocks.AuthEventsRepository
}

func TestAuthEventsHousekeepingJobSuite(t *testing.T) {
	suite.Run(t, new(AuthEventsHousekeepingJobSuite))
}

func (s *AuthEventsHousekeepingJobSuite) SetupTest() {
	s.factoryMock = imocks.NewRepositoryFactory(s.T())
	s.repoMock = imocks.NewAuthEventsRepository(s.T())
}

func (s *AuthEventsHousekeepingJobSuite) newJob(cfg configs.IdentityConfig) *handlers.AuthEventsHousekeepingJob {
	cleanup := usecases.NewCleanupAuthEvents(usecasemocks.NewFakeManager(), s.factoryMock, cfg, noop.NewProvider())
	return handlers.NewAuthEventsHousekeepingJob(cleanup, cfg)
}

func (s *AuthEventsHousekeepingJobSuite) TestMetadata() {
	scenarios := []struct {
		name             string
		cfg              configs.IdentityConfig
		expectedName     string
		expectedSchedule string
	}{
		{
			name:             "deve expor nome e schedule configurado",
			cfg:              configs.IdentityConfig{AuthEventsRetentionDays: 180, AuthEventsHousekeepingSchedule: "@monthly", AuthEventsHousekeepingBatch: 10000},
			expectedName:     "identity-auth-events-housekeeping",
			expectedSchedule: "@monthly",
		},
		{
			name:             "deve usar schedule padrao quando vazio",
			cfg:              configs.IdentityConfig{AuthEventsRetentionDays: 180},
			expectedName:     "identity-auth-events-housekeeping",
			expectedSchedule: "@monthly",
		},
		{
			name:             "deve usar schedule personalizado",
			cfg:              configs.IdentityConfig{AuthEventsRetentionDays: 180, AuthEventsHousekeepingSchedule: "@daily"},
			expectedName:     "identity-auth-events-housekeeping",
			expectedSchedule: "@daily",
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

func (s *AuthEventsHousekeepingJobSuite) TestRun() {
	scenarios := []struct {
		name      string
		cfg       configs.IdentityConfig
		setup     func(context.Context)
		expectErr string
	}{
		{
			name: "deve excluir linhas antigas em lotes",
			cfg:  configs.IdentityConfig{AuthEventsRetentionDays: 180, AuthEventsHousekeepingSchedule: "@monthly", AuthEventsHousekeepingBatch: 10000},
			setup: func(ctx context.Context) {
				s.factoryMock.EXPECT().AuthEventsRepository(nil).Return(s.repoMock).Once()
				s.repoMock.EXPECT().DeleteOlderThan(ctx, mock.Anything, 10000).Return(int64(10000), nil).Once()
				s.repoMock.EXPECT().DeleteOlderThan(ctx, mock.Anything, 10000).Return(int64(5000), nil).Once()
				s.repoMock.EXPECT().DeleteOlderThan(ctx, mock.Anything, 10000).Return(int64(0), nil).Once()
			},
		},
		{
			name: "deve concluir quando nao houver linhas para excluir",
			cfg:  configs.IdentityConfig{AuthEventsRetentionDays: 180, AuthEventsHousekeepingBatch: 10000},
			setup: func(ctx context.Context) {
				s.factoryMock.EXPECT().AuthEventsRepository(nil).Return(s.repoMock).Once()
				s.repoMock.EXPECT().DeleteOlderThan(ctx, mock.Anything, 10000).Return(int64(0), nil).Once()
			},
		},
		{
			name: "deve propagar erro do repositorio",
			cfg:  configs.IdentityConfig{AuthEventsRetentionDays: 180, AuthEventsHousekeepingBatch: 10000},
			setup: func(ctx context.Context) {
				s.factoryMock.EXPECT().AuthEventsRepository(nil).Return(s.repoMock).Once()
				s.repoMock.EXPECT().DeleteOlderThan(ctx, mock.Anything, 10000).Return(int64(0), errors.New("db error")).Once()
			},
			expectErr: "db error",
		},
		{
			name: "deve parar quando o contexto for cancelado",
			cfg:  configs.IdentityConfig{AuthEventsRetentionDays: 180, AuthEventsHousekeepingBatch: 10000},
			setup: func(ctx context.Context) {
				_, ok := ctx.(interface{ Done() <-chan struct{} })
				s.Require().True(ok)
			},
			expectErr: "context cancelled",
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			if scenario.expectErr == "context cancelled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(context.Background())
				s.factoryMock.EXPECT().AuthEventsRepository(nil).Return(s.repoMock).Once()
				s.repoMock.EXPECT().
					DeleteOlderThan(mock.Anything, mock.Anything, 10000).
					RunAndReturn(func(innerCtx context.Context, _ time.Time, _ int) (int64, error) {
						cancel()
						return 5000, nil
					}).
					Once()
			} else {
				scenario.setup(ctx)
			}
			job := s.newJob(scenario.cfg)
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
