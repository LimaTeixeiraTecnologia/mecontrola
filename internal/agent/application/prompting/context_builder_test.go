package prompting_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/prompting"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
)

func TestBuildContext_Empty(t *testing.T) {
	t.Parallel()
	out, err := prompting.BuildContext(prompting.ContextInput{})
	require.NoError(t, err)
	require.NotEmpty(t, strings.TrimSpace(out.SystemPrompt))
	require.Contains(t, out.SystemPrompt, "MeControla")
	require.NotContains(t, out.SystemPrompt, "## MEMÓRIA DO USUÁRIO")
	require.NotContains(t, out.SystemPrompt, "## CONTEXTO DE SESSÕES ANTERIORES")
	require.Nil(t, out.History)
}

func TestBuildContext_OnlyWorkingMemory(t *testing.T) {
	t.Parallel()
	out, err := prompting.BuildContext(prompting.ContextInput{WorkingMemory: "Renda mensal: 5000"})
	require.NoError(t, err)
	require.Contains(t, out.SystemPrompt, "## MEMÓRIA DO USUÁRIO")
	require.Contains(t, out.SystemPrompt, "Renda mensal: 5000")
	require.NotContains(t, out.SystemPrompt, "## CONTEXTO DE SESSÕES ANTERIORES")
	require.Nil(t, out.History)
}

func TestBuildContext_OnlyHistory(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	out, err := prompting.BuildContext(prompting.ContextInput{
		History: []entities.ConversationMessage{
			{Role: "user", Content: "oi", At: now},
			{Role: "assistant", Content: "olá", At: now},
		},
	})
	require.NoError(t, err)
	require.NotContains(t, out.SystemPrompt, "## MEMÓRIA DO USUÁRIO")
	require.Len(t, out.History, 2)
	require.Equal(t, "user", out.History[0].Role)
	require.Equal(t, "oi", out.History[0].Content)
	require.Equal(t, "assistant", out.History[1].Role)
}

func TestBuildContext_AllCombined(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	out, err := prompting.BuildContext(prompting.ContextInput{
		WorkingMemory:      "Objetivo: economizar",
		ObservationContext: "Sessão anterior: criou cartão",
		JourneyHint:        "Usuário sem orçamento ativo",
		History: []entities.ConversationMessage{
			{Role: "user", Content: "quanto gastei?", At: now},
			{Role: "assistant", Content: "você gastou X", At: now},
		},
	})
	require.NoError(t, err)
	require.Contains(t, out.SystemPrompt, "## MEMÓRIA DO USUÁRIO")
	require.Contains(t, out.SystemPrompt, "Objetivo: economizar")
	require.Contains(t, out.SystemPrompt, "## CONTEXTO DE SESSÕES ANTERIORES")
	require.Contains(t, out.SystemPrompt, "Sessão anterior: criou cartão")
	require.Contains(t, out.SystemPrompt, "## CONTEXTO ATUAL DO USUÁRIO")
	require.Contains(t, out.SystemPrompt, "Usuário sem orçamento ativo")
	require.Len(t, out.History, 2)
}

func TestBuildContext_TruncatesHistory(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	history := make([]entities.ConversationMessage, 0, 8)
	for i := 0; i < 4; i++ {
		history = append(history,
			entities.ConversationMessage{Role: "user", Content: "u", At: now},
			entities.ConversationMessage{Role: "assistant", Content: "a", At: now},
		)
	}
	out, err := prompting.BuildContext(prompting.ContextInput{History: history, MaxHistoryPairs: 1})
	require.NoError(t, err)
	require.Len(t, out.History, 2)
}

func TestBuildContext_SkipsBlankTurns(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	out, err := prompting.BuildContext(prompting.ContextInput{
		History: []entities.ConversationMessage{
			{Role: "user", Content: "   ", At: now},
			{Role: "assistant", Content: "ok", At: now},
		},
	})
	require.NoError(t, err)
	require.Len(t, out.History, 1)
	require.Equal(t, "ok", out.History[0].Content)
}
