//go:build e2e

package e2e_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cucumber/godog"
)

func registerWebhookSteps(sc *godog.ScenarioContext, e *billingE2ECtx) {
	sc.Step(`^que o produto billing está configurado$`, e.givenProductConfigured)
	sc.Step(`^que existe uma assinatura billing ativa$`, e.givenActiveBillingSubscription)
	sc.Step(`^o webhook billing "([^"]*)" é enviado$`, e.whenBillingWebhookSent)
	sc.Step(`^o webhook "([^"]*)" é enviado sem parâmetros de rastreamento$`, e.whenBillingWebhookSentWithoutTracking)
	sc.Step(`^o webhook billing "([^"]*)" é reenviado com o mesmo order_id$`, e.whenBillingWebhookReplayed)
	sc.Step(`^o webhook billing "([^"]*)" é reenviado com o mesmo timestamp$`, e.whenBillingWebhookReplayedExact)
	sc.Step(`^o webhook "([^"]*)" é reenviado com data anterior ao primeiro evento$`, e.whenBillingWebhookSentWithPastDate)
	sc.Step(`^o webhook é enviado sem assinatura HMAC$`, e.whenWebhookSentWithoutHMAC)
	sc.Step(`^o webhook é enviado com Content-Type incorreto$`, e.whenWebhookSentWithWrongContentType)
	sc.Step(`^o webhook é enviado com corpo JSON inválido$`, e.whenWebhookSentWithInvalidJSON)
	sc.Step(`^o webhook é enviado com trigger desconhecido$`, e.whenWebhookSentWithUnknownTrigger)
	sc.Step(`^o webhook é enviado com subscription_id inválido$`, e.whenWebhookSentWithInvalidSubID)
	sc.Step(`^o webhook é enviado com sck contendo apenas espaços$`, e.whenWebhookSentWithWhitespaceOnlySCK)
	sc.Step(`^a assinatura billing deve estar salva como "([^"]*)"$`, e.thenSubscriptionStatusShouldBe)
	sc.Step(`^o periodo period_end deve ser estendido$`, e.thenPeriodEndExtended)
	sc.Step(`^o period_end da assinatura deve ser preservado$`, e.thenPeriodEndPreserved)
	sc.Step(`^o evento processado "([^"]*)" deve ter sido registrado$`, e.thenProcessedEventRegistered)
}

func (e *billingE2ECtx) givenProductConfigured() error {
	if e.productMonthly == "" {
		return fmt.Errorf("productMonthly nao configurado")
	}
	e.freshOrderIDs()
	return nil
}

func (e *billingE2ECtx) givenActiveBillingSubscription() error {
	if err := e.givenProductConfigured(); err != nil {
		return err
	}
	if err := e.whenBillingWebhookSent("order_approved"); err != nil {
		return err
	}
	if err := e.thenHTTPStatusShouldBe(202); err != nil {
		return err
	}
	_, periodEnd, _, _, err := e.lookupSubscription()
	if err != nil {
		return err
	}
	e.billingPeriodEnd = periodEnd
	return nil
}

func (e *billingE2ECtx) whenBillingWebhookSent(eventType string) error {
	now := e.nextEventTime()
	if eventType != "order_approved" {
		_, prevPeriodEnd, _, _, err := e.lookupSubscription()
		if err != nil {
			return fmt.Errorf("lookup subscription before %q: %w", eventType, err)
		}
		e.billingPrevPeriodEnd = prevPeriodEnd
	}
	payload := e.buildWebhookPayload(eventType, now)
	return e.makeWebhookRequest(payload)
}

func (e *billingE2ECtx) whenBillingWebhookSentWithoutTracking(eventType string) error {
	now := e.nextEventTime()
	payload := e.buildWebhookPayloadNoTracking(eventType, now)
	return e.makeWebhookRequest(payload)
}

func (e *billingE2ECtx) whenBillingWebhookReplayed(eventType string) error {
	_, _, _, subID, err := e.lookupSubscription()
	if err != nil {
		return fmt.Errorf("lookup subscription before replay %q: %w", eventType, err)
	}
	e.capturedSubID = subID
	count, countErr := e.countOutboxEvents(subID, eventType)
	if countErr == nil {
		e.outboxCountBefore = count
	}
	now := e.nextEventTime()
	payload := e.buildWebhookPayload(eventType, now)
	return e.makeWebhookRequest(payload)
}

func (e *billingE2ECtx) whenBillingWebhookReplayedExact(eventType string) error {
	_, _, _, subID, err := e.lookupSubscription()
	if err != nil {
		return fmt.Errorf("lookup subscription before exact replay %q: %w", eventType, err)
	}
	e.capturedSubID = subID
	payload := e.buildWebhookPayload(eventType, e.billingEventAt)
	return e.makeWebhookRequest(payload)
}

func (e *billingE2ECtx) whenBillingWebhookSentWithPastDate(eventType string) error {
	_, prevPeriodEnd, _, _, err := e.lookupSubscription()
	if err != nil {
		return fmt.Errorf("lookup subscription before regression replay: %w", err)
	}
	e.billingPrevPeriodEnd = prevPeriodEnd
	pastTime := e.billingEventAt.Add(-2 * time.Hour)
	payload := e.buildWebhookPayload(eventType, e.nextEventTime())
	payload["updated_at"] = pastTime.UTC().Format(time.RFC3339)
	return e.makeWebhookRequest(payload)
}

func (e *billingE2ECtx) whenWebhookSentWithoutHMAC() error {
	now := e.nextEventTime()
	payload := e.buildWebhookPayload("order_approved", now)
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return e.makeWebhookRequestRaw(data, "application/json", "")
}

func (e *billingE2ECtx) whenWebhookSentWithWrongContentType() error {
	now := e.nextEventTime()
	payload := e.buildWebhookPayload("order_approved", now)
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	mac := hmac.New(sha1.New, []byte(e.webhookSecret))
	mac.Write(data)
	signature := hex.EncodeToString(mac.Sum(nil))
	return e.makeWebhookRequestRaw(data, "text/plain", signature)
}

func (e *billingE2ECtx) whenWebhookSentWithInvalidJSON() error {
	body := []byte("not-json")
	mac := hmac.New(sha1.New, []byte(e.webhookSecret))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))
	return e.makeWebhookRequestRaw(body, "application/json", signature)
}

func (e *billingE2ECtx) whenWebhookSentWithUnknownTrigger() error {
	now := e.nextEventTime()
	payload := e.buildWebhookPayload("trigger_desconhecido", now)
	payload["webhook_event_type"] = "trigger_desconhecido"
	return e.makeWebhookRequest(payload)
}

func (e *billingE2ECtx) whenWebhookSentWithInvalidSubID() error {
	now := e.nextEventTime()
	payload := e.buildWebhookPayload("order_approved", now)
	payload["subscription_id"] = "   "
	return e.makeWebhookRequest(payload)
}

func (e *billingE2ECtx) whenWebhookSentWithWhitespaceOnlySCK() error {
	now := e.nextEventTime()
	payload := e.buildWebhookPayload("order_approved", now)
	payload["TrackingParameters"] = map[string]any{
		"sck": "   ",
		"s1":  "",
		"src": "",
	}
	return e.makeWebhookRequest(payload)
}

func (e *billingE2ECtx) thenSubscriptionStatusShouldBe(expected string) error {
	status, _, _, _, err := e.lookupSubscription()
	if err != nil {
		return err
	}
	if status != expected {
		return fmt.Errorf("status esperado %q, recebido %q", expected, status)
	}
	return nil
}

func (e *billingE2ECtx) thenPeriodEndExtended() error {
	_, periodEnd, _, _, err := e.lookupSubscription()
	if err != nil {
		return err
	}
	if !periodEnd.After(e.billingPrevPeriodEnd) {
		return fmt.Errorf("period_end %v nao e posterior ao anterior %v", periodEnd, e.billingPrevPeriodEnd)
	}
	return nil
}

func (e *billingE2ECtx) thenPeriodEndPreserved() error {
	_, periodEnd, _, _, err := e.lookupSubscription()
	if err != nil {
		return err
	}
	if !periodEnd.Equal(e.billingPrevPeriodEnd) {
		return fmt.Errorf("period_end %v deveria ser igual ao anterior %v", periodEnd, e.billingPrevPeriodEnd)
	}
	return nil
}

func (e *billingE2ECtx) thenProcessedEventRegistered(trigger string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	row := e.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM billing_processed_events WHERE trigger=$1 AND recurso_id=$2`,
		trigger, e.orderID,
	)
	if err := row.Scan(&count); err != nil {
		return fmt.Errorf("query processed events: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("evento processado com trigger %q e recurso_id %q nao encontrado", trigger, e.orderID)
	}
	return nil
}
