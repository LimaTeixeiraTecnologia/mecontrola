package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
)

type stubActivateTelegram struct {
	called bool
	gotIn  usecases.ActivateTelegramByTokenInput
	resp   usecases.ActivateTelegramResult
	err    error
}

func (s *stubActivateTelegram) Execute(_ context.Context, in usecases.ActivateTelegramByTokenInput) (usecases.ActivateTelegramResult, error) {
	s.called = true
	s.gotIn = in
	return s.resp, s.err
}

func newProcessor(t *testing.T, uc services.ActivateTelegramByTokenUseCase) *services.TelegramMessageProcessor {
	t.Helper()
	messages := map[string]string{
		"welcome_activated":               "Conta ativada com sucesso!",
		"already_active":                  "Sua conta ja esta ativa.",
		"requires_whatsapp_activation":    "Ative sua conta no WhatsApp primeiro.",
		"please_use_ativar_command":       "Envie ATIVAR seguido do codigo.",
		"code_already_used_other_account": "Codigo ja usado por outra conta.",
		"payment_still_processing_retry":  "Pagamento ainda processando.",
		"code_expired_contact_support":    "Codigo expirado.",
		"code_invalid_check_again":        "Codigo invalido.",
		"system_unavailable_retry":        "Sistema indisponivel.",
	}
	return services.NewTelegramMessageProcessor(uc, nil, messages, noop.NewProvider())
}

func TestTelegramMessageProcessor_HandleActivation_Linked(t *testing.T) {
	uc := &stubActivateTelegram{resp: usecases.ActivateTelegramResult{Outcome: usecases.ActivateTelegramOutcomeLinked}}
	sut := newProcessor(t, uc)

	reply, err := sut.HandleActivation(context.Background(), 12345, "abc")
	require.NoError(t, err)
	assert.True(t, uc.called)
	assert.Equal(t, int64(12345), uc.gotIn.TelegramUserID)
	assert.Equal(t, "abc", uc.gotIn.Token)
	assert.Equal(t, "Conta ativada com sucesso!", reply)
}

func TestTelegramMessageProcessor_HandleActivation_OutcomeMessages(t *testing.T) {
	cases := []struct {
		outcome usecases.ActivateTelegramOutcome
		expect  string
	}{
		{outcome: usecases.ActivateTelegramOutcomeAlreadyLinked, expect: "Sua conta ja esta ativa."},
		{outcome: usecases.ActivateTelegramOutcomeRequiresWhatsAppActivation, expect: "Ative sua conta no WhatsApp primeiro."},
		{outcome: usecases.ActivateTelegramOutcomeReusedOtherAccount, expect: "Codigo ja usado por outra conta."},
		{outcome: usecases.ActivateTelegramOutcomeNotYetPaid, expect: "Pagamento ainda processando."},
		{outcome: usecases.ActivateTelegramOutcomeExpired, expect: "Codigo expirado."},
		{outcome: usecases.ActivateTelegramOutcomeNotFound, expect: "Codigo invalido."},
	}

	for _, tc := range cases {
		t.Run(tc.outcome.String(), func(t *testing.T) {
			uc := &stubActivateTelegram{resp: usecases.ActivateTelegramResult{Outcome: tc.outcome}}
			sut := newProcessor(t, uc)

			reply, err := sut.HandleActivation(context.Background(), 999, "tok")
			require.NoError(t, err)
			assert.Equal(t, tc.expect, reply)
		})
	}
}

func TestTelegramMessageProcessor_HandleActivation_ErrorReturnsSystemUnavailable(t *testing.T) {
	uc := &stubActivateTelegram{err: errors.New("db down")}
	sut := newProcessor(t, uc)

	reply, err := sut.HandleActivation(context.Background(), 12345, "abc")
	require.Error(t, err)
	assert.Equal(t, "Sistema indisponivel.", reply)
}

func TestTelegramMessageProcessor_HandleFallback(t *testing.T) {
	sut := newProcessor(t, &stubActivateTelegram{})
	reply, err := sut.HandleFallback(context.Background(), 12345)
	require.NoError(t, err)
	assert.Equal(t, "Envie ATIVAR seguido do codigo.", reply)
}

func TestActivateTelegramOutcome_String(t *testing.T) {
	cases := map[usecases.ActivateTelegramOutcome]string{
		usecases.ActivateTelegramOutcomeLinked:                     "linked",
		usecases.ActivateTelegramOutcomeAlreadyLinked:              "already_linked",
		usecases.ActivateTelegramOutcomeRequiresWhatsAppActivation: "requires_whatsapp_activation",
		usecases.ActivateTelegramOutcomeNotYetPaid:                 "not_yet_paid",
		usecases.ActivateTelegramOutcomeExpired:                    "expired",
		usecases.ActivateTelegramOutcomeNotFound:                   "not_found",
		usecases.ActivateTelegramOutcomeReusedOtherAccount:         "reused_other_account",
		usecases.ActivateTelegramOutcome(0):                        "invalid",
		usecases.ActivateTelegramOutcome(99):                       "invalid",
	}
	for k, want := range cases {
		assert.Equal(t, want, k.String())
	}
}
