//go:build integration

package consumers_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/binding"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	domainservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories"
	postrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type fakePublisher struct{}

func (f *fakePublisher) Publish(_ context.Context, _ outbox.Event) error { return nil }

type fakeIdentityGW struct {
	userID string
	db     *sqlx.DB
}

func (f *fakeIdentityGW) UpsertUserByWhatsApp(ctx context.Context, mobileE164, _ string) (appinterfaces.UpsertUserResult, error) {
	_, err := f.db.ExecContext(ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, status)
		 VALUES ($1, $2, 'ACTIVE')
		 ON CONFLICT (id) DO NOTHING`,
		f.userID, mobileE164,
	)
	if err != nil {
		return appinterfaces.UpsertUserResult{}, err
	}
	return appinterfaces.UpsertUserResult{UserID: f.userID}, nil
}

type fakeGateway struct{}

func (f *fakeGateway) SendActivationTemplate(_ context.Context, _, _, _ string) (string, error) {
	return "", nil
}

func (f *fakeGateway) SendTextMessage(_ context.Context, _, _ string) error { return nil }

type integrationAttemptEvent struct {
	envelope outbox.Envelope
}

func (e *integrationAttemptEvent) GetEventType() string { return "onboarding.activation.attempted.v1" }
func (e *integrationAttemptEvent) GetPayload() any      { return e.envelope }

type ActivationAttemptConsumerIntegrationSuite struct {
	suite.Suite
	db *sqlx.DB
}

func TestActivationAttemptConsumerIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ActivationAttemptConsumerIntegrationSuite))
}

func (s *ActivationAttemptConsumerIntegrationSuite) SetupTest() {
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
}

func (s *ActivationAttemptConsumerIntegrationSuite) insertPaidToken(mobileE164 string) entities.MagicToken {
	ctx := context.Background()
	obs := noop.NewProvider()
	repo := postrepo.NewMagicTokenRepository(obs, s.db)

	hash := make([]byte, 32)
	_, _ = rand.Read(hash)
	token, err := entities.NewMagicToken(uuid.NewString(), hash, "plan-test", time.Now().UTC().Add(7*24*time.Hour))
	s.Require().NoError(err)
	s.Require().NoError(repo.Insert(ctx, token))

	subID := s.seedSubscription(ctx)
	paid, err := token.MarkPaid(subID, mobileE164, "test@example.com", uuid.NewString(), time.Now().UTC())
	s.Require().NoError(err)
	s.Require().NoError(repo.UpdateMarkPaid(ctx, paid))
	return paid
}

func (s *ActivationAttemptConsumerIntegrationSuite) seedSubscription(ctx context.Context) string {
	subID := uuid.NewString()
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO mecontrola.billing_subscriptions
		   (id, funnel_token, user_id, kiwify_order_id, plan_code, status,
		    period_start, period_end, last_event_at)
		 VALUES ($1, $2, NULL, $3, 'MONTHLY', 'ACTIVE', $4, $5, $4)`,
		subID, uuid.NewString(), uuid.NewString(), now, now.Add(30*24*time.Hour),
	)
	s.Require().NoError(err)
	return subID
}

func (s *ActivationAttemptConsumerIntegrationSuite) buildConsumer() *consumers.ActivationAttemptConsumer {
	obs := noop.NewProvider()
	factory := repositories.NewRepositoryFactory(obs)
	subBinder := postrepo.NewSubscriptionBinder(obs, s.db)
	workflow := domainservices.NewMagicTokenWorkflow()
	publisher := &fakePublisher{}
	identityGW := &fakeIdentityGW{userID: uuid.NewString(), db: s.db}
	bindingSvc := binding.NewSubscriptionBindingService(identityGW, subBinder, workflow, publisher, id.NewUUIDGenerator())
	activationWindow := 24 * time.Hour
	consumeUoW := uow.NewUnitOfWork(s.db)
	activateUoW := uow.NewUnitOfWork(s.db)
	throttle := postrepo.NewNoMatchThrottleRepository(obs, s.db)
	consumeToken := usecases.NewConsumeMagicToken(consumeUoW, factory, bindingSvc, id.NewUUIDGenerator(), activationWindow, obs)
	activateUC := usecases.NewActivateFromInbound(
		activateUoW,
		factory,
		bindingSvc,
		consumeToken,
		&fakeGateway{},
		throttle,
		activationWindow,
		"nao encontrado",
		obs,
	)
	return consumers.NewActivationAttemptConsumer(activateUC, obs)
}

func (s *ActivationAttemptConsumerIntegrationSuite) newEvent(peerE164, messageID string) events.Event {
	raw, _ := json.Marshal(map[string]any{
		"peer_e164":  peerE164,
		"text":       "Oi",
		"message_id": messageID,
	})
	return &integrationAttemptEvent{
		envelope: outbox.Envelope{
			ID:        uuid.NewString(),
			EventType: "onboarding.activation.attempted.v1",
			Payload:   json.RawMessage(raw),
		},
	}
}

func (s *ActivationAttemptConsumerIntegrationSuite) TestHandleActivatesTokenConsumed() {
	ctx := context.Background()
	mobileE164 := "+5511988887777"

	token := s.insertPaidToken(mobileE164)
	consumer := s.buildConsumer()

	err := consumer.Handle(ctx, s.newEvent(mobileE164, uuid.NewString()))
	s.Require().NoError(err)

	repo := postrepo.NewMagicTokenRepository(noop.NewProvider(), s.db)
	found, findErr := repo.FindByHash(ctx, token.TokenHash())
	s.Require().NoError(findErr)
	s.Equal(valueobjects.TokenStatusConsumed, found.Status())
}

func (s *ActivationAttemptConsumerIntegrationSuite) TestHandleIdempotentSecondActivation() {
	ctx := context.Background()
	mobileE164 := "+5511988886666"

	_ = s.insertPaidToken(mobileE164)
	consumer := s.buildConsumer()

	err := consumer.Handle(ctx, s.newEvent(mobileE164, uuid.NewString()))
	s.Require().NoError(err)

	err = consumer.Handle(ctx, s.newEvent(mobileE164, uuid.NewString()))
	s.Require().NoError(err)
}
