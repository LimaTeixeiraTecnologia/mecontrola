//go:build integration

package consumers_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/messaging/database/consumers"
	cardrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type stubChannelGateway struct {
	mu    sync.Mutex
	calls int
}

func (s *stubChannelGateway) SendText(_ context.Context, _, _, _ string) error {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	return nil
}

func (s *stubChannelGateway) SendActivationTemplate(_ context.Context, _, _, _, _ string) (string, error) {
	return "", nil
}

func (s *stubChannelGateway) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type stubResolver struct {
	pref interfaces.UserChannelPreference
	ok   bool
	err  error
}

func (r *stubResolver) ResolvePreferred(_ context.Context, _ uuid.UUID) (interfaces.UserChannelPreference, bool, error) {
	return r.pref, r.ok, r.err
}

type InvoiceDueNotifierIntegrationSuite struct {
	suite.Suite
	db *sqlx.DB
}

func TestInvoiceDueNotifierIntegration(t *testing.T) {
	suite.Run(t, new(InvoiceDueNotifierIntegrationSuite))
}

func (s *InvoiceDueNotifierIntegrationSuite) SetupSuite() {
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
}

func (s *InvoiceDueNotifierIntegrationSuite) buildConsumer(gateway notification.ChannelGateway, resolver interfaces.UserChannelResolver) *consumers.InvoiceDueNotifier {
	o11y := noop.NewProvider()
	factory := cardrepo.NewRepositoryFactory(o11y)
	alertRepo := factory.InvoiceDueAlertSentRepository(s.db)
	loc := time.UTC
	uc := usecases.NewNotifyInvoiceDue(alertRepo, resolver, gateway, loc, o11y)
	return consumers.NewInvoiceDueNotifier(uc, o11y)
}

func (s *InvoiceDueNotifierIntegrationSuite) buildEnvelope(userID, cardID uuid.UUID, dueDate time.Time) outbox.Envelope {
	payload := map[string]any{
		"user_id":       userID.String(),
		"card_id":       cardID.String(),
		"card_nickname": "Nubank",
		"due_date":      dueDate.Format("2006-01-02"),
		"days_until":    3,
	}
	raw, _ := json.Marshal(payload)
	return outbox.Envelope{
		ID:        uuid.New().String(),
		EventType: "card.invoice_due.v1",
		Payload:   raw,
	}
}

func (s *InvoiceDueNotifierIntegrationSuite) TestInvoiceDueNotifier_Handle_MarksNotifiedAtInDB() {
	ctx := context.Background()
	userID := uuid.New()
	cardID := uuid.New()
	dueDate := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, 3)

	insertUser(s.T(), s.db, userID)
	insertCard(s.T(), s.db, cardID, userID)
	insertAlertSentPending(s.T(), s.db, userID, cardID, dueDate)

	gateway := &stubChannelGateway{}
	resolver := &stubResolver{
		pref: interfaces.UserChannelPreference{Channel: "whatsapp", ExternalID: "+5511999990000"},
		ok:   true,
	}

	consumer := s.buildConsumer(gateway, resolver)
	env := s.buildEnvelope(userID, cardID, dueDate)

	err := consumer.Handle(ctx, stubEvent{eventType: "card.invoice_due.v1", payload: env})
	s.Require().NoError(err)

	s.True(isAlertNotified(s.T(), s.db, userID, cardID, dueDate))
	s.Equal(1, gateway.callCount())
}

func (s *InvoiceDueNotifierIntegrationSuite) TestInvoiceDueNotifier_Handle_AlreadyNotified_DoesNotCallGatewayAgain() {
	ctx := context.Background()
	userID := uuid.New()
	cardID := uuid.New()
	dueDate := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, 5)

	insertUser(s.T(), s.db, userID)
	insertCard(s.T(), s.db, cardID, userID)
	insertAlertSentPending(s.T(), s.db, userID, cardID, dueDate)

	gateway := &stubChannelGateway{}
	resolver := &stubResolver{
		pref: interfaces.UserChannelPreference{Channel: "whatsapp", ExternalID: "+5511999990001"},
		ok:   true,
	}

	consumer := s.buildConsumer(gateway, resolver)
	env := s.buildEnvelope(userID, cardID, dueDate)

	err := consumer.Handle(ctx, stubEvent{eventType: "card.invoice_due.v1", payload: env})
	s.Require().NoError(err)

	err = consumer.Handle(ctx, stubEvent{eventType: "card.invoice_due.v1", payload: env})
	s.Require().NoError(err)

	s.Equal(1, gateway.callCount())
}

func (s *InvoiceDueNotifierIntegrationSuite) TestInvoiceDueNotifier_Handle_UserWithNoChannel_OutcomeNoChannel() {
	ctx := context.Background()
	userID := uuid.New()
	cardID := uuid.New()
	dueDate := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, 7)

	insertUser(s.T(), s.db, userID)
	insertCard(s.T(), s.db, cardID, userID)
	insertAlertSentPending(s.T(), s.db, userID, cardID, dueDate)

	gateway := &stubChannelGateway{}
	resolver := &stubResolver{ok: false}

	consumer := s.buildConsumer(gateway, resolver)
	env := s.buildEnvelope(userID, cardID, dueDate)

	err := consumer.Handle(ctx, stubEvent{eventType: "card.invoice_due.v1", payload: env})
	s.Require().NoError(err)

	s.Equal(0, gateway.callCount())
}

func insertUser(t *testing.T, db *sqlx.DB, userID uuid.UUID) {
	t.Helper()
	const q = `
		INSERT INTO mecontrola.users (id, whatsapp_number, status)
		VALUES ($1, $2, 'ACTIVE')
	`
	number := "+551" + userID.String()[:10]
	_, err := db.ExecContext(context.Background(), q, userID, number)
	if err != nil {
		t.Fatalf("insertUser: %v", err)
	}
}

func insertCard(t *testing.T, db *sqlx.DB, cardID, userID uuid.UUID) {
	t.Helper()
	const q = `
		INSERT INTO mecontrola.cards (id, user_id, nickname, bank, closing_day, due_day, version)
		VALUES ($1, $2, 'testcard', 'nubank', 5, 10, 1)
	`
	_, err := db.ExecContext(context.Background(), q, cardID, userID)
	if err != nil {
		t.Fatalf("insertCard: %v", err)
	}
}

func insertAlertSentPending(t *testing.T, db *sqlx.DB, userID, cardID uuid.UUID, refDueDate time.Time) {
	t.Helper()
	const q = `
		INSERT INTO mecontrola.card_invoice_alerts_sent (user_id, card_id, ref_due_date)
		VALUES ($1, $2, $3::date)
		ON CONFLICT (user_id, card_id, ref_due_date) DO NOTHING
	`
	day := refDueDate.UTC().Truncate(24 * time.Hour)
	_, err := db.ExecContext(context.Background(), q, userID, cardID, day)
	if err != nil {
		t.Fatalf("insertAlertSentPending: %v", err)
	}
}

func isAlertNotified(t *testing.T, db *sqlx.DB, userID, cardID uuid.UUID, refDueDate time.Time) bool {
	t.Helper()
	const q = `
		SELECT notified_at IS NOT NULL
		  FROM mecontrola.card_invoice_alerts_sent
		 WHERE card_id = $1 AND user_id = $2 AND ref_due_date = $3::date
		 LIMIT 1
	`
	day := refDueDate.UTC().Truncate(24 * time.Hour)
	var notified bool
	err := db.QueryRowContext(context.Background(), q, cardID, userID, day).Scan(&notified)
	if err != nil {
		t.Fatalf("isAlertNotified: %v", err)
	}
	return notified
}
