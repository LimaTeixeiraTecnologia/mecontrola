//go:build e2e

package e2e_test

import (
	"fmt"
	"time"

	"github.com/cucumber/godog"
)

func registerOutboxSteps(sc *godog.ScenarioContext, e *billingE2ECtx) {
	sc.Step(`^o evento "([^"]*)" deve estar na outbox$`, e.thenOutboxEventExists)
	sc.Step(`^deve existir exatamente (\d+) evento "([^"]*)" na outbox$`, e.thenOutboxEventCountExact)
	sc.Step(`^o envelope do evento "([^"]*)" deve ter aggregate_type "([^"]*)"$`, e.thenEnvelopeHasAggregateType)
	sc.Step(`^o payload do evento "([^"]*)" deve conter o campo "([^"]*)"$`, e.thenPayloadContainsField)
	sc.Step(`^o payload do evento "([^"]*)" deve ter previous_period_end anterior ao period_end$`, e.thenRenewedPayloadHasCorrectDates)
	sc.Step(`^o payload do evento "([^"]*)" deve ter grace_end aproximadamente 3 dias após occurred_at$`, e.thenPastDuePayloadHasGraceEnd)
	sc.Step(`^a outbox não deve ter duplicata para o evento "([^"]*)"$`, e.thenOutboxNoDuplicate)
}

func (e *billingE2ECtx) resolveSubID() (string, error) {
	if e.capturedSubID != "" {
		return e.capturedSubID, nil
	}
	_, _, _, subID, err := e.lookupSubscription()
	if err != nil {
		return "", err
	}
	e.capturedSubID = subID
	return subID, nil
}

func (e *billingE2ECtx) thenOutboxEventExists(eventType string) error {
	subID, err := e.resolveSubID()
	if err != nil {
		return err
	}
	count, err := e.countOutboxEvents(subID, eventType)
	if err != nil {
		return err
	}
	if count < 1 {
		return fmt.Errorf("evento %q nao encontrado na outbox para subID %q", eventType, subID)
	}
	return nil
}

func (e *billingE2ECtx) thenOutboxEventCountExact(expected int, eventType string) error {
	subID, err := e.resolveSubID()
	if err != nil {
		return err
	}
	count, err := e.countOutboxEvents(subID, eventType)
	if err != nil {
		return err
	}
	if count != expected {
		return fmt.Errorf("esperado %d evento(s) %q, encontrado %d", expected, eventType, count)
	}
	return nil
}

func (e *billingE2ECtx) thenEnvelopeHasAggregateType(eventType, expected string) error {
	subID, err := e.resolveSubID()
	if err != nil {
		return err
	}
	got, err := e.loadOutboxAggregateType(subID, eventType)
	if err != nil {
		return err
	}
	if got != expected {
		return fmt.Errorf("aggregate_type esperado %q, recebido %q", expected, got)
	}
	return nil
}

func (e *billingE2ECtx) thenPayloadContainsField(eventType, field string) error {
	subID, err := e.resolveSubID()
	if err != nil {
		return err
	}
	payload, err := e.loadOutboxPayload(subID, eventType)
	if err != nil {
		return err
	}
	val, ok := payload[field]
	if !ok || val == nil || val == "" {
		return fmt.Errorf("campo %q ausente ou vazio no payload do evento %q", field, eventType)
	}
	return nil
}

func (e *billingE2ECtx) thenRenewedPayloadHasCorrectDates(eventType string) error {
	subID, err := e.resolveSubID()
	if err != nil {
		return err
	}
	payload, err := e.loadOutboxPayload(subID, eventType)
	if err != nil {
		return err
	}
	prevStr, ok := payload["previous_period_end"].(string)
	if !ok || prevStr == "" {
		return fmt.Errorf("campo previous_period_end ausente ou invalido no payload")
	}
	currStr, ok := payload["period_end"].(string)
	if !ok || currStr == "" {
		return fmt.Errorf("campo period_end ausente ou invalido no payload")
	}
	prev, err := time.Parse(time.RFC3339, prevStr)
	if err != nil {
		return fmt.Errorf("parse previous_period_end %q: %w", prevStr, err)
	}
	curr, err := time.Parse(time.RFC3339, currStr)
	if err != nil {
		return fmt.Errorf("parse period_end %q: %w", currStr, err)
	}
	if !prev.Before(curr) {
		return fmt.Errorf("previous_period_end %v deve ser anterior a period_end %v", prev, curr)
	}
	return nil
}

func (e *billingE2ECtx) thenPastDuePayloadHasGraceEnd(eventType string) error {
	subID, err := e.resolveSubID()
	if err != nil {
		return err
	}
	payload, err := e.loadOutboxPayload(subID, eventType)
	if err != nil {
		return err
	}
	graceStr, ok := payload["grace_end"].(string)
	if !ok || graceStr == "" {
		return fmt.Errorf("campo grace_end ausente ou vazio no payload do evento %q", eventType)
	}
	occurredStr, ok := payload["occurred_at"].(string)
	if !ok || occurredStr == "" {
		return fmt.Errorf("campo occurred_at ausente ou vazio no payload do evento %q", eventType)
	}
	graceEnd, err := time.Parse(time.RFC3339, graceStr)
	if err != nil {
		return fmt.Errorf("parse grace_end %q: %w", graceStr, err)
	}
	occurredAt, err := time.Parse(time.RFC3339, occurredStr)
	if err != nil {
		return fmt.Errorf("parse occurred_at %q: %w", occurredStr, err)
	}
	diff := graceEnd.Sub(occurredAt)
	minDiff := 2 * 24 * time.Hour
	maxDiff := 4 * 24 * time.Hour
	if diff < minDiff || diff > maxDiff {
		return fmt.Errorf("grace_end deveria ser ~3 dias apos occurred_at, diferenca foi %v", diff)
	}
	return nil
}

func (e *billingE2ECtx) thenOutboxNoDuplicate(eventType string) error {
	subID, err := e.resolveSubID()
	if err != nil {
		return err
	}
	count, err := e.countOutboxEvents(subID, eventType)
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("esperado exatamente 1 evento %q, encontrado %d (duplicata detectada)", eventType, count)
	}
	return nil
}
