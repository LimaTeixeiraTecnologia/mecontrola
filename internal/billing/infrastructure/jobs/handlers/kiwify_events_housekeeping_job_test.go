package handlers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/jobs/handlers"
)

type mockKiwifyEventRepo struct {
	mock.Mock
}

func (m *mockKiwifyEventRepo) Persist(ctx context.Context, envelopeID string, trigger string, rawBody []byte, signatureStatus string) error {
	return m.Called(ctx, envelopeID, trigger, rawBody, signatureStatus).Error(0)
}

func (m *mockKiwifyEventRepo) MarkProcessed(ctx context.Context, envelopeID string, processedAt time.Time) error {
	return m.Called(ctx, envelopeID, processedAt).Error(0)
}

func (m *mockKiwifyEventRepo) DeleteOlderThan(ctx context.Context, before time.Time, limit int) (int64, error) {
	args := m.Called(ctx, before, limit)
	return args.Get(0).(int64), args.Error(1)
}

type HousekeepingJobSuite struct {
	suite.Suite
	factoryMock *ucmocks.RepositoryFactory
	eventRepo   *mockKiwifyEventRepo
	job         *handlers.KiwifyEventsHousekeepingJob
}

func TestHousekeepingJob(t *testing.T) {
	suite.Run(t, new(HousekeepingJobSuite))
}

func (s *HousekeepingJobSuite) SetupTest() {
	s.factoryMock = ucmocks.NewRepositoryFactory(s.T())
	s.eventRepo = &mockKiwifyEventRepo{}

	cfg := configs.BillingConfig{
		KiwifyEventsRetentionDays:        90,
		KiwifyEventsHousekeepingSchedule: "@daily",
		KiwifyEventsHousekeepingBatch:    100,
	}
	s.job = handlers.NewKiwifyEventsHousekeepingJob(nil, s.factoryMock, cfg, noop.NewProvider())
}

func (s *HousekeepingJobSuite) TestName() {
	s.Equal("billing-kiwify-events-housekeeping", s.job.Name())
}

func (s *HousekeepingJobSuite) TestSchedule() {
	s.Equal("@daily", s.job.Schedule())
}

func (s *HousekeepingJobSuite) TestScheduleDefaultsToDaily() {
	cfg := configs.BillingConfig{KiwifyEventsRetentionDays: 90}
	job := handlers.NewKiwifyEventsHousekeepingJob(nil, s.factoryMock, cfg, noop.NewProvider())
	s.Equal("@daily", job.Schedule())
}

func (s *HousekeepingJobSuite) TestRunDeletesOldRowsInBatches() {
	ctx := context.Background()

	s.factoryMock.On("KiwifyEventRepository", mock.Anything).Return(s.eventRepo)

	s.eventRepo.On("DeleteOlderThan", mock.Anything, mock.Anything, 100).
		Return(int64(100), nil).Once()
	s.eventRepo.On("DeleteOlderThan", mock.Anything, mock.Anything, 100).
		Return(int64(50), nil).Once()
	s.eventRepo.On("DeleteOlderThan", mock.Anything, mock.Anything, 100).
		Return(int64(0), nil).Once()

	err := s.job.Run(ctx)
	s.Require().NoError(err)
	s.eventRepo.AssertNumberOfCalls(s.T(), "DeleteOlderThan", 3)
}

func (s *HousekeepingJobSuite) TestRunNoRowsToDelete() {
	ctx := context.Background()

	s.factoryMock.On("KiwifyEventRepository", mock.Anything).Return(s.eventRepo)
	s.eventRepo.On("DeleteOlderThan", mock.Anything, mock.Anything, 100).
		Return(int64(0), nil)

	err := s.job.Run(ctx)
	s.Require().NoError(err)
}

func (s *HousekeepingJobSuite) TestRunRespectsRetentionWindow() {
	ctx := context.Background()

	s.factoryMock.On("KiwifyEventRepository", mock.Anything).Return(s.eventRepo)

	before90d := time.Now().UTC().Add(-90 * 24 * time.Hour)

	s.eventRepo.On("DeleteOlderThan", mock.Anything,
		mock.MatchedBy(func(before time.Time) bool {
			diff := before90d.Sub(before)
			if diff < 0 {
				diff = -diff
			}
			return diff < 5*time.Second
		}),
		100).Return(int64(0), nil)

	err := s.job.Run(ctx)
	s.Require().NoError(err)
}

func (s *HousekeepingJobSuite) TestRunPropagatesDeleteError() {
	ctx := context.Background()

	s.factoryMock.On("KiwifyEventRepository", mock.Anything).Return(s.eventRepo)
	s.eventRepo.On("DeleteOlderThan", mock.Anything, mock.Anything, 100).
		Return(int64(0), errors.New("db error"))

	err := s.job.Run(ctx)
	s.Require().Error(err)
}

func (s *HousekeepingJobSuite) TestRunStopsOnContextCancellation() {
	ctx, cancel := context.WithCancel(context.Background())

	s.factoryMock.On("KiwifyEventRepository", mock.Anything).Return(s.eventRepo)

	call := 0
	s.eventRepo.On("DeleteOlderThan", mock.Anything, mock.Anything, 100).
		Run(func(_ mock.Arguments) {
			call++
			if call == 1 {
				cancel()
			}
		}).
		Return(int64(50), nil)

	err := s.job.Run(ctx)
	s.Require().Error(err)
	s.Contains(err.Error(), "context cancelled")
}
