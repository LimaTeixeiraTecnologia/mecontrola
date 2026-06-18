//go:build integration

package producers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/producers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type ThresholdAlertPublisherSuite struct {
	suite.Suite
}

func TestThresholdAlertPublisherSuite(t *testing.T) {
	suite.Run(t, new(ThresholdAlertPublisherSuite))
}

func newThresholdAlertPublisher() *producers.ThresholdAlertPublisher {
	outboxFactory := outbox.NewRepositoryFactory(noop.NewProvider())
	cfg := configs.OutboxConfig{RetryMaxAttempts: 3}
	return producers.NewThresholdAlertPublisher(outboxFactory, cfg, id.NewUUIDGenerator(), noop.NewProvider())
}

func buildDomainAlert(userID, budgetID uuid.UUID) services.DomainAlert {
	rootSlug, _ := valueobjects.ParseRootSlug("expense.prazeres")
	return services.DomainAlert{
		UserID:               userID,
		BudgetID:             budgetID,
		Kind:                 services.ThresholdAlertCategory,
		CategoryID:           uuid.New(),
		CardID:               uuid.Nil,
		RootSlug:             rootSlug,
		PercentUsedBps:       8500,
		AmountRemainingCents: 15000,
		RefDay:               time.Date(2026, time.June, 18, 0, 0, 0, 0, time.UTC),
	}
}

func (s *ThresholdAlertPublisherSuite) TestPublish_CommitPersistsOutboxRow() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	userID := uuid.New()
	budgetID := uuid.New()
	alert := buildDomainAlert(userID, budgetID)
	now := time.Now().UTC()

	publisher := newThresholdAlertPublisher()

	tx, err := db.BeginTx(ctx, nil)
	s.Require().NoError(err)
	s.Require().NoError(publisher.Publish(ctx, tx, alert, now))
	s.Require().NoError(tx.Commit())

	var count int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.outbox_events WHERE event_type = $1 AND aggregate_user_id = $2`,
		"budgets.threshold_alert_triggered.v1", userID.String(),
	).Scan(&count))
	s.Equal(1, count)
}

func (s *ThresholdAlertPublisherSuite) TestPublish_RollbackDoesNotPersistRow() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	userID := uuid.New()
	budgetID := uuid.New()
	alert := buildDomainAlert(userID, budgetID)
	now := time.Now().UTC()

	publisher := newThresholdAlertPublisher()

	tx, err := db.BeginTx(ctx, nil)
	s.Require().NoError(err)
	s.Require().NoError(publisher.Publish(ctx, tx, alert, now))
	s.Require().NoError(tx.Rollback())

	var count int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.outbox_events WHERE event_type = $1 AND aggregate_user_id = $2`,
		"budgets.threshold_alert_triggered.v1", userID.String(),
	).Scan(&count))
	s.Equal(0, count)
}

func (s *ThresholdAlertPublisherSuite) TestPublish_TwiceWithDifferentIDsGeneratesTwoRows() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	userID := uuid.New()
	budgetID := uuid.New()
	alert := buildDomainAlert(userID, budgetID)
	now := time.Now().UTC()

	publisher := newThresholdAlertPublisher()

	tx1, err := db.BeginTx(ctx, nil)
	s.Require().NoError(err)
	s.Require().NoError(publisher.Publish(ctx, tx1, alert, now))
	s.Require().NoError(tx1.Commit())

	tx2, err := db.BeginTx(ctx, nil)
	s.Require().NoError(err)
	s.Require().NoError(publisher.Publish(ctx, tx2, alert, now))
	s.Require().NoError(tx2.Commit())

	var count int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.outbox_events WHERE event_type = $1 AND aggregate_user_id = $2`,
		"budgets.threshold_alert_triggered.v1", userID.String(),
	).Scan(&count))
	s.Equal(2, count)
}

func (s *ThresholdAlertPublisherSuite) TestPublish_PayloadContainsRequiredFields() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	userID := uuid.New()
	budgetID := uuid.New()
	alert := buildDomainAlert(userID, budgetID)
	now := time.Now().UTC()

	publisher := newThresholdAlertPublisher()

	tx, err := db.BeginTx(ctx, nil)
	s.Require().NoError(err)
	s.Require().NoError(publisher.Publish(ctx, tx, alert, now))
	s.Require().NoError(tx.Commit())

	var raw []byte
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT payload FROM mecontrola.outbox_events WHERE event_type = $1 AND aggregate_user_id = $2`,
		"budgets.threshold_alert_triggered.v1", userID.String(),
	).Scan(&raw))

	var payload map[string]any
	s.Require().NoError(json.Unmarshal(raw, &payload))
	s.Contains(payload, "user_id")
	s.Contains(payload, "budget_id")
	s.Contains(payload, "kind")
	s.Contains(payload, "ref_day")
}
