//go:build integration

package consumers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/messaging/database/consumers"
	cardrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type testConsumerEvent struct {
	eventType string
	payload   any
}

func (e *testConsumerEvent) GetEventType() string { return e.eventType }
func (e *testConsumerEvent) GetPayload() any      { return e.payload }

type OnboardingCardConsumerIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	db  *sqlx.DB
}

func TestOnboardingCardConsumerIntegration(t *testing.T) {
	suite.Run(t, new(OnboardingCardConsumerIntegrationSuite))
}

func (s *OnboardingCardConsumerIntegrationSuite) SetupSuite() {
	s.db, _ = testcontainer.Postgres(s.T())
}

func (s *OnboardingCardConsumerIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *OnboardingCardConsumerIntegrationSuite) newSUT() *consumers.OnboardingCardConsumer {
	o11y := noop.NewProvider()
	factory := cardrepos.NewRepositoryFactory(o11y)
	idemStorage := idempotency.NewPostgresStorage(s.db)
	createUoW := uow.NewUnitOfWork(s.db)
	createCard := usecases.NewCreateCard(createUoW, factory, idemStorage, o11y)
	return consumers.NewOnboardingCardConsumer(createCard, idemStorage, o11y)
}

func (s *OnboardingCardConsumerIntegrationSuite) insertUser(ctx context.Context) uuid.UUID {
	userID := uuid.New()
	number := fmt.Sprintf("+5511%09d", time.Now().UnixNano()%1000000000)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		 VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID.String(), number,
	)
	s.Require().NoError(err)
	return userID
}

func (s *OnboardingCardConsumerIntegrationSuite) buildEnvelope(id string, userID uuid.UUID, name string, limitCents int64, closingDay, dueDay int) outbox.Envelope {
	raw, err := json.Marshal(map[string]any{
		"UserID":     userID.String(),
		"Name":       name,
		"LimitCents": limitCents,
		"ClosingDay": closingDay,
		"DueDay":     dueDay,
	})
	s.Require().NoError(err)
	return outbox.Envelope{
		ID:              id,
		EventType:       "onboarding.card_registered",
		AggregateUserID: userID.String(),
		OccurredAt:      time.Now().UTC(),
		Payload:         json.RawMessage(raw),
	}
}

func countCardsByUser(t *testing.T, db *sqlx.DB, userID string) int {
	t.Helper()
	var n int
	err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM mecontrola.cards WHERE user_id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&n)
	if err != nil {
		t.Fatalf("countCardsByUser: %v", err)
	}
	return n
}

func (s *OnboardingCardConsumerIntegrationSuite) TestHandle_PersistsCardInDB() {
	userID := s.insertUser(s.ctx)
	env := s.buildEnvelope(uuid.NewString(), userID, "Nubank Integration", 500000, 10, 15)
	sut := s.newSUT()

	err := sut.Handle(s.ctx, &testConsumerEvent{eventType: "onboarding.card_registered", payload: env})
	s.Require().NoError(err)

	var count int
	queryErr := s.db.QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM mecontrola.cards WHERE name = $1 AND user_id = $2 AND deleted_at IS NULL`,
		"Nubank Integration", userID.String(),
	).Scan(&count)
	s.Require().NoError(queryErr)
	s.Equal(1, count)
}

func (s *OnboardingCardConsumerIntegrationSuite) TestHandle_Idempotent_SameEventID() {
	userID := s.insertUser(s.ctx)
	eventID := uuid.NewString()
	env := s.buildEnvelope(eventID, userID, "Itau Idempotent", 300000, 5, 10)
	sut := s.newSUT()

	err := sut.Handle(s.ctx, &testConsumerEvent{eventType: "onboarding.card_registered", payload: env})
	s.Require().NoError(err)

	_ = sut.Handle(s.ctx, &testConsumerEvent{eventType: "onboarding.card_registered", payload: env})

	s.Equal(1, countCardsByUser(s.T(), s.db, userID.String()))
}

func (s *OnboardingCardConsumerIntegrationSuite) TestHandle_MalformedPayload_NoSideEffect() {
	userID := s.insertUser(s.ctx)
	env := outbox.Envelope{
		ID:              uuid.NewString(),
		EventType:       "onboarding.card_registered",
		AggregateUserID: userID.String(),
		OccurredAt:      time.Now().UTC(),
		Payload:         json.RawMessage([]byte("invalid json")),
	}
	sut := s.newSUT()

	err := sut.Handle(s.ctx, &testConsumerEvent{eventType: "onboarding.card_registered", payload: env})
	s.Require().Error(err)

	s.Equal(0, countCardsByUser(s.T(), s.db, userID.String()))
}
