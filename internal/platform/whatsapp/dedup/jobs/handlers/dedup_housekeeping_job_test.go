package handlers_test

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup/mocks"
)

type DedupHousekeepingJobSuite struct {
	suite.Suite
	repoMock *mocks.MessageRepository
}

func TestDedupHousekeepingJobSuite(t *testing.T) {
	suite.Run(t, new(DedupHousekeepingJobSuite))
}

func (s *DedupHousekeepingJobSuite) SetupTest() {
	s.repoMock = mocks.NewMessageRepository(s.T())
}

func (s *DedupHousekeepingJobSuite) newJob(cfg configs.WhatsAppConfig) *handlers.DedupHousekeepingJob {
	cleanup := dedup.NewCleanupProcessedMessages(s.repoMock, cfg, noop.NewProvider())
	return handlers.NewDedupHousekeepingJob(cleanup, cfg)
}

func (s *DedupHousekeepingJobSuite) TestMetadata() {
	scenarios := []struct {
		name             string
		cfg              configs.WhatsAppConfig
		expectedSchedule string
	}{
		{
			name:             "deve usar schedule configurado",
			cfg:              configs.WhatsAppConfig{DedupHousekeepingSchedule: "@weekly", DedupHousekeepingRetentionDays: 30, DedupHousekeepingBatch: 10000},
			expectedSchedule: "@weekly",
		},
		{
			name:             "deve usar schedule padrao quando vazio",
			cfg:              configs.WhatsAppConfig{DedupHousekeepingRetentionDays: 30, DedupHousekeepingBatch: 10000},
			expectedSchedule: "@daily",
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			job := s.newJob(scenario.cfg)
			s.Equal("whatsapp-dedup-housekeeping", job.Name())
			s.Equal(scenario.expectedSchedule, job.Schedule())
			s.Positive(job.Timeout())
		})
	}
}

func (s *DedupHousekeepingJobSuite) TestRun() {
	cfg := configs.WhatsAppConfig{DedupHousekeepingRetentionDays: 30, DedupHousekeepingBatch: 10000}
	s.repoMock.EXPECT().DeleteProcessedBefore(mock.Anything, mock.Anything, 10000).Return(int64(0), nil).Once()

	job := s.newJob(cfg)
	s.Require().NoError(job.Run(context.Background()))
}
