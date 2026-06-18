//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

func shortCardName(nome string) string {
	runes := []rune(nome)
	if len(runes) > 12 {
		runes = runes[:12]
	}
	return fmt.Sprintf("%s-%s", string(runes), uuid.NewString()[:8])
}

type cardConsumerEvent struct {
	eventType string
	payload   any
}

func (ev *cardConsumerEvent) GetEventType() string { return ev.eventType }
func (ev *cardConsumerEvent) GetPayload() any      { return ev.payload }

func registerConsumerSteps(sc *godog.ScenarioContext, e *cardE2ECtx) {
	sc.Step(`^o consumer recebe o evento "([^"]*)" com nome "([^"]*)", limite (\d+), fechamento (\d+) e vencimento (\d+)$`, e.consumerReceivesOnboardingEvent)
	sc.Step(`^o cartão deve estar persistido no banco para o usuário$`, e.assertCardPersistedForUser)
	sc.Step(`^o consumer recebe o evento "([^"]*)" para o cartão criado com vencimento em (\d+) dias$`, e.consumerReceivesInvoiceDueEvent)
	sc.Step(`^a gateway de canal deve ter recebido ao menos 1 mensagem para o usuário$`, e.assertGatewayReceivedMessage)
	sc.Step(`^o mesmo evento de onboarding é reprocessado com o mesmo event_id$`, e.reprocessSameOnboardingEvent)
	sc.Step(`^o banco deve conter exatamente (\d+) cartão com aquele nome para o usuário$`, e.assertExactCardCountByName)
	sc.Step(`^o mesmo evento de vencimento é reprocessado$`, e.reprocessSameInvoiceDueEvent)
	sc.Step(`^a gateway de canal deve ter recebido exatamente (\d+) mensagem para o usuário$`, e.assertGatewayReceivedExactMessages)
	sc.Step(`^existe um registro de alerta pendente para o cartão com vencimento em (\d+) dias$`, e.insertPendingAlertForCard)
}

func (e *cardE2ECtx) consumerReceivesOnboardingEvent(_ string, nome string, limite int64, fechamento, vencimento int) error {
	h, ok := e.eventHandlers["onboarding.card_registered"]
	if !ok {
		return fmt.Errorf("handler para onboarding.card_registered nao registrado")
	}

	cardName := shortCardName(nome)
	e.cardName = cardName

	raw, err := json.Marshal(struct {
		UserID     string `json:"UserID"`
		Name       string `json:"Name"`
		LimitCents int64  `json:"LimitCents"`
		ClosingDay int    `json:"ClosingDay"`
		DueDay     int    `json:"DueDay"`
	}{
		UserID:     e.userID.String(),
		Name:       cardName,
		LimitCents: limite,
		ClosingDay: fechamento,
		DueDay:     vencimento,
	})
	if err != nil {
		return fmt.Errorf("marshal payload onboarding: %w", err)
	}

	eventID := uuid.NewString()
	e.lastOnboardingEventID = eventID

	envelope := outbox.Envelope{
		ID:              eventID,
		EventType:       "onboarding.card_registered",
		AggregateUserID: e.userID.String(),
		OccurredAt:      time.Now().UTC(),
		Payload:         json.RawMessage(raw),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return h.Handle(ctx, &cardConsumerEvent{eventType: "onboarding.card_registered", payload: envelope})
}

func (e *cardE2ECtx) assertCardPersistedForUser() error {
	if e.cardName == "" {
		return fmt.Errorf("cardName nao definido")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	err := e.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM mecontrola.cards WHERE name = $1 AND user_id = $2 AND deleted_at IS NULL`,
		e.cardName,
		e.userID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("consultar cartao no banco: %w", err)
	}

	if count < 1 {
		return fmt.Errorf("cartao %q nao encontrado no banco para o usuario %s", e.cardName, e.userID)
	}

	return nil
}

func (e *cardE2ECtx) consumerReceivesInvoiceDueEvent(_ string, daysUntil int) error {
	h, ok := e.eventHandlers["card.invoice_due.v1"]
	if !ok {
		return fmt.Errorf("handler para card.invoice_due.v1 nao registrado")
	}

	if e.cardID == "" {
		return fmt.Errorf("cardID nao definido")
	}

	dueDateVal := e.expectedDueDate
	if dueDateVal.IsZero() {
		dueDateVal = time.Now().UTC().AddDate(0, 0, daysUntil)
		e.expectedDueDate = dueDateVal
	}
	dueDate := dueDateVal.Format("2006-01-02")

	raw, err := json.Marshal(struct {
		UserID     string `json:"user_id"`
		CardID     string `json:"card_id"`
		CardName   string `json:"card_name"`
		LimitCents int64  `json:"limit_cents"`
		DueDate    string `json:"due_date"`
		DaysUntil  int    `json:"days_until"`
	}{
		UserID:     e.userID.String(),
		CardID:     e.cardID,
		CardName:   e.cardName,
		LimitCents: 100000,
		DueDate:    dueDate,
		DaysUntil:  daysUntil,
	})
	if err != nil {
		return fmt.Errorf("marshal payload invoice_due: %w", err)
	}

	eventID := uuid.NewString()
	e.lastInvoiceDueEventID = eventID

	envelope := outbox.Envelope{
		ID:              eventID,
		EventType:       "card.invoice_due.v1",
		AggregateUserID: e.userID.String(),
		OccurredAt:      time.Now().UTC(),
		Payload:         json.RawMessage(raw),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return h.Handle(ctx, &cardConsumerEvent{eventType: "card.invoice_due.v1", payload: envelope})
}

func (e *cardE2ECtx) assertGatewayReceivedMessage() error {
	e.channelGateway.mu.Lock()
	defer e.channelGateway.mu.Unlock()

	for _, msg := range e.channelGateway.messages {
		if msg.ExternalID == e2eUserPhone {
			return nil
		}
	}

	return fmt.Errorf("gateway nao recebeu mensagem para o usuario com phone %q", e2eUserPhone)
}

func (e *cardE2ECtx) reprocessSameOnboardingEvent() error {
	h, ok := e.eventHandlers["onboarding.card_registered"]
	if !ok {
		return fmt.Errorf("handler para onboarding.card_registered nao registrado")
	}

	if e.lastOnboardingEventID == "" {
		return fmt.Errorf("lastOnboardingEventID nao definido")
	}

	raw, err := json.Marshal(struct {
		UserID     string `json:"UserID"`
		Name       string `json:"Name"`
		LimitCents int64  `json:"LimitCents"`
		ClosingDay int    `json:"ClosingDay"`
		DueDay     int    `json:"DueDay"`
	}{
		UserID:     e.userID.String(),
		Name:       e.cardName,
		LimitCents: 50000,
		ClosingDay: 3,
		DueDay:     10,
	})
	if err != nil {
		return fmt.Errorf("marshal payload reprocess onboarding: %w", err)
	}

	envelope := outbox.Envelope{
		ID:              e.lastOnboardingEventID,
		EventType:       "onboarding.card_registered",
		AggregateUserID: e.userID.String(),
		OccurredAt:      time.Now().UTC(),
		Payload:         json.RawMessage(raw),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = h.Handle(ctx, &cardConsumerEvent{eventType: "onboarding.card_registered", payload: envelope})
	return nil
}

func (e *cardE2ECtx) assertExactCardCountByName(expected int) error {
	if e.cardName == "" {
		return fmt.Errorf("cardName nao definido")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	err := e.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM mecontrola.cards WHERE name = $1 AND user_id = $2 AND deleted_at IS NULL`,
		e.cardName,
		e.userID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("consultar cartao no banco: %w", err)
	}

	if count != expected {
		return fmt.Errorf("esperado %d cartao(s) com nome %q, encontrado %d", expected, e.cardName, count)
	}

	return nil
}

func (e *cardE2ECtx) reprocessSameInvoiceDueEvent() error {
	h, ok := e.eventHandlers["card.invoice_due.v1"]
	if !ok {
		return fmt.Errorf("handler para card.invoice_due.v1 nao registrado")
	}

	if e.lastInvoiceDueEventID == "" {
		return fmt.Errorf("lastInvoiceDueEventID nao definido")
	}

	if e.cardID == "" {
		return fmt.Errorf("cardID nao definido")
	}

	dueDate := e.expectedDueDate.Format("2006-01-02")

	raw, err := json.Marshal(struct {
		UserID     string `json:"user_id"`
		CardID     string `json:"card_id"`
		CardName   string `json:"card_name"`
		LimitCents int64  `json:"limit_cents"`
		DueDate    string `json:"due_date"`
		DaysUntil  int    `json:"days_until"`
	}{
		UserID:     e.userID.String(),
		CardID:     e.cardID,
		CardName:   e.cardName,
		LimitCents: 100000,
		DueDate:    dueDate,
		DaysUntil:  e.expectedDaysUntil,
	})
	if err != nil {
		return fmt.Errorf("marshal payload reprocess invoice_due: %w", err)
	}

	envelope := outbox.Envelope{
		ID:              e.lastInvoiceDueEventID,
		EventType:       "card.invoice_due.v1",
		AggregateUserID: e.userID.String(),
		OccurredAt:      time.Now().UTC(),
		Payload:         json.RawMessage(raw),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = h.Handle(ctx, &cardConsumerEvent{eventType: "card.invoice_due.v1", payload: envelope})
	return nil
}

func (e *cardE2ECtx) assertGatewayReceivedExactMessages(expected int) error {
	e.channelGateway.mu.Lock()
	defer e.channelGateway.mu.Unlock()

	count := 0
	for _, msg := range e.channelGateway.messages {
		if msg.ExternalID == e2eUserPhone {
			count++
		}
	}

	if count != expected {
		return fmt.Errorf("gateway recebeu %d mensagens, esperado %d", count, expected)
	}

	return nil
}

func (e *cardE2ECtx) insertPendingAlertForCard(daysUntil int) error {
	if e.cardID == "" {
		return fmt.Errorf("cardID nao definido")
	}

	dueDate := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, daysUntil)
	e.expectedDueDate = dueDate

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := e.db.ExecContext(ctx,
		`INSERT INTO mecontrola.card_invoice_alerts_sent (user_id, card_id, ref_due_date)
		 VALUES ($1, $2, $3::date)
		 ON CONFLICT (user_id, card_id, ref_due_date) DO NOTHING`,
		e.userID.String(), e.cardID, dueDate,
	)
	if err != nil {
		return fmt.Errorf("inserir alerta pendente: %w", err)
	}

	return nil
}
