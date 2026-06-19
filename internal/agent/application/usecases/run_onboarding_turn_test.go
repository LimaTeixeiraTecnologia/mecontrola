package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
)

type fakeTurnInterpreter struct {
	resp interfaces.LLMResponse
	err  error
}

func (f *fakeTurnInterpreter) Interpret(_ context.Context, _ interfaces.LLMRequest) (interfaces.LLMResponse, error) {
	return f.resp, f.err
}

type fakeStateReader struct {
	snapshot usecases.OnboardingSnapshot
	err      error
}

func (f *fakeStateReader) Load(_ context.Context, _ uuid.UUID) (usecases.OnboardingSnapshot, error) {
	return f.snapshot, f.err
}

type fakeToolDispatcher struct {
	calls   int
	results map[string]usecases.OnboardingToolResult
	err     error
}

func (f *fakeToolDispatcher) Dispatch(_ context.Context, _ uuid.UUID, _ string, call interfaces.ToolCall) (usecases.OnboardingToolResult, error) {
	f.calls++
	if f.err != nil {
		return usecases.OnboardingToolResult{}, f.err
	}
	return f.results[call.FunctionName], nil
}

func newTurn(t *testing.T, interp usecases.IntentInterpreter, reader usecases.OnboardingStateReader, dispatcher usecases.OnboardingToolDispatcher) *usecases.RunOnboardingTurn {
	t.Helper()
	uc, err := usecases.NewRunOnboardingTurn(interp, reader, dispatcher, 512, noop.NewProvider())
	require.NoError(t, err)
	return uc
}

func TestRunOnboardingTurn_NotInProgress(t *testing.T) {
	t.Parallel()
	interp := &fakeTurnInterpreter{}
	uc := newTurn(t, interp, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: false}}, &fakeToolDispatcher{})
	out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	require.NoError(t, err)
	require.False(t, out.Handled)
}

func TestRunOnboardingTurn_ReaderError(t *testing.T) {
	t.Parallel()
	uc := newTurn(t, &fakeTurnInterpreter{}, &fakeStateReader{err: errors.New("boom")}, &fakeToolDispatcher{})
	_, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	require.Error(t, err)
}

func TestRunOnboardingTurn_TextReply(t *testing.T) {
	t.Parallel()
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("  Vamos começar? 🚀  ")}}
	uc := newTurn(t, interp, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: true, State: "awaiting_income"}}, &fakeToolDispatcher{})
	out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	require.NoError(t, err)
	require.True(t, out.Handled)
	require.Equal(t, "Vamos começar? 🚀", out.Reply)
}

func TestRunOnboardingTurn_ToolCallsJoined(t *testing.T) {
	t.Parallel()
	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{
		ToolCalls: []interfaces.ToolCall{
			{FunctionName: "record_transaction"},
			{FunctionName: "complete_onboarding_session"},
		},
	}}
	dispatcher := &fakeToolDispatcher{results: map[string]usecases.OnboardingToolResult{
		"record_transaction":          {Reply: "🏆 Boa!"},
		"complete_onboarding_session": {Reply: "🎉 Concluído!", Terminal: true},
	}}
	uc := newTurn(t, interp, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: true, State: "awaiting_first_transaction"}}, dispatcher)
	out, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "gastei 35 no mercado"})
	require.NoError(t, err)
	require.True(t, out.Handled)
	require.Equal(t, 2, dispatcher.calls)
	require.Equal(t, "🏆 Boa!\n\n🎉 Concluído!", out.Reply)
}

func TestRunOnboardingTurn_InterpretError(t *testing.T) {
	t.Parallel()
	interp := &fakeTurnInterpreter{err: errors.New("provider down")}
	uc := newTurn(t, interp, &fakeStateReader{snapshot: usecases.OnboardingSnapshot{InProgress: true, State: "awaiting_income"}}, &fakeToolDispatcher{})
	_, err := uc.Execute(context.Background(), usecases.RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	require.Error(t, err)
}
