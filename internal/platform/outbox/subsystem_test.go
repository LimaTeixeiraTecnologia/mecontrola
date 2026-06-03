package outbox_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type SubsystemSuite struct {
	suite.Suite
	ctx context.Context
}

func TestSubsystem(t *testing.T) {
	suite.Run(t, new(SubsystemSuite))
}

func (s *SubsystemSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *SubsystemSuite) validConfig() configs.OutboxConfig {
	return configs.OutboxConfig{
		DispatcherEnabled:         true,
		DispatcherTickInterval:    100 * time.Millisecond,
		DispatcherBatchSize:       10,
		DispatcherHandlerTimeout:  5 * time.Second,
		RetryMaxAttempts:          5,
		RetryBaseBackoff:          1 * time.Second,
		RetryMaxBackoff:           10 * time.Second,
		HousekeepingRetentionDays: 30,
		HousekeepingSchedule:      "@daily",
		ReaperInterval:            "@every 1m",
		ReaperStuckAfter:          5 * time.Minute,
	}
}

func (s *SubsystemSuite) TestName() {
	registry := mocks.NewRegistry(s.T())
	registry.EXPECT().Validate().Return(nil)

	storage := mocks.NewStorage(s.T())

	sub, err := outbox.NewSubsystem(outbox.SubsystemDeps{
		Config:   s.validConfig(),
		Storage:  storage,
		Registry: registry,
	})
	s.Require().NoError(err)
	s.Equal("outbox", sub.Name())
}

func (s *SubsystemSuite) TestNewSubsystemStorageNil() {
	registry := mocks.NewRegistry(s.T())
	_, err := outbox.NewSubsystem(outbox.SubsystemDeps{
		Config:   s.validConfig(),
		Storage:  nil,
		Registry: registry,
	})
	s.Error(err)
	s.ErrorContains(err, "storage é obrigatório")
}

func (s *SubsystemSuite) TestNewSubsystemRegistryNil() {
	storage := mocks.NewStorage(s.T())
	_, err := outbox.NewSubsystem(outbox.SubsystemDeps{
		Config:   s.validConfig(),
		Storage:  storage,
		Registry: nil,
	})
	s.Error(err)
	s.ErrorContains(err, "registry é obrigatório")
}

func (s *SubsystemSuite) TestNewSubsystemRegistryValidateFails() {
	registry := mocks.NewRegistry(s.T())
	registry.EXPECT().Validate().Return(outbox.ErrDuplicateSubscription)

	storage := mocks.NewStorage(s.T())

	_, err := outbox.NewSubsystem(outbox.SubsystemDeps{
		Config:   s.validConfig(),
		Storage:  storage,
		Registry: registry,
	})
	s.Error(err)
	s.ErrorIs(err, outbox.ErrDuplicateSubscription)
}

func (s *SubsystemSuite) TestStartAndStopDispatcherEnabled() {
	registry := mocks.NewRegistry(s.T())
	registry.EXPECT().Validate().Return(nil)
	// Dispatcher chama SubscriptionsFor para cada claim; pode chamar ou não dependendo do tick
	registry.EXPECT().SubscriptionsFor(mock.Anything).Maybe().Return(nil)

	storage := mocks.NewStorage(s.T())
	// ClaimReady pode ser chamado pelo dispatcher loop — permitir chamadas opcionais
	storage.EXPECT().ClaimReady(mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil, nil)

	sub, err := outbox.NewSubsystem(outbox.SubsystemDeps{
		Config:   s.validConfig(),
		Storage:  storage,
		Registry: registry,
	})
	s.Require().NoError(err)

	s.Require().NoError(sub.Start(s.ctx))

	stopCtx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
	defer cancel()
	s.NoError(sub.Stop(stopCtx))
}

func (s *SubsystemSuite) TestStartAndStopDispatcherDisabled() {
	cfg := s.validConfig()
	cfg.DispatcherEnabled = false

	registry := mocks.NewRegistry(s.T())
	registry.EXPECT().Validate().Return(nil)

	storage := mocks.NewStorage(s.T())

	sub, err := outbox.NewSubsystem(outbox.SubsystemDeps{
		Config:   cfg,
		Storage:  storage,
		Registry: registry,
	})
	s.Require().NoError(err)

	s.Require().NoError(sub.Start(s.ctx))

	stopCtx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
	defer cancel()
	s.NoError(sub.Stop(stopCtx))
}

func (s *SubsystemSuite) TestStartInvalidCronSchedule() {
	cfg := s.validConfig()
	cfg.HousekeepingSchedule = "not-a-valid-cron"

	registry := mocks.NewRegistry(s.T())
	registry.EXPECT().Validate().Return(nil)

	storage := mocks.NewStorage(s.T())

	sub, err := outbox.NewSubsystem(outbox.SubsystemDeps{
		Config:   cfg,
		Storage:  storage,
		Registry: registry,
	})
	s.Require().NoError(err)

	err = sub.Start(s.ctx)
	s.Error(err)
	s.ErrorContains(err, "cron.Start")
}
