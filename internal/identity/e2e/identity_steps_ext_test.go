//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

func registerIdentityExtSteps(sc *godog.ScenarioContext, e *e2eCtx) {
	sc.Step(`^o evento de auth "([^"]*)" é projetado para o usuário com envelope fixo$`, e.projectAuthEventForUserWithFixedEnvelope)
	sc.Step(`^o mesmo envelope de auth é reprocessado para o usuário$`, e.replayLastAuthEnvelope)
	sc.Step(`^deve existir exatamente (\d+) auth_event do tipo "([^"]*)" para o usuário$`, e.assertExactAuthEventCountForUser)
	sc.Step(`^o evento de assinatura "([^"]*)" é projetado novamente para o usuário com status "([^"]*)"$`, e.projectSubscriptionEventAgain)
	sc.Step(`^deve existir exatamente (\d+) entitlement para o usuário$`, e.assertExactEntitlementCount)
	sc.Step(`^o principal é resolvido pelo canal "([^"]*)" e external_id "([^"]*)"$`, e.resolvePrincipalByChannelAndExternalID)
	sc.Step(`^o principal resolvido deve ter o UserID do usuário cadastrado$`, e.assertResolvedPrincipalMatchesSeededUser)
	sc.Step(`^a resolução de principal deve retornar erro$`, e.assertResolvePrincipalError)
	sc.Step(`^existe um auth_event antigo com occurred_at superior ao período de retenção$`, e.seedOldAuthEvent)
	sc.Step(`^o auth_event antigo não deve mais existir no banco$`, e.assertOldAuthEventDeleted)
	sc.Step(`^existe um auth_event recente no banco$`, e.seedRecentAuthEvent)
	sc.Step(`^o auth_event recente deve continuar existindo no banco$`, e.assertRecentAuthEventExists)
	sc.Step(`^o job de housekeeping de auth_events é executado$`, e.runAuthEventsHousekeepingJob)
}

func (e *e2eCtx) projectAuthEventForUserWithFixedEnvelope(eventType string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	kind := authKindFromEventType(eventType)
	rawPayload := buildAuthPayloadWithUser(e.identityLinkedUserID.String(), kind)
	fixedID := uuid.NewString()
	e.identityLastAuthEnvID = fixedID
	env := outbox.Envelope{
		ID:              fixedID,
		EventType:       eventType,
		AggregateUserID: e.identityLinkedUserID.String(),
		Payload:         rawPayload,
	}
	e.identityLastAuthEnv = env
	return e.identityModule.AuthEventsConsumer.Handle(ctx, identityTestEvent{
		eventType: eventType,
		payload:   env,
	})
}

func (e *e2eCtx) replayLastAuthEnvelope() error {
	if e.identityLastAuthEnvID == "" {
		return fmt.Errorf("no auth envelope stored for replay")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return e.identityModule.AuthEventsConsumer.Handle(ctx, identityTestEvent{
		eventType: e.identityLastAuthEnv.EventType,
		payload:   e.identityLastAuthEnv,
	})
}

func (e *e2eCtx) assertExactAuthEventCountForUser(expected int, kind string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var count int
	if err := e.mgr.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.auth_events WHERE user_id = $1 AND kind = $2`,
		e.identityLinkedUserID, kind,
	).Scan(&count); err != nil {
		return fmt.Errorf("query auth_events: %w", err)
	}
	if count != expected {
		return fmt.Errorf("expected exactly %d auth_event(s) of kind %q for user %s, found %d", expected, kind, e.identityLinkedUserID, count)
	}
	return nil
}

func (e *e2eCtx) projectSubscriptionEventAgain(eventType, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	periodEnd := time.Now().UTC().Add(30 * 24 * time.Hour)
	if err := e.seedBillingSubscription(ctx, e.identitySubID, e.identityLinkedUserID, status, periodEnd); err != nil {
		return fmt.Errorf("seed billing subscription (replay): %w", err)
	}
	subPayload, _ := json.Marshal(map[string]any{"subscription_id": e.identitySubID})
	env := outbox.Envelope{
		ID:        uuid.NewString(),
		EventType: eventType,
		Payload:   json.RawMessage(subPayload),
	}
	return e.identityModule.SubscriptionProjector.Handle(ctx, identityTestEvent{
		eventType: eventType,
		payload:   env,
	})
}

func (e *e2eCtx) assertExactEntitlementCount(expected int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var count int
	if err := e.mgr.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.identity_entitlements WHERE user_id = $1`,
		e.identityLinkedUserID,
	).Scan(&count); err != nil {
		return fmt.Errorf("query identity_entitlements: %w", err)
	}
	if count != expected {
		return fmt.Errorf("expected exactly %d entitlement(s) for user %s, found %d", expected, e.identityLinkedUserID, count)
	}
	return nil
}

func (e *e2eCtx) resolvePrincipalByChannelAndExternalID(channel, externalID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	principal, err := e.identityModule.ResolvePrincipalByIdentity.Execute(ctx, input.ResolvePrincipalByIdentity{
		Channel:    channel,
		ExternalID: externalID,
	})
	e.identityResolvePrincipalErr = err
	if err != nil {
		return nil
	}
	e.identityResolvedPrincipalUserID = principal.UserID
	return nil
}

func (e *e2eCtx) assertResolvedPrincipalMatchesSeededUser() error {
	if e.identityResolvePrincipalErr != nil {
		return fmt.Errorf("resolve principal returned unexpected error: %w", e.identityResolvePrincipalErr)
	}
	if e.identityResolvedPrincipalUserID != e.identityLinkedUserID {
		return fmt.Errorf("expected resolved user_id %s, got %s", e.identityLinkedUserID, e.identityResolvedPrincipalUserID)
	}
	return nil
}

func (e *e2eCtx) assertResolvePrincipalError() error {
	if e.identityResolvePrincipalErr == nil {
		return fmt.Errorf("expected resolve principal to return an error, got nil")
	}
	return nil
}

func (e *e2eCtx) seedOldAuthEvent() error {
	if e.identityLinkedUserID == uuid.Nil {
		return fmt.Errorf("no identity user seeded")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	oldOccurredAt := time.Now().UTC().Add(-100 * 24 * time.Hour)
	oldID := uuid.NewString()
	e.identityOldAuthEventID = oldID
	_, err := e.mgr.ExecContext(ctx, `
		INSERT INTO mecontrola.auth_events (id, user_id, kind, source, occurred_at, created_at)
		VALUES ($1::uuid, $2::uuid, 'principal_established', 'whatsapp', $3, $3)
	`, oldID, e.identityLinkedUserID, oldOccurredAt)
	if err != nil {
		return fmt.Errorf("seed old auth_event: %w", err)
	}
	return nil
}

func (e *e2eCtx) assertOldAuthEventDeleted() error {
	if e.identityOldAuthEventID == "" {
		return fmt.Errorf("no old auth_event ID stored")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var count int
	if err := e.mgr.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.auth_events WHERE id = $1`,
		e.identityOldAuthEventID,
	).Scan(&count); err != nil {
		return fmt.Errorf("query auth_events: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("expected old auth_event %s to be deleted, but it still exists", e.identityOldAuthEventID)
	}
	return nil
}

func (e *e2eCtx) seedRecentAuthEvent() error {
	if e.identityLinkedUserID == uuid.Nil {
		return fmt.Errorf("no identity user seeded")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	recentOccurredAt := time.Now().UTC().Add(-1 * time.Hour)
	recentID := uuid.NewString()
	e.identityRecentAuthEventID = recentID
	_, err := e.mgr.ExecContext(ctx, `
		INSERT INTO mecontrola.auth_events (id, user_id, kind, source, occurred_at, created_at)
		VALUES ($1::uuid, $2::uuid, 'principal_established', 'whatsapp', $3, $3)
	`, recentID, e.identityLinkedUserID, recentOccurredAt)
	if err != nil {
		return fmt.Errorf("seed recent auth_event: %w", err)
	}
	return nil
}

func (e *e2eCtx) assertRecentAuthEventExists() error {
	if e.identityRecentAuthEventID == "" {
		return fmt.Errorf("no recent auth_event ID stored")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var count int
	if err := e.mgr.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.auth_events WHERE id = $1`,
		e.identityRecentAuthEventID,
	).Scan(&count); err != nil {
		return fmt.Errorf("query auth_events: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("expected recent auth_event %s to still exist, found %d", e.identityRecentAuthEventID, count)
	}
	return nil
}

func (e *e2eCtx) runAuthEventsHousekeepingJob() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return e.identityModule.AuthEventsHousekeepingJob.Run(ctx)
}
