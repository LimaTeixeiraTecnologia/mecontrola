//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type identityTestEvent struct {
	eventType string
	payload   any
}

func (e identityTestEvent) GetEventType() string { return e.eventType }
func (e identityTestEvent) GetPayload() any      { return e.payload }

func registerIdentitySteps(sc *godog.ScenarioContext, e *e2eCtx) {
	sc.Step(`^o sistema recebe um cadastro com whatsapp "([^"]*)" e email "([^"]*)"$`, e.userRegistrationWithWhatsAppAndEmail)
	sc.Step(`^o sistema recebe um cadastro com whatsapp "([^"]*)" sem email$`, e.userRegistrationWithWhatsAppOnly)
	sc.Step(`^o sistema recebe um cadastro com whatsapp "([^"]*)" e display_name "([^"]*)"$`, e.userRegistrationWithDisplayName)
	sc.Step(`^o sistema recebe um cadastro com whatsapp "([^"]*)" e email inválido "([^"]*)"$`, e.userRegistrationWithInvalidEmail)
	sc.Step(`^o usuário deve estar salvo no banco com whatsapp "([^"]*)" e status "([^"]*)"$`, e.assertUserExistsWithStatus)
	sc.Step(`^o usuário deve estar salvo no banco com display_name "([^"]*)"$`, e.assertUserHasDisplayName)
	sc.Step(`^um novo registro de usuário deve existir no banco com whatsapp "([^"]*)"$`, e.assertNewUserCreatedWithWhatsApp)
	sc.Step(`^existe um usuário com whatsapp "([^"]*)" cadastrado no sistema$`, e.seedUserWithWhatsApp)
	sc.Step(`^existe um usuário com whatsapp "([^"]*)" cadastrado com email "([^"]*)"$`, e.seedUserWithWhatsAppAndEmail)
	sc.Step(`^o usuário com whatsapp "([^"]*)" foi deletado há (\d+) horas$`, e.seedDeletedUserHoursAgo)
	sc.Step(`^o canal "([^"]*)" com external_id "([^"]*)" é vinculado ao usuário$`, e.linkChannelToUser)
	sc.Step(`^o canal "([^"]*)" com external_id "([^"]*)" foi vinculado ao usuário$`, e.linkChannelToUser)
	sc.Step(`^o canal "([^"]*)" com external_id "([^"]*)" é vinculado novamente ao mesmo usuário$`, e.linkChannelToUser)
	sc.Step(`^o canal "([^"]*)" com external_id "([^"]*)" é vinculado ao segundo usuário$`, e.linkChannelToSecondUser)
	sc.Step(`^que existe um segundo usuário com whatsapp "([^"]*)" cadastrado no sistema$`, e.seedSecondUserWithWhatsApp)
	sc.Step(`^o canal preferido do usuário é consultado$`, e.resolvePreferredChannel)
	sc.Step(`^o canal preferido resolvido deve ser "([^"]*)"$`, e.assertResolvedChannel)
	sc.Step(`^a vinculação deve estar salva no banco com canal "([^"]*)" e external_id "([^"]*)"$`, e.assertChannelLinkExists)
	sc.Step(`^a operação de vinculação deve retornar erro de canal já vinculado$`, e.assertChannelAlreadyLinkedError)
	sc.Step(`^o evento de assinatura "([^"]*)" é projetado para o usuário com status "([^"]*)"$`, e.projectSubscriptionEvent)
	sc.Step(`^o evento de assinatura "([^"]*)" foi projetado para o usuário com status "([^"]*)"$`, e.projectSubscriptionEvent)
	sc.Step(`^o entitlement do usuário deve estar salvo no banco com status "([^"]*)"$`, e.assertEntitlementStatus)
	sc.Step(`^o usuário é deletado via use case$`, e.deleteUserViaUseCase)
	sc.Step(`^o principal é estabelecido para o whatsapp "([^"]*)"$`, e.establishPrincipalForWhatsApp)
	sc.Step(`^o principal é estabelecido para o whatsapp "([^"]*)" desconhecido$`, e.establishPrincipalForUnknownWhatsApp)
	sc.Step(`^uma falha de autenticação de gateway é registrada para o usuário$`, e.recordGatewayAuthFailure)
	sc.Step(`^o evento "([^"]*)" deve estar registrado na outbox para o usuário$`, e.assertOutboxEventForUser)
	sc.Step(`^o evento "([^"]*)" deve estar na outbox sem usuário associado$`, e.assertOutboxEventWithoutUser)
	sc.Step(`^o evento de auth "([^"]*)" é projetado para o usuário$`, e.projectAuthEventForUser)
	sc.Step(`^o evento de auth "([^"]*)" foi projetado para o usuário$`, e.projectAuthEventForUser)
	sc.Step(`^o evento de auth "([^"]*)" é projetado sem usuário associado$`, e.projectAuthEventWithoutUser)
	sc.Step(`^um auth_event do tipo "([^"]*)" deve existir no banco para o usuário$`, e.assertAuthEventKindForUser)
	sc.Step(`^um auth_event do tipo "([^"]*)" deve existir no banco sem user_id$`, e.assertAuthEventKindWithoutUser)
	sc.Step(`^os auth_events do usuário devem estar com user_id nulo$`, e.assertAuthEventsAnonymized)
}

func (e *e2eCtx) userRegistrationWithWhatsAppAndEmail(whatsapp, email string) error {
	if err := e.makeRequest(http.MethodPost, "/api/v1/identity/users", map[string]any{
		"whatsapp": whatsapp,
		"email":    email,
	}); err != nil {
		return err
	}
	e.captureIdentityUserIDFromResponse()
	return nil
}

func (e *e2eCtx) userRegistrationWithWhatsAppOnly(whatsapp string) error {
	if err := e.makeRequest(http.MethodPost, "/api/v1/identity/users", map[string]any{
		"whatsapp": whatsapp,
	}); err != nil {
		return err
	}
	e.captureIdentityUserIDFromResponse()
	return nil
}

func (e *e2eCtx) userRegistrationWithDisplayName(whatsapp, displayName string) error {
	if err := e.makeRequest(http.MethodPost, "/api/v1/identity/users", map[string]any{
		"whatsapp":     whatsapp,
		"display_name": displayName,
	}); err != nil {
		return err
	}
	e.captureIdentityUserIDFromResponse()
	return nil
}

func (e *e2eCtx) userRegistrationWithInvalidEmail(whatsapp, email string) error {
	return e.makeRequest(http.MethodPost, "/api/v1/identity/users", map[string]any{
		"whatsapp": whatsapp,
		"email":    email,
	})
}

func (e *e2eCtx) captureIdentityUserIDFromResponse() {
	if id, ok := e.lastBody["id"].(string); ok {
		e.identityUserID = id
		if parsed, err := uuid.Parse(id); err == nil {
			e.identityLinkedUserID = parsed
		}
	}
}

func (e *e2eCtx) assertUserExistsWithStatus(whatsapp, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var got string
	if err := e.mgr.QueryRowContext(ctx,
		`SELECT status FROM mecontrola.users WHERE whatsapp_number = $1 AND deleted_at IS NULL`,
		whatsapp,
	).Scan(&got); err != nil {
		return fmt.Errorf("user %q not found: %w", whatsapp, err)
	}
	if got != status {
		return fmt.Errorf("expected status %q, got %q", status, got)
	}
	return nil
}

func (e *e2eCtx) assertUserHasDisplayName(displayName string) error {
	if e.identityUserID == "" {
		return fmt.Errorf("no identity user ID captured from response")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var got string
	if err := e.mgr.QueryRowContext(ctx,
		`SELECT COALESCE(display_name, '') FROM mecontrola.users WHERE id = $1`,
		e.identityUserID,
	).Scan(&got); err != nil {
		return fmt.Errorf("user %q not found: %w", e.identityUserID, err)
	}
	if got != displayName {
		return fmt.Errorf("expected display_name %q, got %q", displayName, got)
	}
	return nil
}

func (e *e2eCtx) assertNewUserCreatedWithWhatsApp(whatsapp string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var total, active int
	if err := e.mgr.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.users WHERE whatsapp_number = $1`,
		whatsapp,
	).Scan(&total); err != nil {
		return fmt.Errorf("count total: %w", err)
	}
	if total < 2 {
		return fmt.Errorf("expected at least 2 records for whatsapp %q, found %d", whatsapp, total)
	}
	if err := e.mgr.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.users WHERE whatsapp_number = $1 AND deleted_at IS NULL AND status = 'ACTIVE'`,
		whatsapp,
	).Scan(&active); err != nil {
		return fmt.Errorf("count active: %w", err)
	}
	if active != 1 {
		return fmt.Errorf("expected exactly 1 active record for whatsapp %q, found %d", whatsapp, active)
	}
	return nil
}

func (e *e2eCtx) seedUserWithWhatsApp(whatsapp string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := e.identityModule.UpsertUserUseCase.Execute(
		ctx,
		input.UpsertUserByWhatsApp{WhatsAppNumber: whatsapp},
	)
	if err != nil {
		return fmt.Errorf("seed user %q: %w", whatsapp, err)
	}
	e.identityUserID = out.ID
	parsed, err := uuid.Parse(out.ID)
	if err != nil {
		return fmt.Errorf("parse seeded user id: %w", err)
	}
	e.identityLinkedUserID = parsed
	e.identitySubID = uuid.NewString()
	return nil
}

func (e *e2eCtx) seedUserWithWhatsAppAndEmail(whatsapp, email string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := e.identityModule.UpsertUserUseCase.Execute(
		ctx,
		input.UpsertUserByWhatsApp{WhatsAppNumber: whatsapp, Email: email},
	)
	return err
}

func (e *e2eCtx) seedDeletedUserHoursAgo(whatsapp string, hours int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	deletedAt := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	result, err := e.mgr.ExecContext(ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at, deleted_at)
		VALUES (gen_random_uuid(), $1, 'DELETED', $2, $2, $3)
	`, whatsapp, deletedAt.Add(-time.Hour), deletedAt)
	if err != nil {
		return fmt.Errorf("seed deleted user %q: %w", whatsapp, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected check: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("seed deleted user %q: no row inserted", whatsapp)
	}
	return nil
}

func (e *e2eCtx) linkChannelToUser(channel, externalID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := e.identityModule.LinkChannelToUser.Execute(ctx, input.LinkChannelToUser{
		UserID:     e.identityLinkedUserID,
		Channel:    channel,
		ExternalID: externalID,
	})
	e.identityLinkErr = err
	return nil
}

func (e *e2eCtx) seedSecondUserWithWhatsApp(whatsapp string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := e.identityModule.UpsertUserUseCase.Execute(
		ctx,
		input.UpsertUserByWhatsApp{WhatsAppNumber: whatsapp},
	)
	if err != nil {
		return fmt.Errorf("seed second user %q: %w", whatsapp, err)
	}
	parsed, err := uuid.Parse(out.ID)
	if err != nil {
		return fmt.Errorf("parse second user id: %w", err)
	}
	e.identitySecondUserID = parsed
	return nil
}

func (e *e2eCtx) linkChannelToSecondUser(channel, externalID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := e.identityModule.LinkChannelToUser.Execute(ctx, input.LinkChannelToUser{
		UserID:     e.identitySecondUserID,
		Channel:    channel,
		ExternalID: externalID,
	})
	e.identityLinkErr = err
	return nil
}

func (e *e2eCtx) resolvePreferredChannel() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resolved, ok, err := e.identityModule.ResolvePreferredChannel.Execute(ctx, e.identityLinkedUserID)
	if err != nil {
		return fmt.Errorf("resolve preferred channel: %w", err)
	}
	if !ok {
		return fmt.Errorf("no preferred channel found for user %s", e.identityLinkedUserID)
	}
	e.identityResolvedChan = resolved.Channel
	return nil
}

func (e *e2eCtx) assertResolvedChannel(expected string) error {
	if e.identityResolvedChan != expected {
		return fmt.Errorf("expected channel %q, got %q", expected, e.identityResolvedChan)
	}
	return nil
}

func (e *e2eCtx) assertChannelLinkExists(channel, externalID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var count int
	if err := e.mgr.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.user_identities
		 WHERE user_id = $1 AND channel = $2 AND external_id = $3 AND unlinked_at IS NULL`,
		e.identityLinkedUserID, channel, externalID,
	).Scan(&count); err != nil {
		return fmt.Errorf("query user_identities: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("expected 1 active link for channel %q / external_id %q, found %d", channel, externalID, count)
	}
	return nil
}

func (e *e2eCtx) assertChannelAlreadyLinkedError() error {
	if e.identityLinkErr == nil {
		return fmt.Errorf("expected channel-already-linked error, got nil")
	}
	return nil
}

func (e *e2eCtx) projectSubscriptionEvent(eventType, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e.identitySubID == "" {
		e.identitySubID = uuid.NewString()
	}
	periodEnd := time.Now().UTC().Add(30 * 24 * time.Hour)
	if err := e.seedBillingSubscription(ctx, e.identitySubID, e.identityLinkedUserID, status, periodEnd); err != nil {
		return fmt.Errorf("seed billing subscription: %w", err)
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

func (e *e2eCtx) seedBillingSubscription(ctx context.Context, subID string, userID uuid.UUID, status string, periodEnd time.Time) error {
	kiwifyOrderID := "e2e-order-" + subID[:8]
	_, err := e.mgr.ExecContext(ctx, `
		INSERT INTO mecontrola.billing_subscriptions
			(id, funnel_token, user_id, kiwify_order_id, plan_code, status, period_start, period_end, last_event_at)
		VALUES
			($1::uuid, 'e2e-funnel', $2::uuid, $3, 'MONTHLY', $4, now() - interval '1 day', $5, now())
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			period_end = EXCLUDED.period_end,
			user_id = EXCLUDED.user_id,
			last_event_at = now(),
			updated_at = now()
	`, subID, userID, kiwifyOrderID, status, periodEnd)
	return err
}

func (e *e2eCtx) assertEntitlementStatus(expected string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var got string
	if err := e.mgr.QueryRowContext(ctx,
		`SELECT status FROM mecontrola.identity_entitlements WHERE user_id = $1`,
		e.identityLinkedUserID,
	).Scan(&got); err != nil {
		return fmt.Errorf("entitlement for user %s not found: %w", e.identityLinkedUserID, err)
	}
	if got != expected {
		return fmt.Errorf("expected entitlement status %q, got %q", expected, got)
	}
	return nil
}

func (e *e2eCtx) deleteUserViaUseCase() error {
	if e.identityUserID == "" {
		return fmt.Errorf("no identity user ID set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return e.identityModule.MarkUserDeleted.Execute(ctx, input.MarkUserDeleted{ID: e.identityUserID})
}

func (e *e2eCtx) establishPrincipalForWhatsApp(whatsapp string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := e.identityModule.EstablishPrincipal.Execute(ctx, input.EstablishPrincipalInput{
		WhatsAppNumber: whatsapp,
		RequestID:      "e2e-req-" + uuid.NewString()[:8],
		ClientIPRaw:    "127.0.0.1",
	})
	return err
}

func (e *e2eCtx) establishPrincipalForUnknownWhatsApp(whatsapp string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := e.identityModule.EstablishPrincipal.Execute(ctx, input.EstablishPrincipalInput{
		WhatsAppNumber: whatsapp,
		RequestID:      "e2e-req-" + uuid.NewString()[:8],
		ClientIPRaw:    "127.0.0.1",
	})
	if errors.Is(err, application.ErrUnknownUser) {
		return nil
	}
	return err
}

func (e *e2eCtx) recordGatewayAuthFailure() error {
	if e.identityUserID == "" {
		return fmt.Errorf("no identity user ID set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return e.identityModule.RecordGatewayAuthFailure.Handle(ctx, input.RecordGatewayAuthFailureInput{
		UserIDRaw:   e.identityUserID,
		Reason:      "gateway_invalid_signature",
		RequestID:   "e2e-req-" + uuid.NewString()[:8],
		ClientIPRaw: "127.0.0.1",
	})
}

func (e *e2eCtx) assertOutboxEventForUser(eventType string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var count int
	if err := e.mgr.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.outbox_events
		 WHERE event_type = $1 AND aggregate_user_id = $2`,
		eventType, e.identityLinkedUserID,
	).Scan(&count); err != nil {
		return fmt.Errorf("query outbox: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("no %q event in outbox for user %s", eventType, e.identityLinkedUserID)
	}
	return nil
}

func (e *e2eCtx) assertOutboxEventWithoutUser(eventType string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var count int
	if err := e.mgr.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.outbox_events
		 WHERE event_type = $1 AND aggregate_user_id IS NULL AND created_at >= $2`,
		eventType, e.identityScenarioStart,
	).Scan(&count); err != nil {
		return fmt.Errorf("query outbox: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("no %q event without user in outbox since scenario start", eventType)
	}
	return nil
}

func (e *e2eCtx) projectAuthEventForUser(eventType string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	kind := authKindFromEventType(eventType)
	rawPayload := buildAuthPayloadWithUser(e.identityLinkedUserID.String(), kind)
	env := outbox.Envelope{
		ID:              uuid.NewString(),
		EventType:       eventType,
		AggregateUserID: e.identityLinkedUserID.String(),
		Payload:         rawPayload,
	}
	return e.identityModule.AuthEventsConsumer.Handle(ctx, identityTestEvent{
		eventType: eventType,
		payload:   env,
	})
}

func (e *e2eCtx) projectAuthEventWithoutUser(eventType string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	kind := authKindFromEventType(eventType)
	rawPayload := buildAuthPayloadWithoutUser(kind)
	env := outbox.Envelope{
		ID:        uuid.NewString(),
		EventType: eventType,
		Payload:   rawPayload,
	}
	return e.identityModule.AuthEventsConsumer.Handle(ctx, identityTestEvent{
		eventType: eventType,
		payload:   env,
	})
}

func (e *e2eCtx) assertAuthEventKindForUser(kind string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var count int
	if err := e.mgr.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.auth_events WHERE user_id = $1 AND kind = $2`,
		e.identityLinkedUserID, kind,
	).Scan(&count); err != nil {
		return fmt.Errorf("query auth_events: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("no auth_event of kind %q for user %s", kind, e.identityLinkedUserID)
	}
	return nil
}

func (e *e2eCtx) assertAuthEventKindWithoutUser(kind string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var count int
	if err := e.mgr.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.auth_events WHERE kind = $1 AND user_id IS NULL`,
		kind,
	).Scan(&count); err != nil {
		return fmt.Errorf("query auth_events: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("no auth_event of kind %q without user_id", kind)
	}
	return nil
}

func (e *e2eCtx) assertAuthEventsAnonymized() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var count int
	if err := e.mgr.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.auth_events WHERE user_id = $1`,
		e.identityLinkedUserID,
	).Scan(&count); err != nil {
		return fmt.Errorf("query auth_events: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("expected 0 auth_events with user_id %s after anonymization, found %d", e.identityLinkedUserID, count)
	}
	return nil
}

func authKindFromEventType(eventType string) string {
	const prefix = "auth."
	if len(eventType) > len(prefix) && eventType[:len(prefix)] == prefix {
		return eventType[len(prefix):]
	}
	return eventType
}

func buildAuthPayloadWithUser(userID, kind string) json.RawMessage {
	data, _ := json.Marshal(map[string]any{
		"event_id":    uuid.NewString(),
		"user_id":     userID,
		"kind":        kind,
		"source":      "whatsapp",
		"occurred_at": time.Now().UTC().Format(time.RFC3339),
		"request_id":  "e2e-req-" + uuid.NewString()[:8],
		"client_ip":   "127.0.0.1",
	})
	return data
}

func buildAuthPayloadWithoutUser(kind string) json.RawMessage {
	data, _ := json.Marshal(map[string]any{
		"event_id":    uuid.NewString(),
		"kind":        kind,
		"source":      "whatsapp",
		"occurred_at": time.Now().UTC().Format(time.RFC3339),
		"request_id":  "e2e-req-" + uuid.NewString()[:8],
		"client_ip":   "127.0.0.1",
	})
	return data
}
