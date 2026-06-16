package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/commands"
	domainservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

type fakePromptContextLoader struct {
	seed interfaces.PromptSeed
	err  error
}

func (f *fakePromptContextLoader) Load(_ context.Context, _ uuid.UUID, _ string) (interfaces.PromptSeed, error) {
	return f.seed, f.err
}

type fakeHandleInterpreter struct {
	resp interfaces.LLMResponse
	err  error
}

func (f *fakeHandleInterpreter) Interpret(_ context.Context, _ interfaces.LLMRequest) (interfaces.LLMResponse, error) {
	return f.resp, f.err
}

type fakeHandleDispatcher struct {
	calls int
	reply interfaces.DispatchResult
	err   error
}

func (f *fakeHandleDispatcher) Dispatch(_ context.Context, _ uuid.UUID, _ domainservices.IntentOutcome) (interfaces.DispatchResult, error) {
	f.calls++
	return f.reply, f.err
}

type fakeIntentEventPublisher struct {
	rejected []interfaces.IntentEvent
	executed []interfaces.IntentEvent
}

func (f *fakeIntentEventPublisher) PublishExecuted(_ context.Context, ev interfaces.IntentEvent) error {
	f.executed = append(f.executed, ev)
	return nil
}

func (f *fakeIntentEventPublisher) PublishRejected(_ context.Context, ev interfaces.IntentEvent) error {
	f.rejected = append(f.rejected, ev)
	return nil
}

func TestHandleInboundMessage_SafetyBlocksAmbiguousDelete(t *testing.T) {
	t.Parallel()

	dispatcher := &fakeHandleDispatcher{}
	publisher := &fakeIntentEventPublisher{}
	uc := usecases.NewHandleInboundMessage(
		&fakePromptContextLoader{},
		&fakeHandleInterpreter{
			resp: interfaces.LLMResponse{
				Provider: valueobjects.ModelSlugGeminiFlashLite(),
				RawJSON:  []byte(`{"module":"cards","action":"delete","payload":{"id":"3b3b3b3b-0000-0000-0000-000000000001"},"response_hint":"Removendo."}`),
			},
		},
		dispatcher,
		publisher,
		services.NewPromptBuilder(),
		services.NewIntentValidator(),
		services.NewIntentSafetyGuard(),
		domainservices.NewIntentWorkflow(),
		noop.NewProvider(),
	)

	out, err := uc.Execute(context.Background(), usecasesRawMessage(
		uuid.MustParse("3b3b3b3b-1111-1111-1111-111111111111"),
		"whatsapp",
		"remover meu cartao nubank",
	))
	require.NoError(t, err)
	assert.Equal(t, domainservices.IntentOutcomeSafetyBlocked, out.Outcome.Kind)
	assert.Zero(t, dispatcher.calls)
	require.Len(t, publisher.rejected, 1)
	assert.Empty(t, publisher.executed)
	assert.Equal(t, "safety_blocked", publisher.rejected[0].Outcome)
}

func TestHandleInboundMessage_DispatchesSafeMutation(t *testing.T) {
	t.Parallel()

	dispatcher := &fakeHandleDispatcher{
		reply: interfaces.DispatchResult{ReplyText: "Cartao atualizado.", WasApplied: true},
	}
	publisher := &fakeIntentEventPublisher{}
	uc := usecases.NewHandleInboundMessage(
		&fakePromptContextLoader{seed: interfaces.PromptSeed{}},
		&fakeHandleInterpreter{
			resp: interfaces.LLMResponse{
				Provider: valueobjects.ModelSlugGeminiFlashLite(),
				RawJSON:  []byte(`{"module":"cards","action":"update","payload":{"id":"3b3b3b3b-0000-0000-0000-000000000001","limit_cents":500000},"response_hint":"Atualizando."}`),
			},
		},
		dispatcher,
		publisher,
		services.NewPromptBuilder(),
		services.NewIntentValidator(),
		services.NewIntentSafetyGuard(),
		domainservices.NewIntentWorkflow(),
		noop.NewProvider(),
	)

	out, err := uc.Execute(context.Background(), usecasesRawMessage(
		uuid.MustParse("3b3b3b3b-1111-1111-1111-111111111111"),
		"whatsapp",
		"atualizar o limite do cartao para 5000",
	))
	require.NoError(t, err)
	assert.Equal(t, domainservices.IntentOutcomeRouted, out.Outcome.Kind)
	assert.Equal(t, "Cartao atualizado.", out.ReplyText)
	assert.Equal(t, 1, dispatcher.calls)
	require.Len(t, publisher.executed, 1)
	assert.Empty(t, publisher.rejected)
	assert.WithinDuration(t, time.Now().UTC(), publisher.executed[0].OccurredAt, time.Minute)
}

func usecasesRawMessage(userID uuid.UUID, channel string, text string) commands.RawInterpretMessage {
	return commands.RawInterpretMessage{UserID: userID, Channel: channel, Text: text}
}
