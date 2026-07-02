//go:build integration

package producers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
	producers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/messaging/database/producers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type InvoiceDuePublisherSuite struct {
	suite.Suite
}

func TestInvoiceDuePublisherSuite(t *testing.T) {
	suite.Run(t, new(InvoiceDuePublisherSuite))
}

func invoiceDueOutboxCfg() configs.OutboxConfig {
	return configs.OutboxConfig{RetryMaxAttempts: 3}
}

func newInvoiceDuePublisher() *producers.InvoiceDuePublisher {
	outboxFactory := outbox.NewRepositoryFactory(noop.NewProvider())
	return producers.NewInvoiceDuePublisher(outboxFactory, invoiceDueOutboxCfg(), id.NewUUIDGenerator(), noop.NewProvider())
}

func seedUserAndCard(t *testing.T, db *sqlx.DB) (userID, cardID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	userID = uuid.New()
	cardID = uuid.New()
	_, err := db.ExecContext(ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, status) VALUES ($1, $2, 'ACTIVE')`,
		userID.String(), "+5511999"+userID.String()[:8],
	)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO mecontrola.cards (id, user_id, nickname, bank, closing_day, due_day)
		 VALUES ($1, $2, 'test', 'nubank', 10, 20)`,
		cardID.String(), userID.String(),
	)
	if err != nil {
		t.Fatalf("seed card: %v", err)
	}
	return userID, cardID
}

func buildAlert(userID, cardID uuid.UUID) services.InvoiceDueAlert {
	return services.InvoiceDueAlert{
		UserID:       userID,
		CardID:       cardID,
		CardNickname: "Test Card",
		DueDate:      time.Now().UTC().AddDate(0, 0, 3),
		DaysUntil:    3,
	}
}

func countOutboxByTypeAndCard(t *testing.T, db *sqlx.DB, eventType, cardID, userID string) int {
	t.Helper()
	var count int
	err := db.QueryRowContext(
		context.Background(),
		`SELECT COUNT(*) FROM mecontrola.outbox_events
		 WHERE event_type = $1
		   AND aggregate_id = $2
		   AND aggregate_user_id = $3`,
		eventType, cardID, userID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count outbox rows: %v", err)
	}
	return count
}

func (s *InvoiceDuePublisherSuite) TestInvoiceDuePublisher_Publish_CommitPersistsOutboxRow() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	userID, cardID := seedUserAndCard(s.T(), db)
	alert := buildAlert(userID, cardID)
	now := time.Now().UTC()

	publisher := newInvoiceDuePublisher()

	tx, err := db.BeginTx(ctx, nil)
	s.Require().NoError(err)
	s.Require().NoError(publisher.Publish(ctx, tx, alert, now))
	s.Require().NoError(tx.Commit())

	count := countOutboxByTypeAndCard(s.T(), db, "card.invoice_due.v1", cardID.String(), userID.String())
	s.Equal(1, count)
}

func (s *InvoiceDuePublisherSuite) TestInvoiceDuePublisher_Publish_RollbackDoesNotPersistEvent() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	userID, cardID := seedUserAndCard(s.T(), db)
	alert := buildAlert(userID, cardID)
	now := time.Now().UTC()

	publisher := newInvoiceDuePublisher()

	tx, err := db.BeginTx(ctx, nil)
	s.Require().NoError(err)
	s.Require().NoError(publisher.Publish(ctx, tx, alert, now))
	s.Require().NoError(tx.Rollback())

	count := countOutboxByTypeAndCard(s.T(), db, "card.invoice_due.v1", cardID.String(), userID.String())
	s.Equal(0, count)
}

func (s *InvoiceDuePublisherSuite) TestInvoiceDuePublisher_Publish_PayloadContainsRequiredFields() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	userID, cardID := seedUserAndCard(s.T(), db)
	alert := buildAlert(userID, cardID)
	now := time.Now().UTC()

	publisher := newInvoiceDuePublisher()

	tx, err := db.BeginTx(ctx, nil)
	s.Require().NoError(err)
	s.Require().NoError(publisher.Publish(ctx, tx, alert, now))
	s.Require().NoError(tx.Commit())

	var raw []byte
	err = db.QueryRowContext(ctx,
		`SELECT payload FROM mecontrola.outbox_events
		 WHERE event_type = $1 AND aggregate_id = $2 AND aggregate_user_id = $3`,
		"card.invoice_due.v1", cardID.String(), userID.String(),
	).Scan(&raw)
	s.Require().NoError(err)

	var payload map[string]any
	s.Require().NoError(json.Unmarshal(raw, &payload))

	s.Contains(payload, "card_id")
	s.Contains(payload, "user_id")
	s.Contains(payload, "due_date")
	s.Contains(payload, "days_until")
}
