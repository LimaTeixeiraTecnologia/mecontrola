package outbox_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

const retention90Days = 90 * 24 * time.Hour

type HousekeepingSuite struct {
	suite.Suite
	cfg configs.OutboxConfig
}

func TestHousekeeping(t *testing.T) {
	suite.Run(t, new(HousekeepingSuite))
}

func (s *HousekeepingSuite) SetupTest() {
	s.cfg = configs.OutboxConfig{HousekeepingRetentionDays: 90}
}

func (s *HousekeepingSuite) TestRun() {
	type args struct {
		ctx context.Context
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func() (*outboxmocks.OutboxRepositoryFactory, *outboxmocks.Storage)
		expect func(error)
	}{
		{
			name: "deve deletar lotes ate o limite",
			args: args{ctx: context.Background()},
			setup: func() (*outboxmocks.OutboxRepositoryFactory, *outboxmocks.Storage) {
				factory := outboxmocks.NewOutboxRepositoryFactory(s.T())
				storage := outboxmocks.NewStorage(s.T())
				factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Times(3)
				storage.EXPECT().DeletePublishedBatch(mock.Anything, retention90Days, 1000).Return(int64(1000), nil).Once()
				storage.EXPECT().DeletePublishedBatch(mock.Anything, retention90Days, 1000).Return(int64(500), nil).Once()
				storage.EXPECT().DeletePublishedBatch(mock.Anything, retention90Days, 1000).Return(int64(0), nil).Once()
				return factory, storage
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "deve concluir sem eventos publicados",
			args: args{ctx: context.Background()},
			setup: func() (*outboxmocks.OutboxRepositoryFactory, *outboxmocks.Storage) {
				factory := outboxmocks.NewOutboxRepositoryFactory(s.T())
				storage := outboxmocks.NewStorage(s.T())
				factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Once()
				storage.EXPECT().DeletePublishedBatch(mock.Anything, retention90Days, 1000).Return(int64(0), nil).Once()
				return factory, storage
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "deve retornar erro ao falhar no delete",
			args: args{ctx: context.Background()},
			setup: func() (*outboxmocks.OutboxRepositoryFactory, *outboxmocks.Storage) {
				factory := outboxmocks.NewOutboxRepositoryFactory(s.T())
				storage := outboxmocks.NewStorage(s.T())
				factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Once()
				storage.EXPECT().DeletePublishedBatch(mock.Anything, retention90Days, 1000).Return(int64(0), errors.New("db error")).Once()
				return factory, storage
			},
			expect: func(err error) { s.Error(err) },
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			factory, _ := scenario.setup()
			sut := outbox.NewHousekeepingJob(&unitOfWorkVoid{}, factory, s.cfg, noopLogger{})
			err := sut.Run(scenario.args.ctx)
			scenario.expect(err)
		})
	}
}
