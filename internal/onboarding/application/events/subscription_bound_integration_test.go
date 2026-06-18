//go:build integration

package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type SubscriptionBoundIntegrationSuite struct {
	suite.Suite
}

func TestSubscriptionBoundIntegrationSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionBoundIntegrationSuite))
}

func (s *SubscriptionBoundIntegrationSuite) newDomainEvt(eventID, tokenID, userID, subID string) entities.SubscriptionBound {
	return entities.SubscriptionBound{
		EventID:         eventID,
		TokenID:         tokenID,
		UserID:          userID,
		SubscriptionID:  subID,
		TokenHashPrefix: "abc",
		ActivationPath:  valueobjects.ActivationPathDirect,
		BoundAt:         time.Now().UTC().Truncate(time.Microsecond),
	}
}

func (s *SubscriptionBoundIntegrationSuite) countRows(db *sqlx.DB, eventID string) int {
	ctx := context.Background()
	var count int
	s.Require().NoError(db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mecontrola.outbox_events WHERE id = $1`, eventID).Scan(&count))
	return count
}

func (s *SubscriptionBoundIntegrationSuite) publishInTx(db *sqlx.DB, evt outbox.Event) error {
	ctx := context.Background()
	cfg := configs.OutboxConfig{RetryMaxAttempts: 3}
	tx, err := db.BeginTx(ctx, nil)
	s.Require().NoError(err)
	pubErr := outbox.NewPostgresPublisher(outbox.NewPostgresStorage(tx), cfg).Publish(ctx, evt)
	if pubErr != nil {
		_ = tx.Rollback()
		return pubErr
	}
	return tx.Commit()
}

func (s *SubscriptionBoundIntegrationSuite) publishInTxRollback(db *sqlx.DB, evt outbox.Event) {
	ctx := context.Background()
	cfg := configs.OutboxConfig{RetryMaxAttempts: 3}
	tx, err := db.BeginTx(ctx, nil)
	s.Require().NoError(err)
	s.Require().NoError(outbox.NewPostgresPublisher(outbox.NewPostgresStorage(tx), cfg).Publish(ctx, evt))
	s.Require().NoError(tx.Rollback())
}

func (s *SubscriptionBoundIntegrationSuite) TestPublish_InsertsCorrectly() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	eventID := uuid.NewString()
	tokenID := uuid.NewString()
	userID := uuid.NewString()
	subID := uuid.NewString()

	domainEvt := s.newDomainEvt(eventID, tokenID, userID, subID)
	evt, err := events.NewSubscriptionBoundEvent(domainEvt)
	s.Require().NoError(err)

	s.Require().NoError(s.publishInTx(db, evt))

	s.Equal(1, s.countRows(db, eventID))

	var eventType, aggregateType, aggregateID, aggregateUserID string
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT event_type, aggregate_type, aggregate_id, aggregate_user_id FROM mecontrola.outbox_events WHERE id = $1`,
		eventID,
	).Scan(&eventType, &aggregateType, &aggregateID, &aggregateUserID))

	s.Equal("onboarding.subscription_bound", eventType)
	s.Equal("onboarding_token", aggregateType)
	s.Equal(tokenID, aggregateID)
	s.Equal(userID, aggregateUserID)
}

func (s *SubscriptionBoundIntegrationSuite) TestPublish_RollbackDoesNotPersist() {
	db, _ := testcontainer.Postgres(s.T())

	eventID := uuid.NewString()
	tokenID := uuid.NewString()
	userID := uuid.NewString()
	subID := uuid.NewString()

	domainEvt := s.newDomainEvt(eventID, tokenID, userID, subID)
	evt, err := events.NewSubscriptionBoundEvent(domainEvt)
	s.Require().NoError(err)

	s.publishInTxRollback(db, evt)

	s.Equal(0, s.countRows(db, eventID))
}

func (s *SubscriptionBoundIntegrationSuite) TestPublish_IdempotentSameEventID() {
	db, _ := testcontainer.Postgres(s.T())

	eventID := uuid.NewString()
	tokenID := uuid.NewString()
	userID := uuid.NewString()
	subID := uuid.NewString()

	domainEvt := s.newDomainEvt(eventID, tokenID, userID, subID)
	evt, err := events.NewSubscriptionBoundEvent(domainEvt)
	s.Require().NoError(err)

	s.Require().NoError(s.publishInTx(db, evt))
	s.Require().NoError(s.publishInTx(db, evt))

	s.Equal(1, s.countRows(db, eventID))
}
