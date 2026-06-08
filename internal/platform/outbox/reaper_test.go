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

type ReaperSuite struct {
	suite.Suite
	cfg configs.OutboxConfig
}

func TestReaper(t *testing.T) {
	suite.Run(t, new(ReaperSuite))
}

func (s *ReaperSuite) SetupTest() {
	s.cfg = configs.OutboxConfig{ReaperStuckAfter: 5 * time.Minute}
}

func (s *ReaperSuite) TestRun() {
	type args struct {
		ctx context.Context
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func() *outboxmocks.OutboxRepositoryFactory
		expect func(error)
	}{
		{
			name: "deve resetar eventos stuck com sucesso",
			args: args{ctx: context.Background()},
			setup: func() *outboxmocks.OutboxRepositoryFactory {
				factory := outboxmocks.NewOutboxRepositoryFactory(s.T())
				storage := outboxmocks.NewStorage(s.T())
				factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Once()
				storage.EXPECT().ResetStuck(mock.Anything, s.cfg.ReaperStuckAfter).Return(int64(3), nil).Once()
				return factory
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "deve retornar erro ao resetar stuck",
			args: args{ctx: context.Background()},
			setup: func() *outboxmocks.OutboxRepositoryFactory {
				factory := outboxmocks.NewOutboxRepositoryFactory(s.T())
				storage := outboxmocks.NewStorage(s.T())
				factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Once()
				storage.EXPECT().ResetStuck(mock.Anything, s.cfg.ReaperStuckAfter).Return(int64(0), errors.New("db error")).Once()
				return factory
			},
			expect: func(err error) { s.Error(err) },
		},
		{
			name: "deve concluir quando nao houver eventos stuck",
			args: args{ctx: context.Background()},
			setup: func() *outboxmocks.OutboxRepositoryFactory {
				factory := outboxmocks.NewOutboxRepositoryFactory(s.T())
				storage := outboxmocks.NewStorage(s.T())
				factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Once()
				storage.EXPECT().ResetStuck(mock.Anything, s.cfg.ReaperStuckAfter).Return(int64(0), nil).Once()
				return factory
			},
			expect: func(err error) { s.NoError(err) },
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			factory := scenario.setup()
			sut := outbox.NewReaperJob(&unitOfWorkVoid{}, factory, s.cfg, noopLogger{})
			err := sut.Run(scenario.args.ctx)
			scenario.expect(err)
		})
	}
}
