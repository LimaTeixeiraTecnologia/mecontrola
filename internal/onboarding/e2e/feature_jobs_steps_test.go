//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
)

func registerJobSteps(sc *godog.ScenarioContext, w *onboardingWorld) {
	sc.Step(`^existe um token pago elegível para outreach via WhatsApp$`, w.givenPaidTokenEligibleForWhatsAppOutreachExists)
	sc.Step(`^existe um token pago expirado$`, w.givenExpiredPaidTokenExists)
	sc.Step(`^existem registros antigos de deduplicação e lookup$`, w.givenOldDedupAndLookupRecordsExist)
	sc.Step(`^o job de outreach é executado$`, w.whenOutreachJobRuns)
	sc.Step(`^o job de expiração de tokens é executado$`, w.whenTokenExpirationJobRuns)
	sc.Step(`^o job de limpeza do onboarding é executado$`, w.whenCleanupJobRuns)
	sc.Step(`^deve ter sido enviado (\d+) template\(s\) de outreach$`, w.thenOutreachTemplateCountShouldBe)
	sc.Step(`^os registros antigos devem ter sido removidos$`, w.thenOldCleanupRecordsShouldBeRemoved)
}

func (w *onboardingWorld) givenPaidTokenEligibleForWhatsAppOutreachExists() error {
	subID, err := w.seedBillingSubscription("+5511999995555", "outreach@example.com")
	if err != nil {
		return err
	}
	paidAt := time.Now().UTC().Add(-48 * time.Hour)
	return w.seedMagicToken(newDeterministicToken('o'), "PAID", tokenSeedOptions{
		subscriptionID: subID,
		customerMobile: "+5511999995555",
		customerEmail:  "outreach@example.com",
		externalSaleID: "sale-outreach",
		paidAt:         paidAt,
	})
}

func (w *onboardingWorld) givenExpiredPaidTokenExists() error {
	subID, err := w.seedBillingSubscription("+5511999996666", "expired@example.com")
	if err != nil {
		return err
	}
	paidAt := time.Now().UTC().Add(-72 * time.Hour)
	return w.seedMagicToken(newDeterministicToken('z'), "PAID", tokenSeedOptions{
		subscriptionID: subID,
		customerMobile: "+5511999996666",
		customerEmail:  "expired@example.com",
		externalSaleID: "sale-expired",
		paidAt:         paidAt,
		expired:        true,
	})
}

func (w *onboardingWorld) givenOldDedupAndLookupRecordsExist() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := w.runtime.deps.db.ExecContext(ctx, `
		INSERT INTO mecontrola.channel_processed_messages (channel, message_id, processed_at)
		VALUES ('whatsapp', 'msg-old', now() - interval '40 day');
		INSERT INTO mecontrola.consumer_lookup_attempts (event_id, attempts, last_attempt_at)
		VALUES ('evt-old', 2, now() - interval '40 day');
	`)
	return err
}

func (w *onboardingWorld) whenOutreachJobRuns() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return w.runtime.deps.outreachJob.Run(ctx)
}

func (w *onboardingWorld) whenTokenExpirationJobRuns() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return w.runtime.deps.expirationJob.Run(ctx)
}

func (w *onboardingWorld) whenCleanupJobRuns() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return w.runtime.deps.cleanupJob.Run(ctx)
}

func (w *onboardingWorld) thenOutreachTemplateCountShouldBe(expected int) error {
	got := len(w.runtime.outreachGateway.templatesSent)
	if got != expected {
		return fmt.Errorf("esperados %d templates de outreach, recebidos %d", expected, got)
	}
	return nil
}

func (w *onboardingWorld) thenOldCleanupRecordsShouldBeRemoved() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var dedupCount int
	if err := w.runtime.deps.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mecontrola.channel_processed_messages WHERE message_id = 'msg-old'`).Scan(&dedupCount); err != nil {
		return err
	}
	var lookupCount int
	if err := w.runtime.deps.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mecontrola.consumer_lookup_attempts WHERE event_id = 'evt-old'`).Scan(&lookupCount); err != nil {
		return err
	}
	if dedupCount != 0 || lookupCount != 0 {
		return fmt.Errorf("registros antigos nao foram removidos")
	}
	return nil
}
