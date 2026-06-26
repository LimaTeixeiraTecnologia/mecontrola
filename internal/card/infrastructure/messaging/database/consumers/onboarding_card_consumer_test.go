package consumers_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type stubCardCreator struct {
	capturedInput input.CreateCard
	capturedCtx   context.Context
	called        bool
	err           error
}

func (f *stubCardCreator) Execute(ctx context.Context, in input.CreateCard) (output.Card, error) {
	f.called = true
	f.capturedCtx = ctx
	f.capturedInput = in
	return output.Card{}, f.err
}

type fakeIdempotencyStorage struct {
	records map[string]idempotency.Record
}

func newFakeIdempotencyStorage() *fakeIdempotencyStorage {
	return &fakeIdempotencyStorage{records: make(map[string]idempotency.Record)}
}

func (f *fakeIdempotencyStorage) Get(_ context.Context, scope, key, userID string) (idempotency.Record, error) {
	if rec, ok := f.records[scope+":"+key+":"+userID]; ok {
		return rec, nil
	}
	return idempotency.Record{}, idempotency.ErrNotFound
}

func (f *fakeIdempotencyStorage) Put(_ context.Context, rec idempotency.Record) error {
	f.records[rec.Scope+":"+rec.Key+":"+rec.UserID] = rec
	return nil
}

type stubEvent struct {
	eventType string
	payload   any
}

func (e stubEvent) GetEventType() string { return e.eventType }
func (e stubEvent) GetPayload() any      { return e.payload }

type onboardingCardConsumerSuite struct {
	suite.Suite
}

func TestOnboardingCardConsumer(t *testing.T) {
	suite.Run(t, new(onboardingCardConsumerSuite))
}

func (s *onboardingCardConsumerSuite) buildEnvelope(id string, userID uuid.UUID, name string, limitCents int64, closingDay, dueDay int) outbox.Envelope {
	raw, _ := json.Marshal(map[string]any{
		"UserID":     userID.String(),
		"Name":       name,
		"LimitCents": limitCents,
		"ClosingDay": closingDay,
		"DueDay":     dueDay,
	})
	return outbox.Envelope{ID: id, Payload: raw}
}

func (s *onboardingCardConsumerSuite) TestHappyPath_CallsExecuteWithCorrectFields() {
	userID := uuid.New()
	eventID := uuid.NewString()
	env := s.buildEnvelope(eventID, userID, "Nubank", 500000, 10, 15)
	storage := newFakeIdempotencyStorage()
	creator := &stubCardCreator{}
	consumer := consumers.NewOnboardingCardConsumer(creator, storage, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.card_registered", payload: env})
	s.Require().NoError(err)

	s.True(creator.called)
	s.Equal(userID, creator.capturedInput.UserID)
	s.Equal("Nubank", creator.capturedInput.Name)
	s.Equal("Nubank", creator.capturedInput.Nickname)
	s.Equal(int64(500000), creator.capturedInput.LimitCents)
	s.Equal(10, creator.capturedInput.ClosingDay)
	s.Require().NotNil(creator.capturedInput.DueDay)
	s.Equal(15, *creator.capturedInput.DueDay)

	ic, ok := idempotency.FromContext(creator.capturedCtx)
	s.Require().True(ok)
	s.Equal("event:onboarding.card_registered", ic.Scope)
	s.Equal(eventID, ic.Key)
	s.Equal(userID.String(), ic.UserID)
	expectedHash := sha256.Sum256([]byte(eventID))
	s.Equal(hex.EncodeToString(expectedHash[:]), ic.RequestHash)
}

func (s *onboardingCardConsumerSuite) TestMissingDueDay_ReturnsError() {
	userID := uuid.New()
	env := s.buildEnvelope(uuid.NewString(), userID, "Nubank", 500000, 10, 0)
	storage := newFakeIdempotencyStorage()
	creator := &stubCardCreator{}
	consumer := consumers.NewOnboardingCardConsumer(creator, storage, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.card_registered", payload: env})
	s.Require().Error(err)
	s.ErrorContains(err, "due_day")
	s.False(creator.called)
}

func (s *onboardingCardConsumerSuite) TestUsecaseError_IsPropagated() {
	userID := uuid.New()
	env := s.buildEnvelope(uuid.NewString(), userID, "Itau", 100000, 5, 10)
	storage := newFakeIdempotencyStorage()
	creator := &stubCardCreator{err: errors.New("infra error")}
	consumer := consumers.NewOnboardingCardConsumer(creator, storage, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.card_registered", payload: env})
	s.Require().Error(err)
	s.ErrorContains(err, "infra error")
}

func (s *onboardingCardConsumerSuite) TestInvalidPayloadType_ReturnsError() {
	storage := newFakeIdempotencyStorage()
	consumer := consumers.NewOnboardingCardConsumer(&stubCardCreator{}, storage, noop.NewProvider())
	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.card_registered", payload: "not-envelope"})
	s.Require().Error(err)
	s.ErrorContains(err, "tipo de payload inesperado")
}

func (s *onboardingCardConsumerSuite) TestInvalidUserID_ReturnsError() {
	raw, _ := json.Marshal(map[string]any{
		"UserID":     "not-a-uuid",
		"Name":       "Card",
		"LimitCents": 100000,
		"ClosingDay": 5,
		"DueDay":     10,
	})
	env := outbox.Envelope{ID: uuid.NewString(), Payload: raw}
	storage := newFakeIdempotencyStorage()
	consumer := consumers.NewOnboardingCardConsumer(&stubCardCreator{}, storage, noop.NewProvider())
	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.card_registered", payload: env})
	s.Require().Error(err)
	s.ErrorContains(err, "user_id inválido")
}

func (s *onboardingCardConsumerSuite) TestMalformedJSON_ReturnsError() {
	env := outbox.Envelope{ID: uuid.NewString(), Payload: []byte("not-json")}
	storage := newFakeIdempotencyStorage()
	consumer := consumers.NewOnboardingCardConsumer(&stubCardCreator{}, storage, noop.NewProvider())
	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.card_registered", payload: env})
	s.Require().Error(err)
	s.ErrorContains(err, "deserializar payload")
}

func (s *onboardingCardConsumerSuite) TestReplay_SkipsCreate() {
	userID := uuid.New()
	eventID := uuid.NewString()
	env := s.buildEnvelope(eventID, userID, "Nubank", 500000, 10, 15)
	storage := newFakeIdempotencyStorage()
	creator := &stubCardCreator{}
	consumer := consumers.NewOnboardingCardConsumer(creator, storage, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.card_registered", payload: env})
	s.Require().NoError(err)
	s.True(creator.called)

	creator.called = false
	_ = storage.Put(context.Background(), idempotency.Record{
		Scope:       "event:onboarding.card_registered",
		Key:         eventID,
		UserID:      userID.String(),
		RequestHash: eventID,
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
	})

	err = consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.card_registered", payload: env})
	s.Require().NoError(err)
	s.False(creator.called)
}
