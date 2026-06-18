//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cucumber/godog"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type billingTestEvent struct {
	eventType string
	payload   any
}

func (e billingTestEvent) GetEventType() string { return e.eventType }
func (e billingTestEvent) GetPayload() any      { return e.payload }

func registerConsumerSteps(sc *godog.ScenarioContext, e *billingE2ECtx) {
	sc.Step(`^o handler do consumer "([^"]*)" recebe um envelope válido$`, e.whenConsumerHandlerReceivesValidEnvelope)
	sc.Step(`^o handler do consumer "([^"]*)" recebe um payload inválido$`, e.whenConsumerHandlerReceivesInvalidPayload)
	sc.Step(`^o handler retorna nil sem erro$`, e.thenConsumerHandlerReturnsNil)
}

func (e *billingE2ECtx) findHandler(eventType string) (events.Handler, error) {
	for _, reg := range e.module.EventHandlers {
		if reg.EventType == eventType {
			return reg.Handler, nil
		}
	}
	return nil, fmt.Errorf("handler para eventType %q nao encontrado", eventType)
}

func (e *billingE2ECtx) whenConsumerHandlerReceivesValidEnvelope(eventType string) error {
	handler, err := e.findHandler(eventType)
	if err != nil {
		return err
	}
	envelope := outbox.Envelope{
		ID:         "test-id",
		EventType:  eventType,
		OccurredAt: time.Now().UTC(),
		Payload:    json.RawMessage(`{"subscription_id":"test-sub-id"}`),
	}
	ev := billingTestEvent{eventType: eventType, payload: envelope}
	ctx := context.Background()
	e.lastConsumerErr = handler.Handle(ctx, ev)
	return nil
}

func (e *billingE2ECtx) whenConsumerHandlerReceivesInvalidPayload(eventType string) error {
	handler, err := e.findHandler(eventType)
	if err != nil {
		return err
	}
	ev := billingTestEvent{eventType: eventType, payload: "not-an-envelope-string"}
	ctx := context.Background()
	e.lastConsumerErr = handler.Handle(ctx, ev)
	return nil
}

func (e *billingE2ECtx) thenConsumerHandlerReturnsNil() error {
	if e.lastConsumerErr != nil {
		return fmt.Errorf("handler retornou erro: %w", e.lastConsumerErr)
	}
	return nil
}
