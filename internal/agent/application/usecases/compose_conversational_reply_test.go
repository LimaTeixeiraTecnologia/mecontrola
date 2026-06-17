package usecases_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
)

func newComposeSUT(t *testing.T, fake *fakeInterpreter, maxTokens int) *usecases.ComposeConversationalReply {
	t.Helper()
	uc, err := usecases.NewComposeConversationalReply(fake, maxTokens, noop.NewProvider())
	require.NoError(t, err)
	return uc
}

func TestComposeConversationalReply_NilDeps(t *testing.T) {
	t.Parallel()
	_, err := usecases.NewComposeConversationalReply(nil, 200, noop.NewProvider())
	require.Error(t, err)

	_, err = usecases.NewComposeConversationalReply(&fakeInterpreter{}, 200, nil)
	require.Error(t, err)
}

func TestComposeConversationalReply_EmptyText(t *testing.T) {
	t.Parallel()
	uc := newComposeSUT(t, &fakeInterpreter{}, 200)
	_, err := uc.Execute(context.Background(), usecases.ComposeConversationalInput{UserID: uuid.New(), Channel: "whatsapp", Text: "   "})
	require.ErrorIs(t, err, usecases.ErrComposeEmptyText)
}

func TestComposeConversationalReply_ReturnsTrimmedProse(t *testing.T) {
	t.Parallel()
	fake := &fakeInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("  Vamos organizar suas finanças! 💪  ")}}
	uc := newComposeSUT(t, fake, 150)

	out, err := uc.Execute(context.Background(), usecases.ComposeConversationalInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi, tudo bem?"})

	require.NoError(t, err)
	require.Equal(t, "Vamos organizar suas finanças! 💪", out.Reply)
}

func TestComposeConversationalReply_SendsFreeTextContract(t *testing.T) {
	t.Parallel()
	fake := &fakeInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("ok")}}
	uc := newComposeSUT(t, fake, 150)

	_, err := uc.Execute(context.Background(), usecases.ComposeConversationalInput{UserID: uuid.New(), Channel: "whatsapp", Text: "me ajuda"})
	require.NoError(t, err)

	require.True(t, fake.lastRequest.FreeText)
	require.Equal(t, 150, fake.lastRequest.MaxTokens)
	require.Nil(t, fake.lastRequest.JSONSchema)
	require.Contains(t, fake.lastRequest.SystemPrompt, "Conexão")
	require.Contains(t, fake.lastRequest.SystemPrompt, "Atualização Automática")
	require.Contains(t, fake.lastRequest.SystemPrompt, "Lançamentos (transações)")
	require.Contains(t, fake.lastRequest.SystemPrompt, "Kiwify")
}

func TestComposeConversationalReply_ProviderErrorReturnsDeterministicRedirect(t *testing.T) {
	t.Parallel()
	fake := &fakeInterpreter{err: fmt.Errorf("wrap: %w", services.ErrFallbackChainExhausted)}
	uc := newComposeSUT(t, fake, 200)

	out, err := uc.Execute(context.Background(), usecases.ComposeConversationalInput{UserID: uuid.New(), Channel: "whatsapp", Text: "qualquer coisa"})

	require.NoError(t, err)
	require.NotEmpty(t, out.Reply)
	require.Contains(t, out.Reply, "finanças")
}

func TestComposeConversationalReply_EmptyProviderReplyReturnsRedirect(t *testing.T) {
	t.Parallel()
	fake := &fakeInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("   ")}}
	uc := newComposeSUT(t, fake, 200)

	out, err := uc.Execute(context.Background(), usecases.ComposeConversationalInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})

	require.NoError(t, err)
	require.NotEmpty(t, out.Reply)
}

func TestComposeConversationalReply_GenericProviderErrorStillReplies(t *testing.T) {
	t.Parallel()
	fake := &fakeInterpreter{err: errors.New("transport down")}
	uc := newComposeSUT(t, fake, 200)

	out, err := uc.Execute(context.Background(), usecases.ComposeConversationalInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})

	require.NoError(t, err)
	require.NotEmpty(t, out.Reply)
}
