//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

func registerInvoiceSteps(sc *godog.ScenarioContext, e *cardE2ECtx) {
	sc.Step(`^o usuário consulta a fatura do cartão para a data "([^"]*)"$`, e.getInvoiceForDate)
	sc.Step(`^o usuário consulta a fatura do cartão sem informar a data$`, e.getInvoiceWithoutDate)
	sc.Step(`^o campo "closing_date" da fatura deve ser "([^"]*)"$`, e.assertClosingDate)
	sc.Step(`^o campo "due_date" da fatura deve ser "([^"]*)"$`, e.assertDueDate)
	sc.Step(`^o usuário consulta a fatura de um cartão com ID aleatório inexistente para a data "([^"]*)"$`, e.getInvoiceForNonExistentCard)
	sc.Step(`^o usuário consulta a fatura do cartão pelo ID cadastrado para a data "([^"]*)"$`, e.getInvoiceForRegisteredCard)
	sc.Step(`^o usuário possui um cartão com fatura vencendo em (\d+) dias$`, e.cardWithInvoiceDueInDays)
	sc.Step(`^o worker de alertas de fatura é executado$`, e.workerDeAlertasEExecutado)
	sc.Step(`^deve existir (\d+) evento do tipo "([^"]*)" no outbox para o cartão$`, e.deveExistirEventoNoOutbox)
	sc.Step(`^o payload do evento deve referenciar o cartão e o vencimento em (\d+) dias$`, e.assertOutboxPayloadReferencesCard)
}

func (e *cardE2ECtx) getInvoiceForDate(data string) error {
	if e.cardID == "" {
		return fmt.Errorf("cardID nao definido")
	}
	return e.makeRequest(http.MethodGet, "/api/v1/cards/"+e.cardID+"/invoices?for="+data, nil)
}

func (e *cardE2ECtx) getInvoiceWithoutDate() error {
	if e.cardID == "" {
		return fmt.Errorf("cardID nao definido")
	}
	return e.makeRequest(http.MethodGet, "/api/v1/cards/"+e.cardID+"/invoices", nil)
}

func (e *cardE2ECtx) assertClosingDate(expected string) error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}
	got, ok := e.lastBody["closing_date"].(string)
	if !ok {
		return fmt.Errorf("campo closing_date ausente ou nao e string")
	}
	if got != expected {
		return fmt.Errorf("closing_date esperado %q, recebido %q", expected, got)
	}
	return nil
}

func (e *cardE2ECtx) assertDueDate(expected string) error {
	if e.lastBody == nil {
		return fmt.Errorf("corpo JSON ausente")
	}
	got, ok := e.lastBody["due_date"].(string)
	if !ok {
		return fmt.Errorf("campo due_date ausente ou nao e string")
	}
	if got != expected {
		return fmt.Errorf("due_date esperado %q, recebido %q", expected, got)
	}
	return nil
}

func (e *cardE2ECtx) getInvoiceForNonExistentCard(data string) error {
	return e.makeRequest(http.MethodGet, "/api/v1/cards/"+uuid.NewString()+"/invoices?for="+data, nil)
}

func (e *cardE2ECtx) getInvoiceForRegisteredCard(data string) error {
	if e.cardID == "" {
		return fmt.Errorf("cardID nao definido")
	}
	return e.makeRequest(http.MethodGet, "/api/v1/cards/"+e.cardID+"/invoices?for="+data, nil)
}

func (e *cardE2ECtx) cardWithInvoiceDueInDays(days int) error {
	now := time.Now().UTC()
	dueDate := now.AddDate(0, 0, days)
	closingDay := dueDate.Day() - 5
	if closingDay < 1 {
		closingDay = 1
	}
	dueDay := dueDate.Day()

	if err := e.createCardViaHTTP(e.uniqueCardName("InvoiceDue"), closingDay, dueDay, 100000); err != nil {
		return err
	}

	e.expectedDueDate = dueDate
	e.expectedDaysUntil = days
	return nil
}

func (e *cardE2ECtx) workerDeAlertasEExecutado() error {
	return e.runInvoiceDueAlertsJob()
}

func (e *cardE2ECtx) deveExistirEventoNoOutbox(expected int, eventType string) error {
	if e.cardID == "" {
		return fmt.Errorf("cardID nao definido")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	err := e.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM mecontrola.outbox_events WHERE event_type = $1 AND aggregate_id = $2 AND aggregate_user_id = $3`,
		eventType,
		e.cardID,
		e.userID.String(),
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("consultar outbox_events: %w", err)
	}

	if count != expected {
		return fmt.Errorf("esperado %d evento(s) do tipo %q, encontrado %d", expected, eventType, count)
	}

	return nil
}

func (e *cardE2ECtx) assertOutboxPayloadReferencesCard(days int) error {
	if e.cardID == "" {
		return fmt.Errorf("cardID nao definido")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var rawPayload []byte
	err := e.db.QueryRowContext(
		ctx,
		`SELECT payload FROM mecontrola.outbox_events WHERE event_type = 'card.invoice_due.v1' AND aggregate_id = $1 AND aggregate_user_id = $2 LIMIT 1`,
		e.cardID,
		e.userID.String(),
	).Scan(&rawPayload)
	if err != nil {
		return fmt.Errorf("consultar payload do outbox: %w", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return fmt.Errorf("parsear payload do outbox: %w", err)
	}

	cardID, _ := payload["card_id"].(string)
	if cardID != e.cardID {
		return fmt.Errorf("payload card_id esperado %q, recebido %q", e.cardID, cardID)
	}

	userID, _ := payload["user_id"].(string)
	if userID != e.userID.String() {
		return fmt.Errorf("payload user_id esperado %q, recebido %q", e.userID.String(), userID)
	}

	if _, ok := payload["due_date"]; !ok {
		return fmt.Errorf("payload sem campo due_date")
	}

	daysUntil, ok := payload["days_until"].(float64)
	if !ok {
		return fmt.Errorf("payload sem campo days_until ou tipo invalido")
	}

	if int(daysUntil) != days {
		return fmt.Errorf("payload days_until esperado %d, recebido %d", days, int(daysUntil))
	}

	return nil
}
