package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

type stubConsumeMagicToken struct {
	resp usecases.ConsumeResult
	err  error
}

func (s *stubConsumeMagicToken) Execute(_ context.Context, _ input.ConsumeMagicTokenInput) (usecases.ConsumeResult, error) {
	return s.resp, s.err
}

type stubStartBudgetConfiguration struct {
	called bool
	gotIn  usecases.StartBudgetConfigurationInput
	resp   usecases.StartBudgetConfigurationResult
	err    error
}

func (s *stubStartBudgetConfiguration) Execute(_ context.Context, in usecases.StartBudgetConfigurationInput) (usecases.StartBudgetConfigurationResult, error) {
	s.called = true
	s.gotIn = in
	return s.resp, s.err
}

type recordingWhatsAppGateway struct {
	sent []string
}

func (g *recordingWhatsAppGateway) SendTextMessage(_ context.Context, _, text string) error {
	g.sent = append(g.sent, text)
	return nil
}

func whatsAppMessages() map[string]string {
	return map[string]string{
		"welcome_activated":               "sentinel_welcome_activated",
		"onboarding_intro":                "sentinel_onboarding_intro",
		"already_active":                  "sentinel_already_active",
		"code_already_used_other_account": "sentinel_code_already_used_other_account",
		"payment_still_processing_retry":  "sentinel_payment_still_processing_retry",
		"code_expired_contact_support":    "sentinel_code_expired_contact_support",
		"code_invalid_check_again":        "sentinel_code_invalid_check_again",
		"invalid_country":                 "sentinel_invalid_country",
		"system_unavailable_retry":        "sentinel_system_unavailable_retry",
	}
}

func newWhatsAppProcessor(
	consume services.ConsumeMagicTokenUseCase,
	startBudget services.StartBudgetConfigurationUseCase,
	gateway services.WhatsAppGateway,
) *services.WhatsAppMessageProcessor {
	return services.NewWhatsAppMessageProcessor(
		consume,
		nil,
		startBudget,
		gateway,
		whatsAppMessages(),
		noop.NewProvider(),
	)
}

func TestWhatsAppMessageProcessor_HandleActivation_ActivatedStartsOnboarding(t *testing.T) {
	userID := uuid.New()
	consume := &stubConsumeMagicToken{resp: usecases.ConsumeResult{
		Outcome: usecases.ConsumeOutcomeActivated,
		UserID:  userID.String(),
	}}
	startBudget := &stubStartBudgetConfiguration{}
	gateway := &recordingWhatsAppGateway{}
	sut := newWhatsAppProcessor(consume, startBudget, gateway)

	err := sut.HandleActivation(context.Background(), "+5511999999999", "tok")

	require.NoError(t, err)
	require.Len(t, gateway.sent, 1)
	assert.Equal(t, "sentinel_welcome_activated", gateway.sent[0])
	assert.NotContains(t, gateway.sent, "sentinel_onboarding_intro")

	assert.True(t, startBudget.called)
	assert.Equal(t, userID, startBudget.gotIn.UserID)
	assert.Equal(t, entities.OnboardingChannelWhatsApp, startBudget.gotIn.Channel)
}

func TestWhatsAppMessageProcessor_HandleActivation_NotActivatedSkipsOnboarding(t *testing.T) {
	cases := []struct {
		name    string
		outcome usecases.ConsumeOutcome
		expect  string
	}{
		{name: "not_found", outcome: usecases.ConsumeOutcomeNotFound, expect: "sentinel_code_invalid_check_again"},
		{name: "already_active", outcome: usecases.ConsumeOutcomeAlreadyActive, expect: "sentinel_already_active"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			consume := &stubConsumeMagicToken{resp: usecases.ConsumeResult{Outcome: tc.outcome}}
			startBudget := &stubStartBudgetConfiguration{}
			gateway := &recordingWhatsAppGateway{}
			sut := newWhatsAppProcessor(consume, startBudget, gateway)

			err := sut.HandleActivation(context.Background(), "+5511999999999", "tok")

			require.NoError(t, err)
			require.Len(t, gateway.sent, 1)
			assert.Equal(t, tc.expect, gateway.sent[0])
			assert.NotContains(t, gateway.sent, "sentinel_onboarding_intro")
			assert.False(t, startBudget.called)
		})
	}
}

func TestWhatsAppMessageProcessor_HandleActivation_StartBudgetErrorStillSucceeds(t *testing.T) {
	userID := uuid.New()
	consume := &stubConsumeMagicToken{resp: usecases.ConsumeResult{
		Outcome: usecases.ConsumeOutcomeActivated,
		UserID:  userID.String(),
	}}
	startBudget := &stubStartBudgetConfiguration{err: errors.New("budget down")}
	gateway := &recordingWhatsAppGateway{}
	sut := newWhatsAppProcessor(consume, startBudget, gateway)

	err := sut.HandleActivation(context.Background(), "+5511999999999", "tok")

	require.NoError(t, err)
	require.Len(t, gateway.sent, 1)
	assert.Equal(t, "sentinel_welcome_activated", gateway.sent[0])
	assert.NotContains(t, gateway.sent, "sentinel_onboarding_intro")
	assert.True(t, startBudget.called)
}
