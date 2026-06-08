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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/jobs/handlers"
)

type HousekeepingJobSuite struct {
	suite.Suite
	factoryMock *ucmocks.RepositoryFactory
	eventRepo   *ucmocks.KiwifyEventRepository
}

func TestHousekeepingJob(t *testing.T) {
	suite.Run(t, new(HousekeepingJobSuite))
}

func (s *HousekeepingJobSuite) SetupTest() {
	s.factoryMock = ucmocks.NewRepositoryFactory(s.T())
	s.eventRepo = ucmocks.NewKiwifyEventRepository(s.T())
}

func (s *HousekeepingJobSuite) newJob(cfg configs.BillingConfig) *handlers.KiwifyEventsHousekeepingJob {
	cleanup := usecases.NewCleanupKiwifyEvents(nil, s.factoryMock, cfg, noop.NewProvider())
	return handlers.NewKiwifyEventsHousekeepingJob(cleanup, cfg)
}

func (s *HousekeepingJobSuite) TestMetadata() {
	scenarios := []struct {
		name             string
		cfg              configs.BillingConfig
		expectedName     string
		expectedSchedule string
	}{
		{
			name:             "deve expor nome e schedule configurado",
			cfg:              configs.BillingConfig{KiwifyEventsRetentionDays: 90, KiwifyEventsHousekeepingSchedule: "@daily", KiwifyEventsHousekeepingBatch: 100},
			expectedName:     "billing-kiwify-events-housekeeping",
			expectedSchedule: "@daily",
		},
		{
			name:             "deve usar schedule padrao quando vazio",
			cfg:              configs.BillingConfig{KiwifyEventsRetentionDays: 90},
			expectedName:     "billing-kiwify-events-housekeeping",
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

func (s *HousekeepingJobSuite) TestRun() {
	scenarios := []struct {
		name      string
		cfg       configs.BillingConfig
		setup     func(context.Context)
		expectErr string
	}{
		{
			name: "deve excluir linhas antigas em lotes",
			cfg:  configs.BillingConfig{KiwifyEventsRetentionDays: 90, KiwifyEventsHousekeepingSchedule: "@daily", KiwifyEventsHousekeepingBatch: 100},
			setup: func(ctx context.Context) {
				s.factoryMock.EXPECT().KiwifyEventRepository(nil).Return(s.eventRepo).Once()
				s.eventRepo.EXPECT().DeleteOlderThan(ctx, mock.Anything, 100).Return(int64(100), nil).Once()
				s.eventRepo.EXPECT().DeleteOlderThan(ctx, mock.Anything, 100).Return(int64(50), nil).Once()
				s.eventRepo.EXPECT().DeleteOlderThan(ctx, mock.Anything, 100).Return(int64(0), nil).Once()
			},
		},
		{
			name: "deve concluir quando nao houver linhas para excluir",
			cfg:  configs.BillingConfig{KiwifyEventsRetentionDays: 90, KiwifyEventsHousekeepingBatch: 100},
			setup: func(ctx context.Context) {
				s.factoryMock.EXPECT().KiwifyEventRepository(nil).Return(s.eventRepo).Once()
				s.eventRepo.EXPECT().DeleteOlderThan(ctx, mock.Anything, 100).Return(int64(0), nil).Once()
			},
		},
		{
			name: "deve respeitar a janela de retencao",
			cfg:  configs.BillingConfig{KiwifyEventsRetentionDays: 90, KiwifyEventsHousekeepingBatch: 100},
			setup: func(ctx context.Context) {
				before90d := time.Now().UTC().Add(-90 * 24 * time.Hour)
				s.factoryMock.EXPECT().KiwifyEventRepository(nil).Return(s.eventRepo).Once()
				s.eventRepo.EXPECT().
					DeleteOlderThan(ctx, mock.MatchedBy(func(before time.Time) bool {
						diff := before90d.Sub(before)
						if diff < 0 {
							diff = -diff
						}
						return diff < 5*time.Second
					}), 100).
					Return(int64(0), nil).
					Once()
			},
		},
		{
			name: "deve propagar erro do repositorio",
			cfg:  configs.BillingConfig{KiwifyEventsRetentionDays: 90, KiwifyEventsHousekeepingBatch: 100},
			setup: func(ctx context.Context) {
				s.factoryMock.EXPECT().KiwifyEventRepository(nil).Return(s.eventRepo).Once()
				s.eventRepo.EXPECT().DeleteOlderThan(ctx, mock.Anything, 100).Return(int64(0), errors.New("db error")).Once()
			},
			expectErr: "db error",
		},
		{
			name: "deve parar quando o contexto for cancelado",
			cfg:  configs.BillingConfig{KiwifyEventsRetentionDays: 90, KiwifyEventsHousekeepingBatch: 100},
			setup: func(ctx context.Context) {
				cancelableCtx, ok := ctx.(interface{ Done() <-chan struct{} })
				s.Require().True(ok)
				_ = cancelableCtx
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
				s.factoryMock.EXPECT().KiwifyEventRepository(nil).Return(s.eventRepo).Once()
				s.eventRepo.EXPECT().
					DeleteOlderThan(mock.Anything, mock.Anything, 100).
					RunAndReturn(func(innerCtx context.Context, before time.Time, limit int) (int64, error) {
						cancel()
						return 50, nil
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
