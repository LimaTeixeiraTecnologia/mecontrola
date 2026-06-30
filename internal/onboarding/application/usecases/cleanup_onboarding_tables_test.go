package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
)

type CleanupOnboardingTablesSuite struct {
	suite.Suite
	ctx         context.Context
	obs         observability.Observability
	cleanupRepo *mocks.OnboardingCleanupRepository
}

func TestCleanupOnboardingTables(t *testing.T) {
	suite.Run(t, new(CleanupOnboardingTablesSuite))
}

func (s *CleanupOnboardingTablesSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.cleanupRepo = mocks.NewOnboardingCleanupRepository(s.T())
}

func (s *CleanupOnboardingTablesSuite) TestExecute() {
	type args struct {
		ctx context.Context
	}
	type dependencies struct {
		cleanupRepo *mocks.OnboardingCleanupRepository
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "deve completar sem erro quando ambas delecoes retornam menos que batchSize",
			args: args{ctx: s.ctx},
			dependencies: dependencies{
				cleanupRepo: func() *mocks.OnboardingCleanupRepository {
					s.cleanupRepo.EXPECT().
						DeleteMetaProcessedOlderThan(mock.Anything, mock.Anything, cleanupBatchSize).
						Return(int64(10), nil).
						Once()
					s.cleanupRepo.EXPECT().
						DeleteConsumerLookupAttemptsOlderThan(mock.Anything, mock.Anything, cleanupBatchSize).
						Return(int64(5), nil).
						Once()
					s.cleanupRepo.EXPECT().
						DeleteWelcomeProcessedOlderThan(mock.Anything, mock.Anything, cleanupBatchSize).
						Return(int64(3), nil).
						Once()
					return s.cleanupRepo
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando DeleteWelcomeProcessedOlderThan falha",
			args: args{ctx: s.ctx},
			dependencies: dependencies{
				cleanupRepo: func() *mocks.OnboardingCleanupRepository {
					s.cleanupRepo.EXPECT().
						DeleteMetaProcessedOlderThan(mock.Anything, mock.Anything, cleanupBatchSize).
						Return(int64(0), nil).
						Once()
					s.cleanupRepo.EXPECT().
						DeleteConsumerLookupAttemptsOlderThan(mock.Anything, mock.Anything, cleanupBatchSize).
						Return(int64(0), nil).
						Once()
					s.cleanupRepo.EXPECT().
						DeleteWelcomeProcessedOlderThan(mock.Anything, mock.Anything, cleanupBatchSize).
						Return(int64(0), errors.New("db error welcome")).
						Once()
					return s.cleanupRepo
				}(),
			},
			expect: func(err error) {
				s.Error(err)
				s.ErrorContains(err, "cleanup onboarding_welcome_processed")
			},
		},
		{
			name: "deve retornar erro quando DeleteMetaProcessedOlderThan falha",
			args: args{ctx: s.ctx},
			dependencies: dependencies{
				cleanupRepo: func() *mocks.OnboardingCleanupRepository {
					s.cleanupRepo.EXPECT().
						DeleteMetaProcessedOlderThan(mock.Anything, mock.Anything, cleanupBatchSize).
						Return(int64(0), errors.New("db error")).
						Once()
					return s.cleanupRepo
				}(),
			},
			expect: func(err error) {
				s.Error(err)
				s.ErrorContains(err, "cleanup channel_processed_messages")
			},
		},
		{
			name: "deve retornar erro quando DeleteConsumerLookupAttemptsOlderThan falha",
			args: args{ctx: s.ctx},
			dependencies: dependencies{
				cleanupRepo: func() *mocks.OnboardingCleanupRepository {
					s.cleanupRepo.EXPECT().
						DeleteMetaProcessedOlderThan(mock.Anything, mock.Anything, cleanupBatchSize).
						Return(int64(0), nil).
						Once()
					s.cleanupRepo.EXPECT().
						DeleteConsumerLookupAttemptsOlderThan(mock.Anything, mock.Anything, cleanupBatchSize).
						Return(int64(0), errors.New("db error lookup")).
						Once()
					return s.cleanupRepo
				}(),
			},
			expect: func(err error) {
				s.Error(err)
				s.ErrorContains(err, "cleanup consumer_lookup_attempts")
			},
		},
		{
			name: "deve iterar ate DeleteMetaProcessed retornar menos que batchSize",
			args: args{ctx: s.ctx},
			dependencies: dependencies{
				cleanupRepo: func() *mocks.OnboardingCleanupRepository {
					s.cleanupRepo.EXPECT().
						DeleteMetaProcessedOlderThan(mock.Anything, mock.Anything, cleanupBatchSize).
						Return(int64(cleanupBatchSize), nil).
						Once()
					s.cleanupRepo.EXPECT().
						DeleteMetaProcessedOlderThan(mock.Anything, mock.Anything, cleanupBatchSize).
						Return(int64(50), nil).
						Once()
					s.cleanupRepo.EXPECT().
						DeleteConsumerLookupAttemptsOlderThan(mock.Anything, mock.Anything, cleanupBatchSize).
						Return(int64(0), nil).
						Once()
					s.cleanupRepo.EXPECT().
						DeleteWelcomeProcessedOlderThan(mock.Anything, mock.Anything, cleanupBatchSize).
						Return(int64(0), nil).
						Once()
					return s.cleanupRepo
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando contexto cancelado no meio do loop de meta processed",
			args: func() args {
				ctx, cancel := context.WithCancel(context.Background())
				s.cleanupRepo.EXPECT().
					DeleteMetaProcessedOlderThan(mock.Anything, mock.Anything, cleanupBatchSize).
					RunAndReturn(func(_ context.Context, _ time.Time, _ int) (int64, error) {
						cancel()
						return int64(cleanupBatchSize), nil
					}).
					Once()
				return args{ctx: ctx}
			}(),
			dependencies: dependencies{cleanupRepo: s.cleanupRepo},
			expect: func(err error) {
				s.Error(err)
				s.ErrorContains(err, "context cancelled")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewCleanupOnboardingTables(scenario.dependencies.cleanupRepo, 30*24*time.Hour, s.obs)
			err := uc.Execute(scenario.args.ctx)
			scenario.expect(err)
		})
	}
}
