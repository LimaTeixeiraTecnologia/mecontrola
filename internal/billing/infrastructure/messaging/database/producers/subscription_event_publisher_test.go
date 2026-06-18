package producers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	dbmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/mocks"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/messaging/database/producers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type SubscriptionEventPublisherSuite struct {
	suite.Suite
	storage        *outboxmocks.Storage
	repoFactory    *outboxmocks.OutboxRepositoryFactory
	tx             *dbmocks.MockDBTX
	subscriptionID string
	occurredAt     time.Time
}

func TestSubscriptionEventPublisherSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionEventPublisherSuite))
}

func (s *SubscriptionEventPublisherSuite) SetupTest() {
	s.storage = outboxmocks.NewStorage(s.T())
	s.repoFactory = outboxmocks.NewOutboxRepositoryFactory(s.T())
	s.tx = dbmocks.NewMockDBTX(s.T())

	s.subscriptionID = "sub-unit-001"
	s.occurredAt = time.Now().UTC().Truncate(time.Millisecond)
}

func (s *SubscriptionEventPublisherSuite) newPublisher() *producers.SubscriptionEventPublisher {
	cfg := configs.OutboxConfig{RetryMaxAttempts: 5}
	idGen := id.NewUUIDGenerator()
	return producers.NewSubscriptionEventPublisher(s.repoFactory, cfg, idGen, noop.NewProvider())
}

func (s *SubscriptionEventPublisherSuite) newActiveSubscription(token string) entities.Subscription {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	ft, err := valueobjects.NewFunnelToken(token)
	s.Require().NoError(err)

	sub := entities.NewSubscription(plan, ft)
	s.Require().NoError(sub.Activate(s.occurredAt))
	return sub
}

func (s *SubscriptionEventPublisherSuite) newPastDueSubscription() entities.Subscription {
	sub := s.newActiveSubscription("token-pastdue-001")
	s.Require().NoError(sub.MarkPastDue(s.occurredAt.Add(time.Hour), 3*24*time.Hour))
	return sub
}

func (s *SubscriptionEventPublisherSuite) newCanceledSubscription() entities.Subscription {
	sub := s.newActiveSubscription("token-canceled-001")
	s.Require().NoError(sub.MarkCanceled(s.occurredAt.Add(time.Hour)))
	return sub
}

func (s *SubscriptionEventPublisherSuite) newRefundedSubscription() entities.Subscription {
	sub := s.newActiveSubscription("token-refunded-001")
	s.Require().NoError(sub.MarkRefunded(s.occurredAt.Add(time.Hour)))
	return sub
}

func (s *SubscriptionEventPublisherSuite) expectInsertOnce(eventType string) {
	s.repoFactory.EXPECT().
		OutboxRepository(s.tx).
		Return(s.storage).
		Once()

	s.storage.EXPECT().
		Insert(mock.Anything, mock.MatchedBy(func(evt outbox.Event) bool {
			return evt.Type == eventType &&
				evt.AggregateType == "Subscription" &&
				evt.AggregateID == s.subscriptionID &&
				len(evt.Payload) > 0
		}), 5).
		Return(nil).
		Once()
}

func (s *SubscriptionEventPublisherSuite) expectInsertError(storageErr error) {
	s.repoFactory.EXPECT().
		OutboxRepository(s.tx).
		Return(s.storage).
		Once()

	s.storage.EXPECT().
		Insert(mock.Anything, mock.AnythingOfType("outbox.Event"), 5).
		Return(storageErr).
		Once()
}

func (s *SubscriptionEventPublisherSuite) TestPublish() {
	scenarios := []struct {
		name      string
		eventType string
		setup     func()
		act       func(*producers.SubscriptionEventPublisher, context.Context) error
	}{
		{
			name:      "deve publicar ativacao com token",
			eventType: producers.EventTypeSubscriptionActivated,
			setup: func() {
				s.expectInsertOnce(producers.EventTypeSubscriptionActivated)
			},
			act: func(publisher *producers.SubscriptionEventPublisher, ctx context.Context) error {
				return publisher.PublishActivated(ctx, s.tx, s.newActiveSubscription("token-unit-001"), s.subscriptionID, "token-unit-001", "+5511999999999", "user@example.com", "sale-001")
			},
		},
		{
			name:      "deve publicar ativacao sem token",
			eventType: producers.EventTypeSubscriptionActivatedWithoutToken,
			setup: func() {
				s.expectInsertOnce(producers.EventTypeSubscriptionActivatedWithoutToken)
			},
			act: func(publisher *producers.SubscriptionEventPublisher, ctx context.Context) error {
				return publisher.PublishActivatedWithoutToken(ctx, s.tx, s.newActiveSubscription("token-unit-002"), s.subscriptionID, "+5511999999999", "user@example.com", "sale-002")
			},
		},
		{
			name:      "deve publicar renovacao",
			eventType: producers.EventTypeSubscriptionRenewed,
			setup: func() {
				s.expectInsertOnce(producers.EventTypeSubscriptionRenewed)
			},
			act: func(publisher *producers.SubscriptionEventPublisher, ctx context.Context) error {
				return publisher.PublishRenewed(ctx, s.tx, s.newActiveSubscription("token-unit-003"), s.subscriptionID, s.occurredAt.Add(-30*24*time.Hour))
			},
		},
		{
			name:      "deve publicar atraso",
			eventType: producers.EventTypeSubscriptionPastDue,
			setup: func() {
				s.expectInsertOnce(producers.EventTypeSubscriptionPastDue)
			},
			act: func(publisher *producers.SubscriptionEventPublisher, ctx context.Context) error {
				return publisher.PublishPastDue(ctx, s.tx, s.newPastDueSubscription(), s.subscriptionID)
			},
		},
		{
			name:      "deve publicar cancelamento",
			eventType: producers.EventTypeSubscriptionCanceled,
			setup: func() {
				s.expectInsertOnce(producers.EventTypeSubscriptionCanceled)
			},
			act: func(publisher *producers.SubscriptionEventPublisher, ctx context.Context) error {
				return publisher.PublishCanceled(ctx, s.tx, s.newCanceledSubscription(), s.subscriptionID)
			},
		},
		{
			name:      "deve publicar reembolso",
			eventType: producers.EventTypeSubscriptionRefunded,
			setup: func() {
				s.expectInsertOnce(producers.EventTypeSubscriptionRefunded)
			},
			act: func(publisher *producers.SubscriptionEventPublisher, ctx context.Context) error {
				return publisher.PublishRefunded(ctx, s.tx, s.newRefundedSubscription(), s.subscriptionID)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()
			publisher := s.newPublisher()
			err := scenario.act(publisher, context.Background())
			s.NoError(err)
		})
	}
}

func (s *SubscriptionEventPublisherSuite) TestPublish_PropagatesError() {
	scenarios := []struct {
		name string
		act  func(*producers.SubscriptionEventPublisher, context.Context) error
	}{
		{
			name: "deve propagar erro ao publicar ativacao",
			act: func(publisher *producers.SubscriptionEventPublisher, ctx context.Context) error {
				return publisher.PublishActivated(ctx, s.tx, s.newActiveSubscription("token-unit-011"), s.subscriptionID, "token-unit-011", "+5511999999999", "user@example.com", "sale-011")
			},
		},
		{
			name: "deve propagar erro ao publicar ativacao sem token",
			act: func(publisher *producers.SubscriptionEventPublisher, ctx context.Context) error {
				return publisher.PublishActivatedWithoutToken(ctx, s.tx, s.newActiveSubscription("token-unit-012"), s.subscriptionID, "+5511999999999", "user@example.com", "sale-012")
			},
		},
		{
			name: "deve propagar erro ao publicar renovacao",
			act: func(publisher *producers.SubscriptionEventPublisher, ctx context.Context) error {
				return publisher.PublishRenewed(ctx, s.tx, s.newActiveSubscription("token-unit-013"), s.subscriptionID, s.occurredAt)
			},
		},
		{
			name: "deve propagar erro ao publicar atraso",
			act: func(publisher *producers.SubscriptionEventPublisher, ctx context.Context) error {
				return publisher.PublishPastDue(ctx, s.tx, s.newPastDueSubscription(), s.subscriptionID)
			},
		},
		{
			name: "deve propagar erro ao publicar cancelamento",
			act: func(publisher *producers.SubscriptionEventPublisher, ctx context.Context) error {
				return publisher.PublishCanceled(ctx, s.tx, s.newCanceledSubscription(), s.subscriptionID)
			},
		},
		{
			name: "deve propagar erro ao publicar reembolso",
			act: func(publisher *producers.SubscriptionEventPublisher, ctx context.Context) error {
				return publisher.PublishRefunded(ctx, s.tx, s.newRefundedSubscription(), s.subscriptionID)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			storageErr := errors.New("db failure")
			s.expectInsertError(storageErr)
			publisher := s.newPublisher()
			err := scenario.act(publisher, context.Background())
			s.ErrorContains(err, "billing/producer:")
			s.ErrorContains(err, "db failure")
		})
	}
}

func (s *SubscriptionEventPublisherSuite) TestPublish_SetsAggregateUserID() {
	expectedUserID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	ft, err := valueobjects.NewFunnelToken("token-user-001")
	s.Require().NoError(err)

	sub := entities.HydrateWithUser(
		s.subscriptionID,
		expectedUserID.String(),
		ft,
		plan,
		valueobjects.StatusActive,
		s.occurredAt,
		s.occurredAt.Add(30*24*time.Hour),
		time.Time{},
		s.occurredAt,
	)

	s.repoFactory.EXPECT().
		OutboxRepository(s.tx).
		Return(s.storage).
		Once()

	s.storage.EXPECT().
		Insert(mock.Anything, mock.MatchedBy(func(evt outbox.Event) bool {
			return evt.AggregateUserID == expectedUserID.String()
		}), 5).
		Return(nil).
		Once()

	publisher := s.newPublisher()
	pubErr := publisher.PublishActivated(context.Background(), s.tx, sub, s.subscriptionID, "token-user-001", "+5511999999999", "user@example.com", "sale-user-001")
	s.NoError(pubErr)
}

func (s *SubscriptionEventPublisherSuite) TestPublish_UsesSubscriptionOccurredAt() {
	sub := s.newPastDueSubscription()

	s.repoFactory.EXPECT().
		OutboxRepository(s.tx).
		Return(s.storage).
		Once()

	s.storage.EXPECT().
		Insert(mock.Anything, mock.MatchedBy(func(evt outbox.Event) bool {
			return evt.Type == producers.EventTypeSubscriptionPastDue &&
				evt.OccurredAt.Equal(sub.LastEventAt())
		}), 5).
		Return(nil).
		Once()

	publisher := s.newPublisher()
	pubErr := publisher.PublishPastDue(context.Background(), s.tx, sub, s.subscriptionID)
	s.NoError(pubErr)
}
