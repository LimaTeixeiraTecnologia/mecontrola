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

func (s *HousekeepingSuite) TestRun_DeleteBatchesAteLimite() {
	storage := outboxmocks.NewStorage(s.T())
	factory := outboxmocks.NewOutboxRepositoryFactory(s.T())

	factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Times(3)
	storage.EXPECT().DeletePublishedBatch(mock.Anything, retention90Days, 1000).Return(int64(1000), nil).Once()
	storage.EXPECT().DeletePublishedBatch(mock.Anything, retention90Days, 1000).Return(int64(500), nil).Once()
	storage.EXPECT().DeletePublishedBatch(mock.Anything, retention90Days, 1000).Return(int64(0), nil).Once()

	h := outbox.NewHousekeepingJob(&fakeUoWVoid{}, factory, s.cfg, noopLogger{})
	err := h.Run(context.Background())
	s.NoError(err)
}

func (s *HousekeepingSuite) TestRun_SemEventos_Sucesso() {
	storage := outboxmocks.NewStorage(s.T())
	factory := outboxmocks.NewOutboxRepositoryFactory(s.T())

	factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Once()
	storage.EXPECT().DeletePublishedBatch(mock.Anything, retention90Days, 1000).Return(int64(0), nil).Once()

	h := outbox.NewHousekeepingJob(&fakeUoWVoid{}, factory, s.cfg, noopLogger{})
	err := h.Run(context.Background())
	s.NoError(err)
}

func (s *HousekeepingSuite) TestRun_ErroDelete_RetornaErro() {
	storage := outboxmocks.NewStorage(s.T())
	factory := outboxmocks.NewOutboxRepositoryFactory(s.T())

	factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Once()
	storage.EXPECT().DeletePublishedBatch(mock.Anything, retention90Days, 1000).Return(int64(0), errors.New("db error")).Once()

	h := outbox.NewHousekeepingJob(&fakeUoWVoid{}, factory, s.cfg, noopLogger{})
	err := h.Run(context.Background())
	s.Error(err)
}
