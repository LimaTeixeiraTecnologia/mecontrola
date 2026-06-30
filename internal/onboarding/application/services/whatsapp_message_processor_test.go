package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
)

type stubConsumeMagicToken struct {
	resp usecases.ConsumeResult
	err  error
}

func (s *stubConsumeMagicToken) Execute(_ context.Context, _ input.ConsumeMagicTokenInput) (usecases.ConsumeResult, error) {
	return s.resp, s.err
}

type stubTryFallbackActivation struct {
	resp usecases.FallbackResult
	err  error
}

func (s *stubTryFallbackActivation) Execute(_ context.Context, _ string) (usecases.FallbackResult, error) {
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
		"already_active":                  "sentinel_already_active",
		"code_already_used_other_account": "sentinel_code_already_used_other_account",
		"payment_still_processing_retry":  "sentinel_payment_still_processing_retry",
		"code_expired_contact_support":    "sentinel_code_expired_contact_support",
		"code_invalid_check_again":        "sentinel_code_invalid_check_again",
		"invalid_country":                 "sentinel_invalid_country",
		"system_unavailable_retry":        "sentinel_system_unavailable_retry",
		"please_use_ativar_command":       "sentinel_please_use_ativar_command",
	}
}

func newWhatsAppProcessor(
	consume services.ConsumeMagicTokenUseCase,
	gateway services.WhatsAppGateway,
) *services.WhatsAppMessageProcessor {
	return services.NewWhatsAppMessageProcessor(
		consume,
		nil,
		gateway,
		whatsAppMessages(),
		noop.NewProvider(),
	)
}

func newWhatsAppProcessorFull(
	consume services.ConsumeMagicTokenUseCase,
	fallback services.TryFallbackActivationUseCase,
	gateway services.WhatsAppGateway,
) *services.WhatsAppMessageProcessor {
	return services.NewWhatsAppMessageProcessor(
		consume,
		fallback,
		gateway,
		whatsAppMessages(),
		noop.NewProvider(),
	)
}

func TestWhatsAppMessageProcessor_HandleActivation_ActivatedSendsWelcome(t *testing.T) {
	consume := &stubConsumeMagicToken{resp: usecases.ConsumeResult{Outcome: usecases.ConsumeOutcomeActivated}}
	gateway := &recordingWhatsAppGateway{}
	sut := newWhatsAppProcessor(consume, gateway)

	err := sut.HandleActivation(context.Background(), "+5511999999999", "tok")

	require.NoError(t, err)
	require.Len(t, gateway.sent, 1)
	assert.Equal(t, "sentinel_welcome_activated", gateway.sent[0])
}

func TestWhatsAppMessageProcessor_HandleActivation_NotFoundSendsInvalid(t *testing.T) {
	consume := &stubConsumeMagicToken{resp: usecases.ConsumeResult{Outcome: usecases.ConsumeOutcomeNotFound}}
	gateway := &recordingWhatsAppGateway{}
	sut := newWhatsAppProcessor(consume, gateway)

	err := sut.HandleActivation(context.Background(), "+5511999999999", "tok")

	require.NoError(t, err)
	require.Len(t, gateway.sent, 1)
	assert.Equal(t, "sentinel_code_invalid_check_again", gateway.sent[0])
}

func TestWhatsAppMessageProcessor_HandleActivation_AlreadyActiveSendsAlreadyActive(t *testing.T) {
	consume := &stubConsumeMagicToken{resp: usecases.ConsumeResult{Outcome: usecases.ConsumeOutcomeAlreadyActive}}
	gateway := &recordingWhatsAppGateway{}
	sut := newWhatsAppProcessor(consume, gateway)

	err := sut.HandleActivation(context.Background(), "+5511999999999", "tok")

	require.NoError(t, err)
	require.Len(t, gateway.sent, 1)
	assert.Equal(t, "sentinel_already_active", gateway.sent[0])
}

func TestWhatsAppMessageProcessor_HandleActivation_NotYetPaidSendsRetry(t *testing.T) {
	consume := &stubConsumeMagicToken{resp: usecases.ConsumeResult{Outcome: usecases.ConsumeOutcomeNotYetPaid}}
	gateway := &recordingWhatsAppGateway{}
	sut := newWhatsAppProcessor(consume, gateway)

	err := sut.HandleActivation(context.Background(), "+5511999999999", "tok")

	require.NoError(t, err)
	require.Len(t, gateway.sent, 1)
	assert.Equal(t, "sentinel_payment_still_processing_retry", gateway.sent[0])
}

func TestWhatsAppMessageProcessor_HandleActivation_ExpiredSendsSupport(t *testing.T) {
	consume := &stubConsumeMagicToken{resp: usecases.ConsumeResult{Outcome: usecases.ConsumeOutcomeExpired}}
	gateway := &recordingWhatsAppGateway{}
	sut := newWhatsAppProcessor(consume, gateway)

	err := sut.HandleActivation(context.Background(), "+5511999999999", "tok")

	require.NoError(t, err)
	require.Len(t, gateway.sent, 1)
	assert.Equal(t, "sentinel_code_expired_contact_support", gateway.sent[0])
}

func TestWhatsAppMessageProcessor_HandleActivation_ReuseOtherAccountSendsAlreadyUsed(t *testing.T) {
	consume := &stubConsumeMagicToken{resp: usecases.ConsumeResult{Outcome: usecases.ConsumeOutcomeReuseOtherAccount}}
	gateway := &recordingWhatsAppGateway{}
	sut := newWhatsAppProcessor(consume, gateway)

	err := sut.HandleActivation(context.Background(), "+5511999999999", "tok")

	require.NoError(t, err)
	require.Len(t, gateway.sent, 1)
	assert.Equal(t, "sentinel_code_already_used_other_account", gateway.sent[0])
}

func TestWhatsAppMessageProcessor_HandleActivation_UnsupportedCountrySendsInvalidCountry(t *testing.T) {
	consume := &stubConsumeMagicToken{resp: usecases.ConsumeResult{Outcome: usecases.ConsumeOutcomeUnsupportedCountry}}
	gateway := &recordingWhatsAppGateway{}
	sut := newWhatsAppProcessor(consume, gateway)

	err := sut.HandleActivation(context.Background(), "+5511999999999", "tok")

	require.NoError(t, err)
	require.Len(t, gateway.sent, 1)
	assert.Equal(t, "sentinel_invalid_country", gateway.sent[0])
}

func TestWhatsAppMessageProcessor_HandleActivation_ExecErrSendsUnavailableAndReturnsError(t *testing.T) {
	consume := &stubConsumeMagicToken{err: errors.New("infra failure")}
	gateway := &recordingWhatsAppGateway{}
	sut := newWhatsAppProcessor(consume, gateway)

	err := sut.HandleActivation(context.Background(), "+5511999999999", "tok")

	require.Error(t, err)
	require.Len(t, gateway.sent, 1)
	assert.Equal(t, "sentinel_system_unavailable_retry", gateway.sent[0])
}

func TestWhatsAppMessageProcessor_HandleFallback_ActivatedSendsWelcome(t *testing.T) {
	fallback := &stubTryFallbackActivation{resp: usecases.FallbackResult{Outcome: usecases.FallbackOutcomeActivated}}
	gateway := &recordingWhatsAppGateway{}
	sut := newWhatsAppProcessorFull(nil, fallback, gateway)

	err := sut.HandleFallback(context.Background(), "+5511999999999")

	require.NoError(t, err)
	require.Len(t, gateway.sent, 1)
	assert.Equal(t, "sentinel_welcome_activated", gateway.sent[0])
}

func TestWhatsAppMessageProcessor_HandleFallback_OutreachRequiredSendsCommand(t *testing.T) {
	fallback := &stubTryFallbackActivation{resp: usecases.FallbackResult{Outcome: usecases.FallbackOutcomeOutreachRequired}}
	gateway := &recordingWhatsAppGateway{}
	sut := newWhatsAppProcessorFull(nil, fallback, gateway)

	err := sut.HandleFallback(context.Background(), "+5511999999999")

	require.NoError(t, err)
	require.Len(t, gateway.sent, 1)
	assert.Equal(t, "sentinel_please_use_ativar_command", gateway.sent[0])
}

func TestWhatsAppMessageProcessor_HandleFallback_ErrorReturnsError(t *testing.T) {
	fallback := &stubTryFallbackActivation{err: errors.New("fallback failure")}
	gateway := &recordingWhatsAppGateway{}
	sut := newWhatsAppProcessorFull(nil, fallback, gateway)

	err := sut.HandleFallback(context.Background(), "+5511999999999")

	require.Error(t, err)
	assert.Empty(t, gateway.sent)
}
