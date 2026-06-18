//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
)

func registerJobSteps(sc *godog.ScenarioContext, e *billingE2ECtx) {
	sc.Step(`^que a assinatura está em PAST_DUE com grace expirado$`, e.givenSubscriptionPastDueExpiredGrace)
	sc.Step(`^que a assinatura está em PAST_DUE com grace vigente$`, e.givenSubscriptionPastDueActiveGrace)
	sc.Step(`^o job de expiração de grace é executado$`, e.whenGraceExpirationJobRuns)
	sc.Step(`^nenhum evento de expiração deve estar na outbox$`, e.thenNoExpirationEventInOutbox)
}

func (e *billingE2ECtx) givenSubscriptionPastDueExpiredGrace() error {
	if err := e.givenProductConfigured(); err != nil {
		return err
	}
	if err := e.whenBillingWebhookSent("order_approved"); err != nil {
		return err
	}
	if err := e.whenBillingWebhookSent("subscription_late"); err != nil {
		return err
	}
	_, _, _, subID, err := e.lookupSubscription()
	if err != nil {
		return err
	}
	e.capturedSubID = subID

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, execErr := e.db.ExecContext(ctx,
		`UPDATE billing_subscriptions SET grace_end = now() - interval '1 second' WHERE kiwify_order_id = $1`,
		e.orderID,
	)
	return execErr
}

func (e *billingE2ECtx) givenSubscriptionPastDueActiveGrace() error {
	if err := e.givenProductConfigured(); err != nil {
		return err
	}
	if err := e.whenBillingWebhookSent("order_approved"); err != nil {
		return err
	}
	if err := e.whenBillingWebhookSent("subscription_late"); err != nil {
		return err
	}
	_, _, _, subID, err := e.lookupSubscription()
	if err != nil {
		return err
	}
	e.capturedSubID = subID
	return nil
}

func (e *billingE2ECtx) whenGraceExpirationJobRuns() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return e.module.GraceExpirationJob.Run(ctx)
}

func (e *billingE2ECtx) thenNoExpirationEventInOutbox() error {
	count, err := e.countOutboxEvents(e.capturedSubID, "billing.subscription.expired_after_grace")
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("esperado 0 eventos de expiracao, encontrado %d", count)
	}
	return nil
}
