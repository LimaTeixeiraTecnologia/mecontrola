package outbox_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	dbmocks "github.com/JailtonJunior94/devkit-go/pkg/database/mocks"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type PublisherSuite struct {
	suite.Suite
	storage   *outboxmocks.Storage
	dbtx      *dbmocks.MockDBTX
	publisher outbox.Publisher
}

func TestPublisher(t *testing.T) {
	suite.Run(t, new(PublisherSuite))
}

func (s *PublisherSuite) SetupTest() {
	s.storage = outboxmocks.NewStorage(s.T())
	s.dbtx = dbmocks.NewMockDBTX(s.T())
	s.publisher = outbox.NewPostgresPublisher(s.storage, configs.OutboxConfig{RetryMaxAttempts: 15})
}

func (s *PublisherSuite) newValidEvent() outbox.Event {
	evt, err := outbox.NewEvent(outbox.EventInput{
		Type:          "test.event",
		AggregateType: "TestAggregate",
		AggregateID:   "agg-1",
		Payload:       []byte(`{"x":1}`),
	})
	s.Require().NoError(err)
	return evt
}

func (s *PublisherSuite) TestPublish_Sucesso() {
	evt := s.newValidEvent()
	ctx := database.WithTx(context.Background(), s.dbtx)

	s.storage.EXPECT().Insert(ctx, evt, 15).Return(nil)

	err := s.publisher.Publish(ctx, evt)
	s.NoError(err)
}

func (s *PublisherSuite) TestPublish_IDInvalido() {
	evt := s.newValidEvent()
	evt.ID = "not-a-uuid"
	ctx := database.WithTx(context.Background(), s.dbtx)

	err := s.publisher.Publish(ctx, evt)
	s.ErrorIs(err, outbox.ErrEventIDMissing)
}

func (s *PublisherSuite) TestPublish_TypeVazio() {
	evt := s.newValidEvent()
	evt.Type = ""
	ctx := database.WithTx(context.Background(), s.dbtx)

	err := s.publisher.Publish(ctx, evt)
	s.ErrorIs(err, outbox.ErrEventTypeMissing)
}

func (s *PublisherSuite) TestPublish_PayloadInvalido() {
	evt := s.newValidEvent()
	evt.Payload = []byte(`not-json`)
	ctx := database.WithTx(context.Background(), s.dbtx)

	err := s.publisher.Publish(ctx, evt)
	s.ErrorIs(err, outbox.ErrInvalidPayload)
}

func (s *PublisherSuite) TestPublish_PayloadNaoEObjeto() {
	ctx := database.WithTx(context.Background(), s.dbtx)

	for _, payload := range [][]byte{[]byte(`null`), []byte(`[]`), []byte(`42`), []byte(`"string"`)} {
		evt := s.newValidEvent()
		evt.Payload = payload
		err := s.publisher.Publish(ctx, evt)
		s.ErrorIs(err, outbox.ErrInvalidPayload, "payload %s deveria falhar", payload)
	}
}

func (s *PublisherSuite) TestPublish_OccurredAtZero() {
	evt := s.newValidEvent()
	evt.OccurredAt = time.Time{}
	ctx := database.WithTx(context.Background(), s.dbtx)

	err := s.publisher.Publish(ctx, evt)
	s.ErrorIs(err, outbox.ErrOccurredAtZero)
}

func (s *PublisherSuite) TestPublish_ConflictIdempotente() {
	evt := s.newValidEvent()
	ctx := database.WithTx(context.Background(), s.dbtx)

	s.storage.EXPECT().Insert(ctx, evt, 15).Return(nil)

	err1 := s.publisher.Publish(ctx, evt)
	s.NoError(err1)

	s.storage.EXPECT().Insert(ctx, evt, 15).Return(nil)

	err2 := s.publisher.Publish(ctx, evt)
	s.NoError(err2)
}

func (s *PublisherSuite) TestPublish_ErroStorage() {
	evt := s.newValidEvent()
	ctx := database.WithTx(context.Background(), s.dbtx)
	dbErr := errors.New("db failure")

	s.storage.EXPECT().Insert(ctx, evt, 15).Return(dbErr)

	err := s.publisher.Publish(ctx, evt)
	s.Error(err)
}
