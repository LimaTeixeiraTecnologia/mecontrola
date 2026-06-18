//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"time"
)

func (e *e2eCtx) existeUmProdutoBillingConfigurado() error {
	if e.billingProductID == "" {
		return fmt.Errorf("produto billing nao configurado")
	}
	return nil
}

func (e *e2eCtx) queExisteUmaAssinaturaBillingAtiva() error {
	e.billingOrderID = fmt.Sprintf("order-e2e-%d", time.Now().UnixNano())
	e.billingSubscriptionID = fmt.Sprintf("sub-e2e-%d", time.Now().UnixNano())

	if err := e.oWebhookBillingEEnviado("order_approved"); err != nil {
		return err
	}
	if err := e.aRespostaHTTPDeveTerStatus(202); err != nil {
		return err
	}

	return e.captureBillingPeriodEnd()
}

func (e *e2eCtx) oWebhookBillingEEnviado(eventType string) error {
	if e.billingOrderID == "" {
		e.billingOrderID = fmt.Sprintf("order-e2e-%d", time.Now().UnixNano())
	}
	if e.billingSubscriptionID == "" {
		e.billingSubscriptionID = fmt.Sprintf("sub-e2e-%d", time.Now().UnixNano())
	}
	if eventType != "order_approved" {
		periodEnd, err := e.lookupBillingPeriodEnd()
		if err != nil {
			return err
		}
		e.billingPreviousPeriodEnd = periodEnd
	}

	now := time.Now().UTC()
	if !e.billingEventAt.IsZero() {
		now = e.billingEventAt.Add(time.Minute)
	}
	e.billingEventAt = now
	payload := map[string]any{
		"order_id":           e.billingOrderID,
		"order_ref":          "e2e-ref",
		"order_status":       "paid",
		"webhook_event_type": eventType,
		"subscription_id":    e.billingSubscriptionID,
		"Product": map[string]any{
			"product_id":   e.billingProductID,
			"product_name": "E2E Billing",
		},
		"Customer": map[string]any{
			"email":  "e2e-billing@example.com",
			"mobile": "+5511900000000",
			"CPF":    "00000000000",
		},
		"Subscription": map[string]any{
			"status":       "active",
			"start_date":   now.Add(-24 * time.Hour).Format(time.RFC3339),
			"next_payment": now.Add(30 * 24 * time.Hour).Format(time.RFC3339),
		},
		"TrackingParameters": map[string]any{
			"sck": "billing-funnel-token-e2e",
			"s1":  nil,
			"src": nil,
		},
		"approved_date": now.Format("2006-01-02 15:04:05"),
		"updated_at":    now.Format("2006-01-02 15:04:05"),
		"created_at":    now.Format("2006-01-02 15:04:05"),
	}

	return e.makeSignedBillingWebhookRequest(payload)
}

func (e *e2eCtx) aAssinaturaBillingDeveEstarSalvaComo(status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := e.mgr.QueryRowContext(ctx, `
		SELECT status, period_end
		  FROM billing_subscriptions
		 WHERE kiwify_order_id = $1
	`, e.billingOrderID)

	var persistedStatus string
	var periodEnd time.Time
	if err := row.Scan(&persistedStatus, &periodEnd); err != nil {
		return fmt.Errorf("buscar billing subscription: %w", err)
	}
	if persistedStatus != status {
		return fmt.Errorf("status esperado %s, recebido %s", status, persistedStatus)
	}

	e.billingPeriodEnd = periodEnd
	return nil
}

func (e *e2eCtx) oEventoDeDominioDeveEstarNaOutbox(eventType string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	row := e.mgr.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM outbox_events
		 WHERE aggregate_id = (
			SELECT id::text FROM billing_subscriptions WHERE kiwify_order_id = $1
		 )
		   AND event_type = $2
	`, e.billingOrderID, eventType)
	if err := row.Scan(&count); err != nil {
		return fmt.Errorf("consultar outbox: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("nenhum evento %s encontrado na outbox", eventType)
	}
	return nil
}

func (e *e2eCtx) oEventoProcessadoDeveTerSidoRegistrado(eventType string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	row := e.mgr.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM billing_processed_events
		 WHERE trigger = $1
		   AND recurso_id = $2
	`, eventType, e.billingOrderID)
	if err := row.Scan(&count); err != nil {
		return fmt.Errorf("consultar processed events: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("evento processado %s nao encontrado para %s", eventType, e.billingOrderID)
	}
	return nil
}

func (e *e2eCtx) oPeriodEndDaAssinaturaBillingDeveSerPreservado() error {
	current, err := e.lookupBillingPeriodEnd()
	if err != nil {
		return err
	}
	if !current.Equal(e.billingPreviousPeriodEnd) {
		return fmt.Errorf("period_end deveria ser preservado; esperado %s, recebido %s", e.billingPreviousPeriodEnd, current)
	}
	return nil
}

func (e *e2eCtx) oPeriodEndDaAssinaturaBillingDeveSerEstendido() error {
	current, err := e.lookupBillingPeriodEnd()
	if err != nil {
		return err
	}
	if !current.After(e.billingPreviousPeriodEnd) {
		return fmt.Errorf("period_end deveria ser estendido; anterior %s, atual %s", e.billingPreviousPeriodEnd, current)
	}
	e.billingPeriodEnd = current
	return nil
}

func (e *e2eCtx) captureBillingPeriodEnd() error {
	periodEnd, err := e.lookupBillingPeriodEnd()
	if err != nil {
		return err
	}
	e.billingPeriodEnd = periodEnd
	return nil
}

func (e *e2eCtx) lookupBillingPeriodEnd() (time.Time, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := e.mgr.QueryRowContext(ctx, `
		SELECT period_end
		  FROM billing_subscriptions
		 WHERE kiwify_order_id = $1
	`, e.billingOrderID)

	var periodEnd time.Time
	if err := row.Scan(&periodEnd); err != nil {
		return time.Time{}, fmt.Errorf("consultar period_end: %w", err)
	}
	return periodEnd, nil
}
