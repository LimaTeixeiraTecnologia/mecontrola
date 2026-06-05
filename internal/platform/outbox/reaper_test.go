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
	storage *outboxmocks.Storage
	factory *outboxmocks.OutboxRepositoryFactory
	cfg     configs.OutboxConfig
}

func TestReaper(t *testing.T) {
	suite.Run(t, new(ReaperSuite))
}

func (s *ReaperSuite) SetupTest() {
	s.storage = outboxmocks.NewStorage(s.T())
	s.factory = outboxmocks.NewOutboxRepositoryFactory(s.T())
	s.cfg = configs.OutboxConfig{ReaperStuckAfter: 5 * time.Minute}
}

type reaperScenario struct {
	name      string
	resetN    int64
	resetErr  error
	wantError bool
}

func (s *ReaperSuite) TestRun() {
	scenarios := []reaperScenario{
		{
			name:      "deve resetar eventos stuck com sucesso",
			resetN:    3,
			resetErr:  nil,
			wantError: false,
		},
		{
			name:      "deve retornar erro ao resetar stuck",
			resetN:    0,
			resetErr:  errors.New("db error"),
			wantError: true,
		},
		{
			name:      "deve ter sucesso sem eventos stuck",
			resetN:    0,
			resetErr:  nil,
			wantError: false,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			storage := outboxmocks.NewStorage(s.T())
			factory := outboxmocks.NewOutboxRepositoryFactory(s.T())
			factory.EXPECT().OutboxRepository(mock.Anything).Return(storage)
			storage.EXPECT().ResetStuck(mock.Anything, s.cfg.ReaperStuckAfter).Return(sc.resetN, sc.resetErr)

			r := outbox.NewReaperJob(&fakeUoWVoid{}, factory, s.cfg, noopLogger{})
			err := r.Run(context.Background())

			if sc.wantError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}
