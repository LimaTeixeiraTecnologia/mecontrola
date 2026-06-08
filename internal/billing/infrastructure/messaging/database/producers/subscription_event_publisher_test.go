package producers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	dbmocks "github.com/JailtonJunior94/devkit-go/pkg/database/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

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
	publisher      *producers.SubscriptionEventPublisher
	sub            entities.Subscription
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

	cfg := configs.OutboxConfig{RetryMaxAttempts: 5}
	idGen := id.NewUUIDGenerator()

	s.publisher = producers.NewSubscriptionEventPublisher(s.repoFactory, cfg, idGen)

	s.subscriptionID = "sub-unit-001"
	s.occurredAt = time.Now().UTC().Truncate(time.Millisecond)

	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	ft, err := valueobjects.NewFunnelToken("token-unit-001")
	s.Require().NoError(err)

	sub := entities.NewSubscription(plan, ft)
	s.Require().NoError(sub.Activate(s.occurredAt))
	s.sub = sub
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

func (s *SubscriptionEventPublisherSuite) TestPublishActivated_CallsInsertExactlyOnce() {
	s.expectInsertOnce(producers.EventTypeSubscriptionActivated)
	ctx := context.Background()
	err := s.publisher.PublishActivated(ctx, s.tx, s.sub, s.subscriptionID, "token-unit-001", "+5511999999999", "user@example.com", "sale-001")
	s.NoError(err)
}

func (s *SubscriptionEventPublisherSuite) TestPublishActivated_PropagatesError() {
	storageErr := errors.New("db failure")
	s.expectInsertError(storageErr)
	ctx := context.Background()
	err := s.publisher.PublishActivated(ctx, s.tx, s.sub, s.subscriptionID, "token-unit-001", "+5511999999999", "user@example.com", "sale-001")
	s.ErrorContains(err, "billing/producer:")
	s.ErrorContains(err, "db failure")
}

func (s *SubscriptionEventPublisherSuite) TestPublishActivatedWithoutToken_CallsInsertExactlyOnce() {
	s.expectInsertOnce(producers.EventTypeSubscriptionActivatedWithoutToken)
	ctx := context.Background()
	err := s.publisher.PublishActivatedWithoutToken(ctx, s.tx, s.sub, s.subscriptionID, "+5511999999999", "user@example.com", "sale-002")
	s.NoError(err)
}

func (s *SubscriptionEventPublisherSuite) TestPublishActivatedWithoutToken_PropagatesError() {
	storageErr := errors.New("db failure")
	s.expectInsertError(storageErr)
	ctx := context.Background()
	err := s.publisher.PublishActivatedWithoutToken(ctx, s.tx, s.sub, s.subscriptionID, "+5511999999999", "user@example.com", "sale-002")
	s.ErrorContains(err, "billing/producer:")
	s.ErrorContains(err, "db failure")
}

func (s *SubscriptionEventPublisherSuite) TestPublishRenewed_CallsInsertExactlyOnce() {
	s.expectInsertOnce(producers.EventTypeSubscriptionRenewed)
	ctx := context.Background()
	previousEnd := s.occurredAt.Add(-30 * 24 * time.Hour)
	err := s.publisher.PublishRenewed(ctx, s.tx, s.sub, s.subscriptionID, previousEnd)
	s.NoError(err)
}

func (s *SubscriptionEventPublisherSuite) TestPublishRenewed_PropagatesError() {
	storageErr := errors.New("db failure")
	s.expectInsertError(storageErr)
	ctx := context.Background()
	err := s.publisher.PublishRenewed(ctx, s.tx, s.sub, s.subscriptionID, s.occurredAt)
	s.ErrorContains(err, "billing/producer:")
}

func (s *SubscriptionEventPublisherSuite) TestPublishPastDue_CallsInsertExactlyOnce() {
	plan, _ := valueobjects.NewPlan("MONTHLY", 30)
	ft, _ := valueobjects.NewFunnelToken("token-pastdue-001")
	sub := entities.NewSubscription(plan, ft)
	s.Require().NoError(sub.Activate(s.occurredAt))
	s.Require().NoError(sub.MarkPastDue(s.occurredAt.Add(time.Hour), 3*24*time.Hour))

	s.repoFactory.EXPECT().OutboxRepository(s.tx).Return(s.storage).Once()
	s.storage.EXPECT().
		Insert(mock.Anything, mock.MatchedBy(func(evt outbox.Event) bool {
			return evt.Type == producers.EventTypeSubscriptionPastDue
		}), 5).
		Return(nil).Once()

	ctx := context.Background()
	err := s.publisher.PublishPastDue(ctx, s.tx, sub, s.subscriptionID)
	s.NoError(err)
}

func (s *SubscriptionEventPublisherSuite) TestPublishPastDue_PropagatesError() {
	storageErr := errors.New("db failure")
	s.expectInsertError(storageErr)
	ctx := context.Background()
	err := s.publisher.PublishPastDue(ctx, s.tx, s.sub, s.subscriptionID)
	s.ErrorContains(err, "billing/producer:")
}

func (s *SubscriptionEventPublisherSuite) TestPublishCanceled_CallsInsertExactlyOnce() {
	plan, _ := valueobjects.NewPlan("MONTHLY", 30)
	ft, _ := valueobjects.NewFunnelToken("token-canceled-001")
	sub := entities.NewSubscription(plan, ft)
	s.Require().NoError(sub.Activate(s.occurredAt))
	s.Require().NoError(sub.MarkCanceled(s.occurredAt.Add(time.Hour)))

	s.repoFactory.EXPECT().OutboxRepository(s.tx).Return(s.storage).Once()
	s.storage.EXPECT().
		Insert(mock.Anything, mock.MatchedBy(func(evt outbox.Event) bool {
			return evt.Type == producers.EventTypeSubscriptionCanceled
		}), 5).
		Return(nil).Once()

	ctx := context.Background()
	err := s.publisher.PublishCanceled(ctx, s.tx, sub, s.subscriptionID)
	s.NoError(err)
}

func (s *SubscriptionEventPublisherSuite) TestPublishCanceled_PropagatesError() {
	storageErr := errors.New("db failure")
	s.expectInsertError(storageErr)
	ctx := context.Background()
	err := s.publisher.PublishCanceled(ctx, s.tx, s.sub, s.subscriptionID)
	s.ErrorContains(err, "billing/producer:")
}

func (s *SubscriptionEventPublisherSuite) TestPublishRefunded_CallsInsertExactlyOnce() {
	plan, _ := valueobjects.NewPlan("MONTHLY", 30)
	ft, _ := valueobjects.NewFunnelToken("token-refunded-001")
	sub := entities.NewSubscription(plan, ft)
	s.Require().NoError(sub.Activate(s.occurredAt))
	s.Require().NoError(sub.MarkRefunded(s.occurredAt.Add(time.Hour)))

	s.repoFactory.EXPECT().OutboxRepository(s.tx).Return(s.storage).Once()
	s.storage.EXPECT().
		Insert(mock.Anything, mock.MatchedBy(func(evt outbox.Event) bool {
			return evt.Type == producers.EventTypeSubscriptionRefunded
		}), 5).
		Return(nil).Once()

	ctx := context.Background()
	err := s.publisher.PublishRefunded(ctx, s.tx, sub, s.subscriptionID)
	s.NoError(err)
}

func (s *SubscriptionEventPublisherSuite) TestPublishRefunded_PropagatesError() {
	storageErr := errors.New("db failure")
	s.expectInsertError(storageErr)
	ctx := context.Background()
	err := s.publisher.PublishRefunded(ctx, s.tx, s.sub, s.subscriptionID)
	s.ErrorContains(err, "billing/producer:")
}
